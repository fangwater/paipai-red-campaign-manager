import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState, type DragEvent } from "react";
import type { CellValue, Worksheet } from "exceljs";
import {
  AlertCircle, ArrowRight, Bell, CalendarDays, ChartNoAxesCombined, Check, CheckCircle2, ChevronDown, Clock3, Database,
  FilePlus2, FileSpreadsheet, FileText, GitCompareArrows, Image as ImageIcon, LayoutDashboard, Lightbulb, Link2, LoaderCircle, Menu, Megaphone, Tags,
  PanelLeftClose, RefreshCw, Route, Rows3, Search, Settings, Trash2, UploadCloud
} from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import SavedReportHistory, { type SavedImport } from "./SavedReportHistory";

const NoteCampaignAnalysis = lazy(() => import("./NoteCampaignAnalysis"));
const AccountPlanDiagnosis = lazy(() => import("./AccountPlanDiagnosis"));
const TrafficComparison = lazy(() => import("./TrafficComparison"));
const XhsLinkQuery = lazy(() => import("./XhsLinkQuery"));
const XhsSyncCenter = lazy(() => import("./XhsSyncCenter"));
const DataSyncCenter = lazy(() => import("./DataSyncCenter"));
const SystemSettings = lazy(() => import("./SystemSettings"));
const BusinessOverview = lazy(() => import("./BusinessOverview"));
const GuoraiData = lazy(() => import("./GuoraiData"));
const RedMaterials = lazy(() => import("./RedMaterials"));
const RedMaterialComposer = lazy(() => import("./RedMaterialComposer"));
const RedMaterialPending = lazy(() => import("./RedMaterialPending"));
const ContentAnalysis = lazy(() => import("./ContentAnalysis"));
const PlacementNotePerformance = lazy(() => import("./PlacementNotePerformance"));
const SpotlightCampaigns = lazy(() => import("./SpotlightCampaigns"));
const SelfServeDeliveryConsole = lazy(() => import("./SelfServeDeliveryConsole"));
const DandelionUpdate = lazy(() => import("./DandelionUpdate"));

type TableImportResult = {
  key: string;
  name: string;
  fetched: number;
  inserted: number;
  updated: number;
  unchanged: number;
  deleted: number;
};

type ImportResult = {
  run_id: number;
  file_name: string;
  file_sha256: string;
  report_date: string;
  already_saved: boolean;
  present_sheets: string[];
  missing_sheets: string[];
  table_count: number;
  fetched: number;
  inserted: number;
  updated: number;
  unchanged: number;
  deleted: number;
  tables: TableImportResult[];
};

type QueueStatus = "ready" | "saved" | "duplicate" | "uploading" | "complete" | "error";

type UploadItem = {
  id: string;
  file: File;
  fileSHA256: string;
  reportDate: string;
  sheetCount: number;
  presentSheets: string[];
  missingSheets: string[];
  rowCount: number;
  status: QueueStatus;
  error?: string;
  result?: ImportResult;
};

const MAX_FILE_SIZE = 50 * 1024 * 1024;
const EXPECTED_SHEETS = ["总览KPI", "笔记明细", "分SPU总览", "分子账户", "淘搜趋势"];

const navGroups = [
  { label: "总览", items: [{ label: "工作台", icon: LayoutDashboard, path: "/" }] },
  {
    label: "数据中心",
    items: [
      { label: "Maituo 客户日报", icon: UploadCloud, path: "/maituo-daily-report" },
      { label: "薯量数据", icon: ChartNoAxesCombined, path: "/guorai-data" },
      { label: "蒲公英数据更新", icon: UploadCloud, path: "/dandelion-upload" },
      { label: "稿件数据", icon: FileText, path: "/data-sync/manuscripts" },
      { label: "cid数据", icon: CalendarDays, path: "/data-sync/cid" }
    ]
  },
  {
    label: "素材中心",
    items: [
      { label: "添加素材", icon: FilePlus2, path: "/red-materials/new" },
      { label: "待标注素材", icon: Tags, path: "/red-materials/pending" },
      { label: "检索素材", icon: ImageIcon, path: "/red-materials" }
    ]
  },
  {
    label: "分析中心",
    items: [
      { label: "数据总览", icon: LayoutDashboard, path: "/overview" },
      { label: "内容分析", icon: Rows3, path: "/content-analysis" },
      { label: "笔记场域分析", icon: ChartNoAxesCombined, path: "/note-campaign-analysis" },
      { label: "投流情况对比", icon: GitCompareArrows, path: "/traffic-comparison" },
      { label: "聚光关联查询", icon: Link2, path: "/xhs-link-query" }
    ]
  },
  {
    label: "投放管理",
    items: [
      { label: "信息流", icon: Rows3, path: "/delivery/feed" },
      { label: "搜索", icon: Search, path: "/delivery/search" },
      { label: "计划详情", icon: Megaphone, path: "/delivery/campaigns" },
      { label: "自建投流", icon: Route, path: "/self-serve-delivery" },
      { label: "推广计划", icon: Megaphone, path: "/xhs-jg-sync/campaigns" },
      { label: "广告单元", icon: Rows3, path: "/xhs-jg-sync/units" },
      { label: "创意", icon: Lightbulb, path: "/xhs-jg-sync/creativities" }
    ]
  }
];

function fileSize(bytes: number): string {
  return bytes < 1024 * 1024
    ? `${(bytes / 1024).toFixed(1)} KB`
    : `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function bytesToHex(buffer: ArrayBuffer): string {
  return Array.from(new Uint8Array(buffer), (value) => value.toString(16).padStart(2, "0")).join("");
}

function normalizeDate(value: string): string | null {
  const match = value.match(/\d{4}-\d{2}-\d{2}/)?.[0];
  if (!match) return null;
  const date = new Date(`${match}T00:00:00Z`);
  return Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== match ? null : match;
}

function cellDate(value: CellValue): string | null {
  if (value instanceof Date) {
    const year = value.getFullYear();
    const month = String(value.getMonth() + 1).padStart(2, "0");
    const day = String(value.getDate()).padStart(2, "0");
    return `${year}-${month}-${day}`;
  }
  if (typeof value === "object" && value && "result" in value) return normalizeDate(String(value.result ?? ""));
  return normalizeDate(String(value ?? ""));
}

function latestTrendDate(sheet: Worksheet): string | null {
  let latest: string | null = null;
  for (let index = 2; index <= sheet.actualRowCount; index += 1) {
    const date = cellDate(sheet.getRow(index).getCell(1).value);
    if (date && (!latest || date > latest)) latest = date;
  }
  return latest;
}

function statusLabel(item: UploadItem): string {
  if (item.status === "ready") return "待保存";
  if (item.status === "saved") return "已保存";
  if (item.status === "duplicate") return "重复文件";
  if (item.status === "uploading") return "保存中";
  if (item.status === "complete") return "保存完成";
  return "处理失败";
}

function MaituoConsole() {
  const location = useLocation();
  const navigate = useNavigate();
  const dashboard = location.pathname === "/";
  const businessOverview = location.pathname === "/overview";
  const contentAnalysis = location.pathname === "/content-analysis";
  const feedDelivery = location.pathname === "/delivery/feed";
  const searchDelivery = location.pathname === "/delivery/search";
  const spotlightCampaigns = location.pathname === "/delivery/campaigns";
  const selfServeDelivery = location.pathname === "/self-serve-delivery";
  const redMaterialsSearch = location.pathname === "/red-materials";
  const redMaterialsCompose = location.pathname === "/red-materials/new";
  const redMaterialsPending = location.pathname === "/red-materials/pending";
  const redMaterials = redMaterialsSearch || redMaterialsCompose || redMaterialsPending;
  const guoraiData = location.pathname === "/guorai-data";
  const dandelionUpload = location.pathname === "/dandelion-upload";
  const analysis = location.pathname === "/note-campaign-analysis";
  const accountDiagnosis = location.pathname === "/account-plan-diagnosis";
  const trafficComparison = location.pathname === "/traffic-comparison";
  const xhsLinkQuery = location.pathname === "/xhs-link-query";
  const sync = location.pathname.startsWith("/xhs-jg-sync/");
  const syncTarget = location.pathname.endsWith("/units") ? "units" : location.pathname.endsWith("/creativities") ? "creativities" : "campaigns";
  const syncTargetLabel = syncTarget === "units" ? "广告单元" : syncTarget === "creativities" ? "创意" : "推广计划";
  const dataSync = location.pathname.startsWith("/data-sync/");
  const dataSyncTarget = location.pathname.endsWith("/cid") || location.pathname.endsWith("/coenzyme-q10") ? "cid" : location.pathname.endsWith("/manuscripts") ? "manuscripts" : "dandelion";
  const dataSyncTargetLabel = dataSyncTarget === "cid" ? "cid数据" : dataSyncTarget === "manuscripts" ? "稿件数据" : "蒲公英数据";
  const settings = location.pathname === "/settings";
  const breadcrumbSection = redMaterials ? "素材中心" : businessOverview || contentAnalysis || analysis || accountDiagnosis || trafficComparison || xhsLinkQuery ? "分析中心" : feedDelivery || searchDelivery || spotlightCampaigns || selfServeDelivery || sync ? "投放管理" : settings ? "系统" : "数据中心";
  const breadcrumbPage = redMaterialsCompose ? "添加素材" : redMaterialsPending ? "待标注素材" : redMaterialsSearch ? "检索素材" : businessOverview ? "数据总览" : contentAnalysis ? "内容分析" : feedDelivery ? "信息流" : searchDelivery ? "搜索" : spotlightCampaigns ? "计划详情" : selfServeDelivery ? "自建投流" : guoraiData ? "薯量数据" : dandelionUpload ? "蒲公英数据更新" : analysis ? "笔记场域分析" : accountDiagnosis ? "子账户诊断" : trafficComparison ? "投流情况对比" : xhsLinkQuery ? "聚光关联查询" : sync ? syncTargetLabel : dataSync ? dataSyncTargetLabel : settings ? "系统设置" : "Maituo 客户日报";
  const inputRef = useRef<HTMLInputElement>(null);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [preparing, setPreparing] = useState(false);
  const [importing, setImporting] = useState(false);
  const [queue, setQueue] = useState<UploadItem[]>([]);
  const [savedImports, setSavedImports] = useState<SavedImport[]>([]);
  const [savedLoading, setSavedLoading] = useState(true);
  const [pageError, setPageError] = useState("");
  const [serviceState, setServiceState] = useState<"checking" | "online" | "offline">("checking");

  const loadSavedImports = useCallback(async () => {
    setSavedLoading(true);
    try {
      const response = await fetch(`${import.meta.env.BASE_URL}api/imports/maituo-customer-daily`);
      const payload = await response.json() as { success: boolean; data?: SavedImport[]; error?: string };
      if (!response.ok || !payload.success) throw new Error(payload.error || "无法读取已保存报表");
      const items = [...(payload.data ?? [])].sort((left, right) => right.report_date.localeCompare(left.report_date));
      setSavedImports(items);
      const hashes = new Set(items.map((item) => item.file_sha256));
      setQueue((current) => current.map((item) => hashes.has(item.fileSHA256) && item.status === "ready"
        ? { ...item, status: "saved" }
        : item));
    } catch (error) {
      setPageError(error instanceof Error ? error.message : "无法读取已保存报表");
    } finally {
      setSavedLoading(false);
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    fetch(`${import.meta.env.BASE_URL}healthz`, { signal: controller.signal })
      .then((response) => setServiceState(response.ok ? "online" : "offline"))
      .catch(() => setServiceState("offline"));
    void loadSavedImports();
    return () => controller.abort();
  }, [loadSavedImports]);

  const prepareFile = useCallback(async (file: File, id: string, savedHashes: Set<string>): Promise<UploadItem> => {
    if (file.name.split(".").pop()?.toLowerCase() !== "xlsx") {
      return { id, file, fileSHA256: "", reportDate: "", sheetCount: 0, presentSheets: [], missingSheets: [...EXPECTED_SHEETS], rowCount: 0, status: "error", error: "仅支持 .xlsx 文件" };
    }
    if (file.size > MAX_FILE_SIZE) {
      return { id, file, fileSHA256: "", reportDate: "", sheetCount: 0, presentSheets: [], missingSheets: [...EXPECTED_SHEETS], rowCount: 0, status: "error", error: "文件不能超过 50 MB" };
    }
    try {
      const data = await file.arrayBuffer();
      const [ExcelJS, digest] = await Promise.all([
        import("exceljs"),
        crypto.subtle.digest("SHA-256", data.slice(0))
      ]);
      const workbook = new ExcelJS.Workbook();
      await workbook.xlsx.load(data);
      const sheetNames = workbook.worksheets.map((sheet) => sheet.name);
      const presentSheets = EXPECTED_SHEETS.filter((name) => sheetNames.includes(name));
      const missingSheets = EXPECTED_SHEETS.filter((name) => !sheetNames.includes(name));
      if (presentSheets.length === 0) throw new Error("未找到可识别的数据表");
      const trendSheet = workbook.getWorksheet("淘搜趋势");
      const reportDate = normalizeDate(file.name) ?? (trendSheet ? latestTrendDate(trendSheet) : null);
      if (!reportDate) throw new Error("无法识别报表日期");
      const fileSHA256 = bytesToHex(digest);
      const rowCount = presentSheets.reduce((total, name) => total + Math.max((workbook.getWorksheet(name)?.actualRowCount ?? 1) - 1, 0), 0);
      return {
        id, file, fileSHA256, reportDate, sheetCount: presentSheets.length, presentSheets, missingSheets, rowCount,
        status: savedHashes.has(fileSHA256) ? "saved" : "ready"
      };
    } catch (error) {
      return {
        id, file, fileSHA256: "", reportDate: normalizeDate(file.name) ?? "", sheetCount: 0, presentSheets: [], missingSheets: [...EXPECTED_SHEETS], rowCount: 0,
        status: "error", error: error instanceof Error ? error.message : "Excel 解析失败"
      };
    }
  }, []);

  const addFiles = useCallback(async (files: File[]) => {
    if (files.length === 0 || preparing || importing) return;
    setPreparing(true);
    setPageError("");
    const savedHashes = new Set(savedImports.map((item) => item.file_sha256));
    const stamp = Date.now();
    const prepared = await Promise.all(files.map((file, index) => prepareFile(file, `${stamp}-${index}-${file.name}`, savedHashes)));
    setQueue((current) => {
      const seen = new Set(current.map((item) => item.fileSHA256).filter(Boolean));
      const additions = prepared.map((item) => {
        if (item.fileSHA256 && seen.has(item.fileSHA256)) return { ...item, status: "duplicate" as const };
        if (item.fileSHA256) seen.add(item.fileSHA256);
        return item;
      });
      return [...current, ...additions].sort((left, right) =>
        (left.reportDate || "9999-99-99").localeCompare(right.reportDate || "9999-99-99") || left.file.name.localeCompare(right.file.name));
    });
    setPreparing(false);
  }, [importing, prepareFile, preparing, savedImports]);

  const onDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setDragging(false);
    void addFiles(Array.from(event.dataTransfer.files));
  };

  const updateQueueItem = (id: string, update: Partial<UploadItem>) => {
    setQueue((current) => current.map((item) => item.id === id ? { ...item, ...update } : item));
  };

  const importFiles = async () => {
    if (importing) return;
    const pending = queue.filter((item) => item.status === "ready").sort((left, right) => left.reportDate.localeCompare(right.reportDate));
    if (pending.length === 0) return;
    setImporting(true);
    setPageError("");
    for (const item of pending) {
      updateQueueItem(item.id, { status: "uploading", error: undefined });
      try {
        const formData = new FormData();
        formData.append("file", item.file);
        const response = await fetch(`${import.meta.env.BASE_URL}api/imports/maituo-customer-daily`, { method: "POST", body: formData });
        const payload = await response.json() as { success: boolean; data?: ImportResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "保存失败");
        updateQueueItem(item.id, { status: payload.data.already_saved ? "saved" : "complete", result: payload.data });
      } catch (error) {
        updateQueueItem(item.id, { status: "error", error: error instanceof Error ? error.message : "保存失败" });
      }
    }
    setImporting(false);
    await loadSavedImports();
  };

  const readyCount = queue.filter((item) => item.status === "ready").length;
  const savedCount = queue.filter((item) => item.status === "saved" || item.status === "complete").length;
  const savedDates = useMemo(() => new Set(savedImports.map((item) => item.report_date)), [savedImports]);

  return (
    <div className="app-shell">
      <aside className={`sidebar ${sidebarOpen ? "sidebar-open" : ""}`}>
        <div className="brand-row">
          <div className="brand-mark">P</div>
          <div className="brand-copy"><strong>PaiPai RED</strong><span>数据中台</span></div>
          <button className="icon-button sidebar-close" onClick={() => setSidebarOpen(false)} aria-label="关闭导航"><PanelLeftClose size={19} /></button>
        </div>
        <nav className="navigation" aria-label="主导航">
          {navGroups.map((group) => <div className="nav-group" key={group.label}>
            <div className="nav-label">{group.label}</div>
            {group.items.map((item) => {
              const Icon = item.icon;
              return <button className={"nav-item " + (item.path === location.pathname ? "active" : "")} key={item.label} disabled={!item.path} onClick={() => { if (item.path) { navigate(item.path); setSidebarOpen(false); } }}>
                <Icon size={18} strokeWidth={1.8} /><span>{item.label}</span>
              </button>;
            })}
          </div>)}
        </nav>
        <div className="sidebar-footer">
          <div className="service-state"><span className={`status-dot ${serviceState}`} />{serviceState === "online" ? "同步服务正常" : serviceState === "offline" ? "同步服务异常" : "正在检查服务"}</div>
          <button className={"nav-item " + (settings ? "active" : "")} onClick={() => { navigate("/settings"); setSidebarOpen(false); }}><Settings size={18} /><span>系统设置</span></button>
        </div>
      </aside>
      {sidebarOpen && <button className="sidebar-backdrop" onClick={() => setSidebarOpen(false)} aria-label="关闭导航" />}

      <div className="workspace">
        <header className="topbar">
          <div className="topbar-left">
            <button className="icon-button mobile-menu" onClick={() => setSidebarOpen(true)} aria-label="打开导航"><Menu size={20} /></button>
            <div className="breadcrumb">{dashboard ? <strong>工作台</strong> : <><span>{breadcrumbSection}</span><b>/</b><strong>{breadcrumbPage}</strong></>}</div>
          </div>
          <div className="topbar-actions">
            <button className="icon-button" aria-label="搜索"><Search size={19} /></button>
            <button className="icon-button notification-button" aria-label="通知"><Bell size={19} /><span /></button>
            <div className="profile"><div className="avatar">PA</div><span>管理员</span><ChevronDown size={15} /></div>
          </div>
        </header>

        <main className="main-content">
          {dandelionUpload ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载更新页面</div>}><DandelionUpdate /></Suspense> : contentAnalysis ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载内容分析</div>}><ContentAnalysis serviceState={serviceState} /></Suspense> : feedDelivery ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载信息流</div>}><PlacementNotePerformance placement="feed" serviceState={serviceState} /></Suspense> : searchDelivery ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载搜索</div>}><PlacementNotePerformance placement="search" serviceState={serviceState} /></Suspense> : spotlightCampaigns ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载计划详情</div>}><SpotlightCampaigns /></Suspense> : selfServeDelivery ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载自建投流工作台</div>}><SelfServeDeliveryConsole /></Suspense> : dashboard ? <>
            <section className="page-heading">
              <div><h1>数据中台</h1><p>PaiPai RED · 业务数据工作台</p></div>
              <div className="heading-status"><span className={`status-dot ${serviceState}`} />{serviceState === "online" ? "后端服务已连接" : serviceState === "offline" ? "后端服务未连接" : "正在检查连接"}</div>
            </section>
            <section className="entry-section">
              <div className="entry-header"><h2>业务入口</h2><span>14 个可用功能</span></div>
              <button className="entry-row" onClick={() => navigate("/overview")}>
                <span className="entry-icon analysis-entry-icon"><LayoutDashboard size={22} /></span>
                <span className="entry-copy"><strong>查看数据总览</strong><small>辅酶投放趋势与机构每日新增笔记</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/content-analysis")}>
                <span className="entry-icon analysis-entry-icon"><Rows3 size={22} /></span>
                <span className="entry-copy"><strong>查看内容分析</strong><small>内容类型、人群与场景表现热力图</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/delivery/feed")}>
                <span className="entry-icon sync-entry-icon"><Rows3 size={22} /></span>
                <span className="entry-copy"><strong>查看信息流</strong><small>按信息流累计消耗查看笔记表现</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/delivery/search")}>
                <span className="entry-icon sync-entry-icon"><Search size={22} /></span>
                <span className="entry-copy"><strong>查看搜索</strong><small>按搜索累计消耗查看笔记表现</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/self-serve-delivery")}>
                <span className="entry-icon sync-entry-icon"><Route size={22} /></span>
                <span className="entry-copy"><strong>进入自建投流</strong><small>草稿、校验审批、发布、资产报表与算法建议</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/red-materials/new")}>
                <span className="entry-icon material-entry-icon"><FilePlus2 size={22} /></span>
                <span className="entry-copy"><strong>进入添加素材</strong><small>配置笔记 ID、链接、图片、评论、标题和正文</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/red-materials/pending")}>
                <span className="entry-icon material-entry-icon"><Tags size={22} /></span>
                <span className="entry-copy"><strong>进入待标注素材</strong><small>为手动添加的素材补齐标签</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/red-materials")}>
                <span className="entry-icon material-entry-icon"><ImageIcon size={22} /></span>
                <span className="entry-copy"><strong>进入检索素材</strong><small>查找稿件引用的参考小红书笔记</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/maituo-daily-report")}>
                <span className="entry-icon"><FileSpreadsheet size={22} /></span>
                <span className="entry-copy"><strong>更新 Maituo 客户日报</strong><small>按日期保存前四张业务表</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/guorai-data")}>
                <span className="entry-icon"><ChartNoAxesCombined size={22} /></span>
                <span className="entry-copy"><strong>查看薯量数据</strong><small>最新笔记与计划快照、关联和投放指标</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/note-campaign-analysis")}>
                <span className="entry-icon analysis-entry-icon"><ChartNoAxesCombined size={22} /></span>
                <span className="entry-copy"><strong>查看笔记场域分析</strong><small>按笔记与场域查看累计消耗、回搜人数与成本</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/xhs-link-query")}>
                <span className="entry-icon analysis-entry-icon"><Link2 size={22} /></span>
                <span className="entry-copy"><strong>查询聚光关联数据</strong><small>按笔记场域查看真实聚光投放层级</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/traffic-comparison")}>
                <span className="entry-icon analysis-entry-icon"><GitCompareArrows size={22} /></span>
                <span className="entry-copy"><strong>对比投流情况</strong><small>同笔记同场域下比较不同计划成本</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
              <button className="entry-row" onClick={() => navigate("/xhs-jg-sync/campaigns")}>
                <span className="entry-icon sync-entry-icon"><RefreshCw size={22} /></span>
                <span className="entry-copy"><strong>同步聚光投放数据</strong><small>计划、单元与创意</small></span>
                <span className="entry-action">进入<ArrowRight size={17} /></span>
              </button>
            </section>
          </> : redMaterialsCompose ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载添加素材</div>}><RedMaterialComposer serviceState={serviceState} /></Suspense> : redMaterialsPending ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载待标注素材</div>}><RedMaterialPending serviceState={serviceState} /></Suspense> : redMaterialsSearch ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载检索素材</div>}><RedMaterials serviceState={serviceState} /></Suspense> : businessOverview ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载数据总览</div>}><BusinessOverview serviceState={serviceState} /></Suspense> : guoraiData ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载薯量数据</div>}><GuoraiData serviceState={serviceState} /></Suspense> : analysis ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载分析页面</div>}><NoteCampaignAnalysis serviceState={serviceState} /></Suspense> : accountDiagnosis ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载诊断页面</div>}><AccountPlanDiagnosis serviceState={serviceState} /></Suspense> : trafficComparison ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载投流对比</div>}><TrafficComparison serviceState={serviceState} /></Suspense> : xhsLinkQuery ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载关联查询</div>}><XhsLinkQuery /></Suspense> : sync ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载同步页面</div>}><XhsSyncCenter activeTarget={syncTarget} /></Suspense> : dataSync ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载同步页面</div>}><DataSyncCenter activeTarget={dataSyncTarget} /></Suspense> : settings ? <Suspense fallback={<div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在加载系统状态</div>}><SystemSettings /></Suspense> : <>
            <section className="page-heading">
              <div><h1>Maituo 客户日报</h1><p>数据中心 · 多文件导入</p></div>
              <div className="heading-status"><span className={`status-dot ${serviceState}`} />{serviceState === "online" ? "后端服务已连接" : serviceState === "offline" ? "后端服务未连接" : "正在检查连接"}</div>
            </section>

            <section className="import-layout multi-import-layout">
              <div className="upload-panel">
                <div className="section-heading"><div><span className="step-number">1</span><h2>待导入文件</h2></div><span className="file-rule">XLSX · 单个 50 MB</span></div>
                <div className={`dropzone compact-dropzone ${dragging ? "dragging" : ""}`}
                  onDragEnter={(event) => { event.preventDefault(); setDragging(true); }}
                  onDragOver={(event) => event.preventDefault()}
                  onDragLeave={() => setDragging(false)} onDrop={onDrop}>
                  <div className="upload-icon"><UploadCloud size={26} strokeWidth={1.7} /></div>
                  <div className="dropzone-copy"><strong>{preparing ? "正在解析文件" : "选择 Excel 文件"}</strong><span>支持一次选择多个文件</span></div>
                  <button className="primary-button" disabled={preparing || importing} onClick={() => inputRef.current?.click()}>{preparing ? "解析中" : "选择文件"}</button>
                </div>
                <input ref={inputRef} type="file" accept=".xlsx" multiple hidden onChange={(event) => { void addFiles(Array.from(event.target.files ?? [])); event.target.value = ""; }} />

                {queue.length > 0 && <div className="upload-queue" aria-label="待导入文件列表">
                  <div className="queue-columns"><span>报表日期</span><span>文件</span><span>状态</span><span /></div>
                  {queue.map((item) => <div className={`queue-row queue-${item.status}`} key={item.id}>
                    <div className="queue-date"><strong>{item.reportDate || "日期未知"}</strong>{item.reportDate && savedDates.has(item.reportDate) && item.status === "ready" ? <span>同日期已有版本</span> : null}</div>
                    <div className="queue-file"><FileSpreadsheet size={18} /><div><strong title={item.file.name}>{item.file.name}</strong><span>{fileSize(item.file.size)}{item.sheetCount ? ` · 已识别 ${item.sheetCount}/5 张表 · ${item.rowCount} 行` : ""}</span>{item.missingSheets.length > 0 && item.status !== "error" ? <small className="sheet-warning" title={item.missingSheets.join("、")}>缺少：{item.missingSheets.join("、")}</small> : null}{item.result && <small>新增 {item.result.inserted} · 更新 {item.result.updated} · 未变化 {item.result.unchanged}</small>}{item.error && <small className="queue-error">{item.error}</small>}</div></div>
                    <div className={`queue-status status-${item.status}`}>
                      {item.status === "uploading" ? <LoaderCircle size={15} className="spin" /> : item.status === "error" ? <AlertCircle size={15} /> : item.status === "ready" ? <Clock3 size={15} /> : <CheckCircle2 size={15} />}
                      {statusLabel(item)}
                    </div>
                    <button className="icon-button queue-remove" disabled={item.status === "uploading"} onClick={() => setQueue((current) => current.filter((candidate) => candidate.id !== item.id))} aria-label={`移除 ${item.file.name}`}><Trash2 size={16} /></button>
                  </div>)}
                </div>}
                {pageError && <div className="error-message"><AlertCircle size={16} />{pageError}</div>}
              </div>

              <aside className="strategy-panel">
                <div className="section-heading compact"><div><span className="step-number">2</span><h2>保存任务</h2></div></div>
                <div className="task-metrics"><div><span>待保存</span><strong>{readyCount}</strong></div><div><span>已识别</span><strong>{queue.length}</strong></div><div><span>已保存</span><strong>{savedCount}</strong></div></div>
                <div className="task-rule"><Check size={15} /><span>按报表日期从早到晚</span></div>
                <div className="task-rule"><Check size={15} /><span>相同文件自动跳过</span></div>
                <div className="task-rule"><Check size={15} /><span>前四张业务表按日期保留</span></div>
                <div className="task-rule"><Check size={15} /><span>缺少的表跳过且不改动</span></div>
                <button className="secondary-button task-submit" disabled={readyCount === 0 || importing || preparing} onClick={() => void importFiles()}>{importing ? "正在依次保存" : `保存 ${readyCount} 个文件`}</button>
              </aside>
            </section>

            <SavedReportHistory reports={savedImports} loading={savedLoading} expectedSheets={EXPECTED_SHEETS} />
          </>}
        </main>
      </div>
    </div>
  );
}

export default MaituoConsole;
