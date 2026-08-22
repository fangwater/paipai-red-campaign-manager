import { useEffect, useMemo, useRef, useState } from "react";
import { LineChart } from "echarts/charts";
import { GridComponent, TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { AlertCircle, ArrowDownWideNarrow, CheckCircle2, ChevronLeft, ChevronRight, ExternalLink, FileSearch, Image as ImageIcon, Link2, LoaderCircle, Search, Tags as TagsIcon, X, ZoomIn } from "lucide-react";
import { useSearchParams } from "react-router-dom";

echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

type WindowOption = "3d" | "7d" | "all";
type SortOption = "daily_spend" | "cumulative_spend" | "search_cost_change";

type AnalysisPoint = {
  report_date: string;
  spend: number;
  search_users: number;
  search_cost: number;
  cumulative_spend: number;
  cumulative_search_users: number;
};

type AnalysisItem = {
  note_id: string;
  placement: string;
  first_report_date: string;
  last_report_date: string;
  active_days: number;
  latest_spend: number;
  total_spend: number;
  total_search_users: number;
  latest_search_cost: number;
  search_cost_change: number;
  points: AnalysisPoint[];
};

type AnalysisResult = {
  window: WindowOption;
  sort: SortOption;
  report_dates: string[];
  total: number;
  page: number;
  page_size: number;
  items: AnalysisItem[];
};

type ManuscriptBlock = {
  type: "paragraph" | "heading" | "bullet" | "ordered" | "quote" | "code" | "todo" | "equation" | "divider" | "image";
  text?: string;
  level?: number;
  asset_id?: string;
  width?: number;
  height?: number;
  caption?: string;
};

type NoteTags = {
  note_type: string[];
  cover_type: string[];
  commercial_intensity: string[];
  audience: string[];
  user_scenario: string[];
  progress: string[];
  complete: boolean;
  missing_fields: string[];
};

type NoteContentResult = {
  note_id: string;
  note_url: string;
  found: boolean;
  note_content: string;
  blocks?: ManuscriptBlock[];
  providers: string[];
  tags?: NoteTags;
};

type ZoomedImage = { src: string; caption: string };

type ServiceState = "checking" | "online" | "offline";

const EMPTY_RESULT: AnalysisResult = { window: "7d", sort: "cumulative_spend", report_dates: [], total: 0, page: 1, page_size: 25, items: [] };
const WINDOW_OPTIONS: Array<{ value: WindowOption; label: string }> = [
  { value: "3d", label: "3D" },
  { value: "7d", label: "7D" },
  { value: "all", label: "全部" }
];
const SORT_OPTIONS: Array<{ value: SortOption; label: string }> = [
  { value: "daily_spend", label: "当天消耗" },
  { value: "cumulative_spend", label: "累计消耗" },
  { value: "search_cost_change", label: "回搜成本差值" }
];

const moneyFormatter = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const countFormatter = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });

function formatCostChange(value: number): string {
  if (value > 0) return "+¥" + moneyFormatter.format(value);
  if (value < 0) return "-¥" + moneyFormatter.format(Math.abs(value));
  return "¥" + moneyFormatter.format(0);
}

function itemKey(item: AnalysisItem): string {
  return `${item.note_id}\u0000${item.placement}`;
}

function compactNumber(value: number): string {
  if (Math.abs(value) >= 10000) return `${(value / 10000).toFixed(1)}万`;
  if (Math.abs(value) >= 1000) return `${(value / 1000).toFixed(1)}k`;
  return value.toFixed(value % 1 === 0 ? 0 : 1);
}

type MetricChartProps = {
  title: string;
  value: string;
  color: string;
  dates: string[];
  values: number[];
};

function MetricChart({ title, value, color, dates, values }: MetricChartProps) {
  const chartRef = useRef<HTMLDivElement>(null);
  const option = useMemo<EChartsCoreOption>(() => ({
    animationDuration: 320,
    grid: { left: 48, right: 18, top: 18, bottom: 36 },
    tooltip: {
      trigger: "axis",
      backgroundColor: "rgba(31, 35, 38, 0.94)",
      borderWidth: 0,
      padding: [8, 10],
      textStyle: { color: "#fff", fontSize: 11 }
    },
    xAxis: {
      type: "category",
      boundaryGap: false,
      data: dates.map((date) => date.slice(5)),
      axisLine: { lineStyle: { color: "#dfe3e5" } },
      axisTick: { show: false },
      axisLabel: { color: "#858c92", fontSize: 10, interval: dates.length > 12 ? "auto" : 0 }
    },
    yAxis: {
      type: "value",
      min: 0,
      splitNumber: 4,
      axisLabel: { color: "#858c92", fontSize: 10, formatter: (axisValue: number) => compactNumber(axisValue) },
      splitLine: { lineStyle: { color: "#edf0f1", type: "dashed" } }
    },
    series: [{
      name: title,
      type: "line",
      data: values,
      smooth: 0.22,
      showSymbol: dates.length <= 7,
      symbol: "circle",
      symbolSize: 6,
      lineStyle: { width: 3, color },
      itemStyle: { color, borderColor: "#fff", borderWidth: 2 },
      emphasis: { focus: "series", scale: 1.25 }
    }]
  }), [color, dates, title, values]);

  useEffect(() => {
    if (!chartRef.current) return;
    const chart = echarts.init(chartRef.current, undefined, { renderer: "canvas" });
    chart.setOption(option);
    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(chartRef.current);
    return () => {
      observer.disconnect();
      chart.dispose();
    };
  }, [option]);

  return <article className="metric-chart">
    <header><span>{title}</span><strong>{value}</strong></header>
    <div className="metric-chart-canvas" ref={chartRef} role="img" aria-label={title + "折线图"} />
  </article>;
}

function manuscriptAssetURL(assetID: string | undefined): string | null {
  if (!assetID || !/^[0-9a-f]{64}$/.test(assetID)) return null;
  return `${import.meta.env.BASE_URL}api/manuscript-assets/${assetID}`;
}

function ManuscriptImage({ block, onZoom }: { block: ManuscriptBlock; onZoom: (image: ZoomedImage) => void }) {
  const [failed, setFailed] = useState(false);
  const src = manuscriptAssetURL(block.asset_id);
  const caption = block.caption?.trim() ?? "";
  if (!src || failed) {
    return <div className="manuscript-image-error" role="status"><ImageIcon size={18} /><span>图片加载失败</span></div>;
  }
  return <figure className="manuscript-image">
    <button type="button" aria-label={caption ? `查看大图：${caption}` : "查看稿件大图"} onClick={() => onZoom({ src, caption })}>
      <img
        src={src}
        alt={caption || "稿件图片"}
        loading="lazy"
        decoding="async"
        width={block.width && block.width > 0 ? block.width : undefined}
        height={block.height && block.height > 0 ? block.height : undefined}
        onError={() => setFailed(true)}
      />
      <span className="manuscript-image-zoom" aria-hidden="true"><ZoomIn size={15} /></span>
    </button>
    {caption ? <figcaption>{caption}</figcaption> : null}
  </figure>;
}

function ManuscriptContent({ blocks, fallback, onZoom }: {
  blocks: ManuscriptBlock[];
  fallback: string;
  onZoom: (image: ZoomedImage) => void;
}) {
  if (blocks.length === 0) return <pre>{fallback}</pre>;
  return <div className="manuscript-blocks">{blocks.map((block, index) => {
    const key = `${index}-${block.type}-${block.asset_id ?? ""}`;
    switch (block.type) {
    case "image":
      return <ManuscriptImage key={key} block={block} onZoom={onZoom} />;
    case "heading":
      return <h3 className={`manuscript-heading level-${block.level ?? 0}`} key={key}>{block.text}</h3>;
    case "bullet":
    case "ordered":
    case "todo":
      return <div className={`manuscript-list-item ${block.type}`} key={key}><span aria-hidden="true" /><p>{block.text}</p></div>;
    case "quote":
      return <blockquote key={key}>{block.text}</blockquote>;
    case "code":
      return <pre className="manuscript-code" key={key}>{block.text}</pre>;
    case "divider":
      return <hr key={key} />;
    default:
      return <p key={key}>{block.text}</p>;
    }
  })}</div>;
}

type NoteTagField = "note_type" | "cover_type" | "commercial_intensity" | "audience" | "user_scenario" | "progress";

const NOTE_TAG_FIELDS: Array<{ key: NoteTagField; label: string }> = [
  { key: "note_type", label: "内容类型" },
  { key: "audience", label: "对话人群" },
  { key: "user_scenario", label: "用户场景" },
  { key: "cover_type", label: "封面类型" },
  { key: "commercial_intensity", label: "商业强度" },
  { key: "progress", label: "进度" }
];

function NoteTagsPanel({ tags }: { tags: NoteTags | undefined }) {
  const complete = tags?.complete === true;
  return <section className="note-content-tags" aria-label="稿件标签">
    <div className="note-content-tags-heading">
      <div><TagsIcon size={15} /><span>标签信息</span></div>
      <span className={`note-tags-completeness ${complete ? "complete" : "incomplete"}`} role="status">
        {complete ? <CheckCircle2 size={13} /> : <AlertCircle size={13} />}
        {complete ? "标签完整" : "标签待补充"}
      </span>
    </div>
    <dl>{NOTE_TAG_FIELDS.map((field) => {
      const values = tags?.[field.key] ?? [];
      return <div className={values.length === 0 ? "missing" : ""} key={field.key}>
        <dt>{field.label}</dt>
        <dd>{values.length > 0
          ? values.map((value) => <span className="note-tag-value" key={`${field.key}-${value}`}>{value}</span>)
          : <span className="note-tag-value missing">待补充</span>}
        </dd>
      </div>;
    })}</dl>
  </section>;
}

function NoteCampaignAnalysis({ serviceState }: { serviceState: ServiceState }) {
  const [routeParams, setRouteParams] = useSearchParams();
  const planID = routeParams.get("plan_id")?.trim() ?? "";
  const planName = routeParams.get("plan_name")?.trim() ?? "";
  const initialWindow = routeParams.get("window");
  const initialSearch = routeParams.get("q")?.trim() ?? "";
  const [windowOption, setWindowOption] = useState<WindowOption>(initialWindow === "3d" || initialWindow === "all" ? initialWindow : "7d");
  const [sortOption, setSortOption] = useState<SortOption>("cumulative_spend");
  const [searchInput, setSearchInput] = useState(initialSearch);
  const [searchQuery, setSearchQuery] = useState(initialSearch);
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<AnalysisResult>(EMPTY_RESULT);
  const [selectedKey, setSelectedKey] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [contentOpen, setContentOpen] = useState(false);
  const [contentLoading, setContentLoading] = useState(false);
  const [contentError, setContentError] = useState("");
  const [noteContent, setNoteContent] = useState<NoteContentResult | null>(null);

  const [zoomedImage, setZoomedImage] = useState<ZoomedImage | null>(null);
  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearchQuery(searchInput.trim());
      setPage(1);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    if (!contentOpen) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (zoomedImage) {
        setZoomedImage(null);
        return;
      }
      setContentOpen(false);
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [contentOpen, zoomedImage]);

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ window: windowOption, sort: sortOption, page: String(page), page_size: "25" });
    if (searchQuery) params.set("q", searchQuery);
    if (planID) params.set("plan_id", planID);
    setLoading(true);
    setError("");
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/note-campaigns?${params}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: AnalysisResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "分析数据读取失败");
        setResult(payload.data);
        setSelectedKey((current) => payload.data?.items.some((item) => itemKey(item) === current)
          ? current
          : payload.data?.items[0] ? itemKey(payload.data.items[0]) : "");
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "分析数据读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [page, planID, searchQuery, sortOption, windowOption]);

  const selected = result.items.find((item) => itemKey(item) === selectedKey) ?? result.items[0];
  const pageCount = Math.max(1, Math.ceil(result.total / result.page_size));
  const dateRange = result.report_dates.length > 0
    ? `${result.report_dates[0]} - ${result.report_dates[result.report_dates.length - 1]}`
    : "暂无报表日期";
  const sortLabel = SORT_OPTIONS.find((option) => option.value === sortOption)?.label ?? "累计消耗";
  const latestDate = result.report_dates[result.report_dates.length - 1];
  const previousDate = result.report_dates[result.report_dates.length - 2];
  const costChangeTitle = latestDate && previousDate ? latestDate + " 回搜成本 - " + previousDate + " 回搜成本" : "暂无前一报表日";

  const queryNoteContent = async (noteID: string) => {
    setContentOpen(true);
    setContentLoading(true);
    setContentError("");
    setNoteContent(null);
    try {
      const params = new URLSearchParams({ note_id: noteID });
      const response = await fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/note-content?${params}`);
      const payload = await response.json() as { success: boolean; data?: NoteContentResult; error?: string };
      if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "笔记内容读取失败");
      setNoteContent(payload.data);
    } catch (fetchError) {
      setContentError(fetchError instanceof Error ? fetchError.message : "笔记内容读取失败");
    } finally {
      setContentLoading(false);
    }
  };

  const clearPlanFilter = () => {
    const next = new URLSearchParams(routeParams);
    next.delete("plan_id");
    next.delete("plan_name");
    setRouteParams(next, { replace: true });
    setPage(1);
  };

  return <>
    <section className="page-heading analysis-page-heading">
      <div><h1>笔记场域分析</h1><p>笔记ID + 场域</p></div>
      <div className="heading-status"><span className={`status-dot ${serviceState}`} />{serviceState === "online" ? "分析服务已连接" : serviceState === "offline" ? "分析服务未连接" : "正在检查连接"}</div>
    </section>

    <section className="analysis-toolbar">
      <div className="analysis-query-controls">
        <label className="analysis-search"><Search size={16} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索笔记ID或场域" /></label>
        {planID ? <span className="analysis-plan-filter" title={`薯量计划关联：${planName || planID}`}><Link2 size={13} /><span>薯量计划关联：{planName || planID}</span><button title="清除薯量计划关联筛选" aria-label="清除薯量计划关联筛选" onClick={clearPlanFilter}><X size={13} /></button></span> : null}
      </div>
      <div className="analysis-range"><span>{dateRange} · {result.report_dates.length} 个报表日</span><div className="segmented-control" aria-label="分析时间范围">
        {WINDOW_OPTIONS.map((option) => <button key={option.value} className={windowOption === option.value ? "active" : ""} onClick={() => { setWindowOption(option.value); setPage(1); }}>{option.label}</button>)}
      </div></div>
    </section>

    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <section className="analysis-focus">
      {loading && !selected ? <div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在读取分析数据</div>
        : selected ? <>
          <div className="focus-identity"><span className={`placement-swatch placement-${selected.placement}`}>{selected.placement}</span><strong>{selected.note_id}</strong><button className="note-content-trigger" onClick={() => void queryNoteContent(selected.note_id)} disabled={contentLoading && contentOpen}>{contentLoading && contentOpen ? <LoaderCircle size={15} className="spin" /> : <FileSearch size={15} />}查询内容</button></div>
          <div className="metric-chart-grid">
            <MetricChart title="累计消耗" value={`¥${moneyFormatter.format(selected.total_spend)}`} color="#2f7d67" dates={selected.points.map((point) => point.report_date)} values={selected.points.map((point) => point.cumulative_spend)} />
            <MetricChart title="累计回搜人数" value={countFormatter.format(selected.total_search_users)} color="#c94e55" dates={selected.points.map((point) => point.report_date)} values={selected.points.map((point) => point.cumulative_search_users)} />
            <MetricChart title="回搜成本" value={"¥" + moneyFormatter.format(selected.latest_search_cost)} color="#b5852d" dates={selected.points.map((point) => point.report_date)} values={selected.points.map((point) => point.search_cost)} />
          </div>
        </> : <div className="analysis-loading">没有符合条件的笔记场域</div>}
    </section>

    <section className="analysis-table-section">
      <header><div className="analysis-table-title"><h2>笔记场域列表</h2><p>{result.total.toLocaleString()} 个组合，按{sortLabel}降序</p></div><div className="analysis-table-actions"><ArrowDownWideNarrow size={15} /><span>排序</span><div className="sort-segmented" aria-label="笔记排序方式">
        {SORT_OPTIONS.map((option) => <button key={option.value} className={sortOption === option.value ? "active" : ""} onClick={() => { setSortOption(option.value); setPage(1); }}>{option.label}</button>)}
      </div>{loading ? <LoaderCircle size={18} className="spin" /> : null}</div></header>
      <div className="analysis-table-wrap"><table className="analysis-table"><thead><tr><th>笔记ID</th><th>场域</th><th>投放天数</th><th>当天消耗</th><th>累计消耗</th><th>累计回搜人数</th><th>当天回搜成本</th><th title={costChangeTitle}>较前一日</th></tr></thead><tbody>
        {result.items.map((item) => <tr key={itemKey(item)} className={itemKey(item) === itemKey(selected ?? item) ? "selected" : ""} onClick={() => setSelectedKey(itemKey(item))}>
          <td title={item.note_id}><strong>{item.note_id}</strong></td><td><span className={`placement-swatch placement-${item.placement}`}>{item.placement}</span></td><td>{item.active_days}/{result.report_dates.length}</td><td>¥{moneyFormatter.format(item.latest_spend)}</td><td>¥{moneyFormatter.format(item.total_spend)}</td><td>{countFormatter.format(item.total_search_users)}</td><td>¥{moneyFormatter.format(item.latest_search_cost)}</td>
          <td className={"search-cost-change " + (item.search_cost_change > 0 ? "increase" : item.search_cost_change < 0 ? "decrease" : "")} title={costChangeTitle}>{formatCostChange(item.search_cost_change)}</td>
        </tr>)}
      </tbody></table></div>
      <footer className="analysis-pagination"><span>第 {result.page}/{pageCount} 页</span><div><button className="icon-button" title="上一页" aria-label="上一页" disabled={page <= 1 || loading} onClick={() => setPage((current) => Math.max(1, current - 1))}><ChevronLeft size={17} /></button><button className="icon-button" title="下一页" aria-label="下一页" disabled={page >= pageCount || loading} onClick={() => setPage((current) => current + 1)}><ChevronRight size={17} /></button></div></footer>
    </section>

    {contentOpen ? <div className="note-content-overlay" onMouseDown={(event) => {
      if (event.target === event.currentTarget) setContentOpen(false);
    }}>
      <section className="note-content-dialog" role="dialog" aria-modal="true" aria-labelledby="note-content-title">
        <header><div><h2 id="note-content-title">笔记内容</h2><span>{noteContent?.note_id || selected?.note_id || "--"}</span></div><button className="icon-button" title="关闭" aria-label="关闭笔记内容" onClick={() => setContentOpen(false)}><X size={18} /></button></header>
        <div className="note-content-body">
          {contentLoading ? <div className="note-content-state"><LoaderCircle size={18} className="spin" />正在读取稿件内容</div>
            : contentError ? <div className="note-content-state error"><AlertCircle size={18} />{contentError}</div>
              : noteContent?.found ? <>
                {noteContent.providers.length > 0 ? <div className="note-content-source"><span>来源机构</span><strong>{noteContent.providers.join("、")}</strong></div> : null}
                <NoteTagsPanel tags={noteContent.tags} />
                <ManuscriptContent blocks={noteContent.blocks ?? []} fallback={noteContent.note_content} onZoom={setZoomedImage} />
              </> : <div className="note-content-state">当前稿件库未收录这篇笔记的内容</div>}
        </div>
        <footer>{noteContent?.note_url ? <a href={noteContent.note_url} target="_blank" rel="noreferrer"><ExternalLink size={15} />打开小红书笔记</a> : <span />}</footer>
      </section>
    </div> : null}
    {zoomedImage ? <div className="manuscript-lightbox" role="dialog" aria-modal="true" aria-label="稿件大图" onMouseDown={(event) => {
      if (event.target === event.currentTarget) setZoomedImage(null);
    }}>
      <button className="icon-button" title="关闭" aria-label="关闭稿件大图" onClick={() => setZoomedImage(null)}><X size={19} /></button>
      <img src={zoomedImage.src} alt={zoomedImage.caption || "稿件大图"} />
      {zoomedImage.caption ? <p>{zoomedImage.caption}</p> : null}
    </div> : null}
  </>;
}

export default NoteCampaignAnalysis;
