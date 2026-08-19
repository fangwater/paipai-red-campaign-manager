import { useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, ArrowDownWideNarrow, ChevronLeft, ChevronRight, ExternalLink, LoaderCircle, X } from "lucide-react";
import { Link } from "react-router-dom";
import "./content-analysis.css";

type ServiceState = "checking" | "online" | "offline";
type SPUOption = "辅酶" | "磷虾油";
type AgencyOption = "全部" | "曼杰" | "有一有二" | "智元";
type DimensionOption = "audience" | "scenario";
type DetailFilter = "all" | "boom" | "flow" | "roi" | "qualified";
type NoteSortOption = "search_spend" | "feed_spend" | "search_cost_change";
type CostFilter = "search" | "feed" | "search_stopped" | "feed_stopped";

type ContentNote = {
  note_id: string;
  title: string;
  url: string;
  author: string;
  published_date: string;
  agency: string;
  content_type: string;
  audience: string;
  scenario: string;
  dandelion_cost: number | null;
  boom: boolean;
  search_spend: number;
  search_cost: number | null;
  latest_search_cost: number | null;
  search_cost_change: number | null;
  search_qualified: boolean;
  feed_spend: number;
  feed_cost: number | null;
  feed_qualified: boolean;
  latest_search_spend: number;
  latest_feed_spend: number;
  search_stopped: boolean;
  feed_stopped: boolean;
  flow_evaluated: boolean;
  flow_qualified: boolean;
  roi: number | null;
  roi_qualified: boolean;
  all_qualified: boolean;
};

type ContentCell = {
  content_type: string;
  dimension: string;
  total_notes: number;
  dandelion_eligible: number;
  boom_count: number;
  boom_rate: number | null;
  flow_evaluated: number;
  flow_qualified: number;
  roi_evaluated: number;
  roi_qualified: number;
  all_qualified: number;
  notes: ContentNote[];
};

type ContentResult = {
  spu: SPUOption;
  agency: AgencyOption;
  dimension: DimensionOption;
  published_start_date: string;
  published_end_date: string;
  sources: {
    dandelion_data_date: string;
    dandelion_synced_at: string;
    maituo_report_date: string;
    guorai_snapshot_date: string;
    guorai_window_start: string;
    guorai_window_end: string;
    manuscript_synced_at: string;
  };
  coverage: {
    total_notes: number;
    content_type_tagged: number;
    audience_tagged: number;
    scenario_tagged: number;
    dandelion_cost_notes: number;
    flow_evaluated_notes: number;
    roi_evaluated_notes: number;
    all_metrics_notes: number;
  };
  types: string[];
  dimensions: string[];
  cells: ContentCell[];
};

const SPU_OPTIONS: SPUOption[] = ["辅酶", "磷虾油"];
const AGENCY_OPTIONS: AgencyOption[] = ["全部", "曼杰", "有一有二", "智元"];
const DETAIL_FILTERS: Array<{ value: DetailFilter; label: string }> = [
  { value: "all", label: "全部" },
  { value: "boom", label: "爆文" },
  { value: "flow", label: "投流达标" },
  { value: "roi", label: "ROI 达标" },
  { value: "qualified", label: "三项达标" }
];
const NOTE_PAGE_SIZE = 20;
const DEFAULT_SEARCH_COST_LIMIT = 30;
const DEFAULT_FEED_COST_LIMIT = 70;
const NOTE_SORT_OPTIONS: Array<{ value: NoteSortOption; label: string; description: string }> = [
  { value: "search_spend", label: "搜索累计消耗", description: "按搜索累计消耗从高到低排序" },
  { value: "feed_spend", label: "信息流累计消耗", description: "按信息流累计消耗从高到低排序" },
  { value: "search_cost_change", label: "回搜成本变化", description: "按当日回搜成本 − 累计回搜成本从高到低排序" }
];
const integer = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
const money = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const decimal = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

function safeURL(value: string): string | null {
  try {
    const parsed = new URL(value);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? parsed.toString() : null;
  } catch {
    return null;
  }
}

function rateClass(cell: ContentCell): string {
  if (cell.boom_rate === null) return "no-data";
  if (cell.boom_rate === 0) return "rate-1";
  if (cell.boom_rate < 0.25) return "rate-2";
  if (cell.boom_rate < 0.5) return "rate-3";
  if (cell.boom_rate < 0.75) return "rate-4";
  return "rate-5";
}

function percentage(value: number | null): string {
  return value === null ? "--" : Math.round(value * 100) + "%";
}

function coverageRate(value: number, total: number): string {
  return total === 0 ? "--" : Math.round(value / total * 100) + "%";
}

function metricCost(spend: number, cost: number | null): string {
  if (spend <= 0) return "--";
  return cost === null ? "¥" + money.format(spend) + " · 暂无成本" : "¥" + money.format(spend) + " · ¥" + money.format(cost);
}

function formatCostChange(value: number | null): string {
  if (value === null) return "--";
  if (value > 0) return "+¥" + money.format(value);
  if (value < 0) return "-¥" + money.format(Math.abs(value));
  return "¥" + money.format(0);
}

function parseCostLimit(value: string, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
}

function costExceeds(cost: number | null, limit: number): boolean {
  return cost !== null && cost > limit;
}

function PlacementMetric({ spend, cost, qualified, stopped }: {
  spend: number;
  cost: number | null;
  qualified: boolean;
  stopped: boolean;
}) {
  return <div className={"content-note-metric" + (qualified && !stopped ? " metric-good" : "")}>
    <span>{metricCost(spend, cost)}</span>
    {stopped ? <em className="content-note-stopped">已停投</em> : null}
  </div>;
}

function noteCampaignAnalysisPath(noteID: string): string {
  return "/note-campaign-analysis?" + new URLSearchParams({ q: noteID }).toString();
}

function NoteStatus({ note }: { note: ContentNote }) {
  return <div className="content-note-status">
    {note.all_qualified ? <span className="all">三项达标</span> : null}
    {note.boom ? <span className="boom">爆文</span> : null}
    {note.flow_qualified ? <span className="flow">投流达标</span> : null}
    {note.roi_qualified ? <span className="roi">ROI 达标</span> : null}
    {!note.boom && !note.flow_qualified && !note.roi_qualified ? <span className="none">暂无达标项</span> : null}
  </div>;
}

function ContentNoteTable({ notes, showStatus = true, label }: {
  notes: ContentNote[];
  showStatus?: boolean;
  label: string;
}) {
  return <div className="content-note-table-wrap"><table className={"content-note-table" + (showStatus ? "" : " content-note-table-summary")} aria-label={label}>
    <thead><tr><th>笔记</th><th>机构与标签</th><th>站外成本 15 天</th><th>搜索累计消耗 · 成本</th><th>信息流累计消耗 · 成本</th><th>回搜成本变化</th><th>薯量 ROI</th>{showStatus ? <th>状态</th> : null}</tr></thead>
    <tbody>{notes.map((note) => {
      const noteURL = safeURL(note.url);
      return <tr key={note.note_id}>
        <td><div className="content-note-identity">
          {noteURL ? <a className="content-note-title" href={noteURL} target="_blank" rel="noreferrer" title={note.title}>{note.title}<ExternalLink size={12} /></a> : <strong className="content-note-title" title={note.title}>{note.title}</strong>}
          <Link className="content-note-id" to={noteCampaignAnalysisPath(note.note_id)} title="查看笔记计划分析" aria-label={"查看笔记计划分析 " + note.note_id}>{note.note_id}</Link>
          <small>{note.author || "未知达人"} · {note.published_date || "发布时间未知"}</small>
        </div></td>
        <td><div className="content-note-labels"><strong>{note.agency}</strong><span>{note.content_type} · {note.audience} · {note.scenario}</span></div></td>
        <td className={note.boom ? "metric-good" : ""}>{note.dandelion_cost === null || note.dandelion_cost <= 0 ? "--" : "¥" + money.format(note.dandelion_cost)}</td>
        <td><PlacementMetric spend={note.search_spend} cost={note.search_cost} qualified={note.search_qualified} stopped={note.search_stopped} /></td>
        <td><PlacementMetric spend={note.feed_spend} cost={note.feed_cost} qualified={note.feed_qualified} stopped={note.feed_stopped} /></td>
        <td className={"search-cost-change" + (note.search_cost_change !== null && note.search_cost_change > 0 ? " increase" : note.search_cost_change !== null && note.search_cost_change < 0 ? " decrease" : "")} title={note.latest_search_cost === null || note.search_cost === null ? "暂无完整回搜成本" : "当日回搜成本 ¥" + money.format(note.latest_search_cost) + " − 累计回搜成本 ¥" + money.format(note.search_cost)}>{formatCostChange(note.search_cost_change)}</td>
        <td className={note.roi_qualified ? "metric-good" : ""}>{note.roi === null ? "--" : decimal.format(note.roi)}</td>
        {showStatus ? <td><NoteStatus note={note} /></td> : null}
      </tr>;
    })}</tbody>
  </table></div>;
}

function ContentDetailDrawer({ cell, dimension, onClose }: {
  cell: ContentCell;
  dimension: DimensionOption;
  onClose: () => void;
}) {
  const [filter, setFilter] = useState<DetailFilter>("all");
  const notes = useMemo(() => cell.notes.filter((note) => {
    if (filter === "boom") return note.boom;
    if (filter === "flow") return note.flow_qualified;
    if (filter === "roi") return note.roi_qualified;
    if (filter === "qualified") return note.all_qualified;
    return true;
  }), [cell.notes, filter]);
  const counts: Record<DetailFilter, number> = {
    all: cell.total_notes,
    boom: cell.boom_count,
    flow: cell.flow_qualified,
    roi: cell.roi_qualified,
    qualified: cell.all_qualified
  };

  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => { if (event.key === "Escape") onClose(); };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [onClose]);

  return <>
    <button className="content-drawer-backdrop" type="button" aria-label="关闭笔记明细" onClick={onClose} />
    <aside className="content-drawer" aria-label={cell.content_type + "与" + cell.dimension + "笔记明细"}>
      <header className="content-drawer-head">
        <div><h2>{cell.content_type} × {cell.dimension}</h2><p>{dimension === "audience" ? "人群标签" : "用户场景"} · 爆文率 {percentage(cell.boom_rate)} · 有效成本 {cell.dandelion_eligible}/{cell.total_notes}</p></div>
        <button className="icon-button" type="button" title="关闭" aria-label="关闭" onClick={onClose}><X size={19} /></button>
      </header>
      <div className="content-drawer-tabs" aria-label="笔记表现分类">
        {DETAIL_FILTERS.map((item) => <button type="button" className={filter === item.value ? "active" : ""} key={item.value} onClick={() => setFilter(item.value)}>{item.label}<span>{counts[item.value]}</span></button>)}
      </div>
      <div className="content-drawer-body">
        <div className="content-drawer-summary">
          <span>爆文 <strong>{cell.boom_count}</strong></span>
          <span>投流达标 <strong>{cell.flow_qualified}</strong></span>
          <span>ROI 达标 <strong>{cell.roi_qualified}</strong></span>
          <span>三项达标 <strong>{cell.all_qualified}</strong></span>
        </div>
        {notes.length === 0 ? <div className="content-drawer-empty">当前分类暂无笔记</div> : <ContentNoteTable notes={notes} label="热力图笔记明细" />}
      </div>
    </aside>
  </>;
}

function ContentAnalysis({ serviceState }: { serviceState: ServiceState }) {
  const [spu, setSPU] = useState<SPUOption>("辅酶");
  const [agency, setAgency] = useState<AgencyOption>("全部");
  const [dimension, setDimension] = useState<DimensionOption>("audience");
  const [publishedStartDate, setPublishedStartDate] = useState("");
  const [publishedEndDate, setPublishedEndDate] = useState("");
  const [includeUnlabeled, setIncludeUnlabeled] = useState(false);
  const [noteSort, setNoteSort] = useState<NoteSortOption>("search_spend");
  const [costFilters, setCostFilters] = useState<CostFilter[]>([]);
  const [searchCostLimitInput, setSearchCostLimitInput] = useState(String(DEFAULT_SEARCH_COST_LIMIT));
  const [feedCostLimitInput, setFeedCostLimitInput] = useState(String(DEFAULT_FEED_COST_LIMIT));
  const [notePage, setNotePage] = useState(1);
  const noteSectionRef = useRef<HTMLElement>(null);
  const [result, setResult] = useState<ContentResult | null>(null);
  const [selectedCell, setSelectedCell] = useState<ContentCell | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    setSelectedCell(null);
    const params = new URLSearchParams({ spu, agency, dimension });
    if (publishedStartDate) params.set("published_start_date", publishedStartDate);
    if (publishedEndDate) params.set("published_end_date", publishedEndDate);
    fetch(import.meta.env.BASE_URL + "api/analytics/content-analysis?" + params, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: ContentResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "内容分析读取失败");
        setResult(payload.data);
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "内容分析读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [agency, dimension, publishedEndDate, publishedStartDate, spu]);

  const types = result?.types.filter((value) => includeUnlabeled || value !== "未标注") ?? [];
  const dimensions = result?.dimensions.filter((value) => includeUnlabeled || value !== "未标注") ?? [];
  const cells = useMemo(() => new Map((result?.cells ?? []).map((cell) => [cell.content_type + "\u0000" + cell.dimension, cell])), [result]);
  const searchCostLimit = parseCostLimit(searchCostLimitInput, DEFAULT_SEARCH_COST_LIMIT);
  const feedCostLimit = parseCostLimit(feedCostLimitInput, DEFAULT_FEED_COST_LIMIT);
  const visibleNotes = useMemo(() => {
    const notesByID = new Map<string, ContentNote>();
    for (const cell of result?.cells ?? []) {
      if (!includeUnlabeled && (cell.content_type === "未标注" || cell.dimension === "未标注")) continue;
      for (const note of cell.notes) notesByID.set(note.note_id, note);
    }
    return Array.from(notesByID.values()).filter((note) => {
      if (costFilters.length === 0) return true;
      return costFilters.some((filter) => {
        if (filter === "search") return costExceeds(note.search_cost, searchCostLimit);
        if (filter === "feed") return costExceeds(note.feed_cost, feedCostLimit);
        if (filter === "search_stopped") return note.search_stopped;
        return note.feed_stopped;
      });
    }).sort((left, right) => {
      const sortValue = (note: ContentNote) => {
        if (noteSort === "feed_spend") return note.feed_spend;
        if (noteSort === "search_cost_change") return note.search_cost_change ?? Number.NEGATIVE_INFINITY;
        return note.search_spend;
      };
      return sortValue(right) - sortValue(left) || left.note_id.localeCompare(right.note_id);
    });
  }, [costFilters, feedCostLimit, includeUnlabeled, noteSort, result, searchCostLimit]);
  const notePageCount = Math.max(1, Math.ceil(visibleNotes.length / NOTE_PAGE_SIZE));
  const pagedVisibleNotes = useMemo(() => {
    const start = (notePage - 1) * NOTE_PAGE_SIZE;
    return visibleNotes.slice(start, start + NOTE_PAGE_SIZE);
  }, [notePage, visibleNotes]);
  const unqualifiedCounts = useMemo(() => {
    const notesByID = new Map<string, ContentNote>();
    for (const cell of result?.cells ?? []) {
      if (!includeUnlabeled && (cell.content_type === "未标注" || cell.dimension === "未标注")) continue;
      for (const note of cell.notes) notesByID.set(note.note_id, note);
    }
    const notes = Array.from(notesByID.values());
    return {
      search: notes.filter((note) => costExceeds(note.search_cost, searchCostLimit)).length,
      feed: notes.filter((note) => costExceeds(note.feed_cost, feedCostLimit)).length,
      searchStopped: notes.filter((note) => note.search_stopped).length,
      feedStopped: notes.filter((note) => note.feed_stopped).length
    };
  }, [feedCostLimit, includeUnlabeled, result, searchCostLimit]);
  const sortLabel = NOTE_SORT_OPTIONS.find((option) => option.value === noteSort)?.label ?? "搜索累计消耗";

  const toggleCostFilter = (value: CostFilter) => {
    setCostFilters((current) => current.includes(value) ? current.filter((item) => item !== value) : [...current, value]);
  };

  useEffect(() => {
    setNotePage(1);
  }, [agency, costFilters, dimension, feedCostLimit, includeUnlabeled, noteSort, publishedEndDate, publishedStartDate, searchCostLimit, spu]);

  useEffect(() => {
    setNotePage((current) => Math.min(current, notePageCount));
  }, [notePageCount]);

  const changeNotePage = (page: number) => {
    setNotePage(page);
    window.requestAnimationFrame(() => {
      noteSectionRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  };
  const total = result?.coverage.total_notes ?? 0;
  const qualityItems = result ? [
    { label: "内容类型覆盖", value: result.coverage.content_type_tagged, display: coverageRate(result.coverage.content_type_tagged, total) },
    { label: "人群标签覆盖", value: result.coverage.audience_tagged, display: coverageRate(result.coverage.audience_tagged, total) },
    { label: "用户场景覆盖", value: result.coverage.scenario_tagged, display: coverageRate(result.coverage.scenario_tagged, total) },
    { label: "有效站外成本", value: result.coverage.dandelion_cost_notes, display: integer.format(result.coverage.dandelion_cost_notes) },
    { label: "可评估投流", value: result.coverage.flow_evaluated_notes, display: integer.format(result.coverage.flow_evaluated_notes) },
    { label: "可评估 ROI", value: result.coverage.roi_evaluated_notes, display: integer.format(result.coverage.roi_evaluated_notes) }
  ] : [];

  return <>
    <section className="page-heading content-page-heading">
      <div><h1>内容分析</h1><p>{spu} · 内容类型 × {dimension === "audience" ? "人群标签" : "用户场景"}</p></div>
      <div className="content-heading-actions">
        <div className="segmented-control content-spu-control" aria-label="内容分析 SPU">
          {SPU_OPTIONS.map((option) => <button type="button" className={spu === option ? "active" : ""} key={option} onClick={() => setSPU(option)}>{option}</button>)}
        </div>
        <div className="heading-status"><span className={"status-dot " + serviceState} />{serviceState === "online" ? "数据服务已连接" : serviceState === "offline" ? "数据服务未连接" : "正在检查连接"}</div>
      </div>
    </section>

    <section className="content-toolbar">
      <div className="content-filter-group"><span>机构</span><div className="segmented-control content-agency-control" aria-label="内容分析机构">
        {AGENCY_OPTIONS.map((option) => <button type="button" className={agency === option ? "active" : ""} key={option} onClick={() => setAgency(option)}>{option === "全部" ? "全部机构" : option}</button>)}
      </div></div>
      <div className="content-filter-group"><span>矩阵列</span><div className="segmented-control content-dimension-control" aria-label="热力图维度">
        <button type="button" className={dimension === "audience" ? "active" : ""} onClick={() => setDimension("audience")}>人群标签</button>
        <button type="button" className={dimension === "scenario" ? "active" : ""} onClick={() => setDimension("scenario")}>用户场景</button>
      </div></div>
      <div className="content-filter-group content-date-filter"><span>发布时间</span><div className="content-date-range">
        <input type="date" aria-label="发布时间开始" value={publishedStartDate} max={publishedEndDate || undefined} onChange={(event) => setPublishedStartDate(event.target.value)} />
        <span aria-hidden="true">至</span>
        <input type="date" aria-label="发布时间结束" value={publishedEndDate} min={publishedStartDate || undefined} onChange={(event) => setPublishedEndDate(event.target.value)} />
        <button type="button" title="清除发布时间范围" aria-label="清除发布时间范围" disabled={!publishedStartDate && !publishedEndDate} onClick={() => { setPublishedStartDate(""); setPublishedEndDate(""); }}><X size={14} /></button>
      </div></div>
      <label className="content-unlabeled-toggle"><input type="checkbox" checked={includeUnlabeled} onChange={(event) => setIncludeUnlabeled(event.target.checked)} />包含未标注</label>
    </section>

    {error ? <div className="analysis-error content-error"><AlertCircle size={17} />{error}</div> : null}
    {loading ? <div className="content-loading"><LoaderCircle size={19} className="spin" />正在汇总内容表现</div> : null}

    {!loading && result ? <>
      <section className="content-source-strip">
        <div><span>蒲公英数据</span><strong>{result.sources.dandelion_data_date || "--"}</strong></div>
        <div><span>Maituo 客户日报</span><strong>{result.sources.maituo_report_date || "--"}</strong></div>
        <div><span>薯量 ROI 快照</span><strong>{result.sources.guorai_snapshot_date || "--"}</strong></div>
        <div><span>薯量窗口</span><strong>{result.sources.guorai_window_start && result.sources.guorai_window_end ? result.sources.guorai_window_start + " 至 " + result.sources.guorai_window_end : "--"}</strong></div>
        <div><span>笔记总数</span><strong>{integer.format(total)}</strong></div>
      </section>

      <section className="content-quality-strip">
        {qualityItems.map((item) => <div key={item.label}><span>{item.label}</span><strong>{item.display}</strong><small>{integer.format(item.value)} / {integer.format(total)}</small></div>)}
      </section>

      <section className="content-heatmap-section">
        <header><div><h2>内容表现热力图</h2><p>爆文：蒲公英站外活跃成本（15天设备归因）大于 0 且不高于 20</p></div><span>{agency === "全部" ? "曼杰 · 有一有二 · 智元" : agency}</span></header>
        {types.length === 0 || dimensions.length === 0 ? <div className="content-empty">当前筛选条件下暂无完整内容标签</div> : <div className="content-heatmap-scroll">
          <div className="content-heatmap" style={{ gridTemplateColumns: "126px repeat(" + dimensions.length + ", 142px)" }}>
            <div className="content-axis-label corner">内容类型</div>
            {dimensions.map((value) => <div className="content-axis-label top" key={value} title={value}>{value}</div>)}
            {types.flatMap((contentType) => [
              <div className="content-axis-label side" key={"type-" + contentType}>{contentType}</div>,
              ...dimensions.map((dimensionValue) => {
                const cell = cells.get(contentType + "\u0000" + dimensionValue);
                if (!cell) return <div className="content-heat-cell no-data empty" key={contentType + "-" + dimensionValue}><strong>--</strong><span>无样本</span></div>;
                return <button type="button" className={"content-heat-cell " + rateClass(cell) + (cell.dandelion_eligible > 0 && cell.dandelion_eligible < 3 ? " small-sample" : "")} key={contentType + "-" + dimensionValue} onClick={() => setSelectedCell(cell)} aria-label={contentType + " " + dimensionValue + " 爆文率" + percentage(cell.boom_rate)}>
                  <strong>{percentage(cell.boom_rate)}</strong>
                  <span>爆文 {cell.boom_count}/{cell.dandelion_eligible}</span>
                  <small>投流 {cell.flow_qualified} · ROI {cell.roi_qualified}</small>
                  <small>三项 {cell.all_qualified} · 总 {cell.total_notes}</small>
                </button>;
              })
            ])}
          </div>
        </div>}
        <footer className="content-heat-legend">
          <span>爆文率</span><i className="legend-rate-1" /><i className="legend-rate-2" /><i className="legend-rate-3" /><i className="legend-rate-4" /><i className="legend-rate-5" /><span>低 → 高</span>
          <em>斜纹：有效成本样本少于 3</em><em>灰色：无有效成本</em>
        </footer>
      </section>

      <section className="content-note-section" ref={noteSectionRef}>
        <header>
          <div><h2>笔记表现</h2><p>默认按搜索累计消耗降序；回搜成本变化 = 当日回搜成本 − 累计回搜成本</p></div>
          <span>{integer.format(visibleNotes.length)} 篇笔记</span>
        </header>
        <div className="content-note-controls">
          <div className="content-note-sort">
            <ArrowDownWideNarrow size={14} />
            <span>排序</span>
            <div className="content-note-sort-buttons" aria-label="笔记排序方式">
              {NOTE_SORT_OPTIONS.map((option) => <button type="button" className={noteSort === option.value ? "active" : ""} aria-pressed={noteSort === option.value} key={option.value} title={option.description} onClick={() => setNoteSort(option.value)}>{option.label}</button>)}
            </div>
          </div>
          <div className="content-note-filters" aria-label="笔记表现筛选">
            <button type="button" className={"content-note-filter-card" + (costFilters.includes("search") ? " active" : "")} aria-pressed={costFilters.includes("search")} onClick={() => toggleCostFilter("search")}>
              <span>搜索成本不达标</span>
              <strong>{integer.format(unqualifiedCounts.search)}</strong>
              <small>累计回搜成本 &gt; {searchCostLimit}</small>
            </button>
            <button type="button" className={"content-note-filter-card" + (costFilters.includes("feed") ? " active" : "")} aria-pressed={costFilters.includes("feed")} onClick={() => toggleCostFilter("feed")}>
              <span>信息流成本不达标</span>
              <strong>{integer.format(unqualifiedCounts.feed)}</strong>
              <small>累计成本 &gt; {feedCostLimit}</small>
            </button>
            <button type="button" className={"content-note-filter-card" + (costFilters.includes("search_stopped") ? " active" : "")} aria-pressed={costFilters.includes("search_stopped")} onClick={() => toggleCostFilter("search_stopped")}>
              <span>搜索已停投</span>
              <strong>{integer.format(unqualifiedCounts.searchStopped)}</strong>
              <small>近一天搜索消耗为 0</small>
            </button>
            <button type="button" className={"content-note-filter-card" + (costFilters.includes("feed_stopped") ? " active" : "")} aria-pressed={costFilters.includes("feed_stopped")} onClick={() => toggleCostFilter("feed_stopped")}>
              <span>信息流已停投</span>
              <strong>{integer.format(unqualifiedCounts.feedStopped)}</strong>
              <small>近一天信息流消耗为 0</small>
            </button>
            <label className="content-note-threshold">搜索阈值<input type="number" min="0" step="1" aria-label="搜索成本不达标阈值" value={searchCostLimitInput} onChange={(event) => setSearchCostLimitInput(event.target.value)} /></label>
            <label className="content-note-threshold">信息流阈值<input type="number" min="0" step="1" aria-label="信息流成本不达标阈值" value={feedCostLimitInput} onChange={(event) => setFeedCostLimitInput(event.target.value)} /></label>
          </div>
        </div>
        {visibleNotes.length === 0 ? <div className="content-note-section-empty">当前筛选条件下暂无笔记</div> : <>
          <ContentNoteTable notes={pagedVisibleNotes} showStatus={false} label={"按" + sortLabel + "排序的笔记"} />
          <nav className="content-note-pagination" aria-label="笔记分页">
            <span>共 {integer.format(visibleNotes.length)} 篇 · 每页 {NOTE_PAGE_SIZE} 篇</span>
            <div className="content-note-pagination-controls">
              <button type="button" title="上一页" aria-label="上一页" disabled={notePage === 1} onClick={() => changeNotePage(notePage - 1)}><ChevronLeft size={15} /></button>
              <select aria-label="选择笔记页码" value={notePage} onChange={(event) => changeNotePage(Number(event.target.value))}>
                {Array.from({ length: notePageCount }, (_, index) => <option value={index + 1} key={index + 1}>第 {index + 1} / {notePageCount} 页</option>)}
              </select>
              <button type="button" title="下一页" aria-label="下一页" disabled={notePage === notePageCount} onClick={() => changeNotePage(notePage + 1)}><ChevronRight size={15} /></button>
            </div>
          </nav>
        </>}
      </section>
    </> : null}

    {selectedCell ? <ContentDetailDrawer cell={selectedCell} dimension={dimension} onClose={() => setSelectedCell(null)} /> : null}
  </>;
}

export default ContentAnalysis;
