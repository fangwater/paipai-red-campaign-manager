import { useEffect, useMemo, useRef, useState } from "react";
import { LineChart } from "echarts/charts";
import { GridComponent, TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { AlertCircle, ArrowDownWideNarrow, ChevronLeft, ChevronRight, LoaderCircle, Search } from "lucide-react";

echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

type WindowOption = "3d" | "7d" | "all";
type SortOption = "daily_spend" | "cumulative_spend";

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
  campaign_name: string;
  placement: string;
  first_report_date: string;
  last_report_date: string;
  active_days: number;
  latest_spend: number;
  total_spend: number;
  total_search_users: number;
  latest_search_cost: number;
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

type ServiceState = "checking" | "online" | "offline";

const EMPTY_RESULT: AnalysisResult = { window: "7d", sort: "cumulative_spend", report_dates: [], total: 0, page: 1, page_size: 25, items: [] };
const WINDOW_OPTIONS: Array<{ value: WindowOption; label: string }> = [
  { value: "3d", label: "3D" },
  { value: "7d", label: "7D" },
  { value: "all", label: "全部" }
];
const SORT_OPTIONS: Array<{ value: SortOption; label: string }> = [
  { value: "daily_spend", label: "当天消耗" },
  { value: "cumulative_spend", label: "累计消耗" }
];

const moneyFormatter = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const countFormatter = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });

function itemKey(item: AnalysisItem): string {
  return `${item.note_id}\u0000${item.campaign_name}\u0000${item.placement}`;
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

function NoteCampaignAnalysis({ serviceState }: { serviceState: ServiceState }) {
  const [windowOption, setWindowOption] = useState<WindowOption>("7d");
  const [sortOption, setSortOption] = useState<SortOption>("cumulative_spend");
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<AnalysisResult>(EMPTY_RESULT);
  const [selectedKey, setSelectedKey] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearchQuery(searchInput.trim());
      setPage(1);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ window: windowOption, sort: sortOption, page: String(page), page_size: "25" });
    if (searchQuery) params.set("q", searchQuery);
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
  }, [page, searchQuery, sortOption, windowOption]);

  const selected = result.items.find((item) => itemKey(item) === selectedKey) ?? result.items[0];
  const pageCount = Math.max(1, Math.ceil(result.total / result.page_size));
  const dateRange = result.report_dates.length > 0
    ? `${result.report_dates[0]} - ${result.report_dates[result.report_dates.length - 1]}`
    : "暂无报表日期";

  return <>
    <section className="page-heading analysis-page-heading">
      <div><h1>笔记计划分析</h1><p>笔记ID + 计划 + 场域</p></div>
      <div className="heading-status"><span className={`status-dot ${serviceState}`} />{serviceState === "online" ? "分析服务已连接" : serviceState === "offline" ? "分析服务未连接" : "正在检查连接"}</div>
    </section>

    <section className="analysis-toolbar">
      <label className="analysis-search"><Search size={16} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索笔记ID、计划或场域" /></label>
      <div className="analysis-range"><span>{dateRange} · {result.report_dates.length} 个报表日</span><div className="segmented-control" aria-label="分析时间范围">
        {WINDOW_OPTIONS.map((option) => <button key={option.value} className={windowOption === option.value ? "active" : ""} onClick={() => { setWindowOption(option.value); setPage(1); }}>{option.label}</button>)}
      </div></div>
    </section>

    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <section className="analysis-focus">
      {loading && !selected ? <div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在读取分析数据</div>
        : selected ? <>
          <div className="focus-identity"><span className={`placement-swatch placement-${selected.placement}`}>{selected.placement}</span><strong>{selected.campaign_name}</strong><small>{selected.note_id}</small></div>
          <div className="metric-chart-grid">
            <MetricChart title="累计消耗" value={`¥${moneyFormatter.format(selected.total_spend)}`} color="#2f7d67" dates={selected.points.map((point) => point.report_date)} values={selected.points.map((point) => point.cumulative_spend)} />
            <MetricChart title="累计回搜人数" value={countFormatter.format(selected.total_search_users)} color="#c94e55" dates={selected.points.map((point) => point.report_date)} values={selected.points.map((point) => point.cumulative_search_users)} />
            <MetricChart title="回搜成本" value={"¥" + moneyFormatter.format(selected.latest_search_cost)} color="#b5852d" dates={selected.points.map((point) => point.report_date)} values={selected.points.map((point) => point.search_cost)} />
          </div>
        </> : <div className="analysis-loading">没有符合条件的笔记计划</div>}
    </section>

    <section className="analysis-table-section">
      <header><div className="analysis-table-title"><h2>笔记计划列表</h2><p>{result.total.toLocaleString()} 个组合，按{sortOption === "daily_spend" ? "当天消耗" : "累计消耗"}排序</p></div><div className="analysis-table-actions"><ArrowDownWideNarrow size={15} /><span>排序</span><div className="sort-segmented" aria-label="笔记排序方式">
        {SORT_OPTIONS.map((option) => <button key={option.value} className={sortOption === option.value ? "active" : ""} onClick={() => { setSortOption(option.value); setPage(1); }}>{option.label}</button>)}
      </div>{loading ? <LoaderCircle size={18} className="spin" /> : null}</div></header>
      <div className="analysis-table-wrap"><table className="analysis-table"><thead><tr><th>笔记ID</th><th>计划</th><th>场域</th><th>投放天数</th><th>当天消耗</th><th>累计消耗</th><th>累计回搜人数</th><th>当天回搜成本</th></tr></thead><tbody>
        {result.items.map((item) => <tr key={itemKey(item)} className={itemKey(item) === itemKey(selected ?? item) ? "selected" : ""} onClick={() => setSelectedKey(itemKey(item))}>
          <td title={item.note_id}><strong>{item.note_id}</strong></td><td title={item.campaign_name}>{item.campaign_name}</td><td><span className={`placement-swatch placement-${item.placement}`}>{item.placement}</span></td><td>{item.active_days}/{result.report_dates.length}</td><td>¥{moneyFormatter.format(item.latest_spend)}</td><td>¥{moneyFormatter.format(item.total_spend)}</td><td>{countFormatter.format(item.total_search_users)}</td><td>¥{moneyFormatter.format(item.latest_search_cost)}</td>
        </tr>)}
      </tbody></table></div>
      <footer className="analysis-pagination"><span>第 {result.page}/{pageCount} 页</span><div><button className="icon-button" title="上一页" aria-label="上一页" disabled={page <= 1 || loading} onClick={() => setPage((current) => Math.max(1, current - 1))}><ChevronLeft size={17} /></button><button className="icon-button" title="下一页" aria-label="下一页" disabled={page >= pageCount || loading} onClick={() => setPage((current) => current + 1)}><ChevronRight size={17} /></button></div></footer>
    </section>
  </>;
}

export default NoteCampaignAnalysis;
