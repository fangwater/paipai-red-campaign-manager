import { useEffect, useMemo, useRef, useState } from "react";
import { BarChart, LineChart } from "echarts/charts";
import { GridComponent, TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { AlertCircle, ArrowDownRight, ArrowUpRight, ExternalLink, LoaderCircle, Minus } from "lucide-react";

echarts.use([LineChart, BarChart, GridComponent, TooltipComponent, CanvasRenderer]);

type ServiceState = "checking" | "online" | "offline";
type PeriodDays = 7 | 14 | 30;
type SPUOption = "辅酶" | "磷虾油";

type MetricPoint = {
  date: string;
  value: number | null;
};

type OverviewMetric = {
  key: string;
  label: string;
  unit: "currency" | "count";
  current_value: number | null;
  previous_value: number | null;
  change_pct: number | null;
  points: MetricPoint[];
};

type SearchUserPlacementCoefficient = {
  placement: string;
  search_users: number;
  coefficient: number | null;
};

type SearchUserOverlapPoint = {
  report_date: string;
  placement_coefficients: SearchUserPlacementCoefficient[];
};

type OverviewNote = {
  note_id: string;
  title: string;
  url: string;
  author: string;
  published_date: string;
  agency: string;
  audience: string;
  note_type: string;
  content_tag: string;
};

type OverviewAgency = {
  agency: string;
  count: number;
  audience_tags: string[];
  notes: OverviewNote[];
};

type OverviewResult = {
  days: PeriodDays;
  spu: SPUOption;
  overlap_points: SearchUserOverlapPoint[];
  trend: {
    start_date: string;
    end_date: string;
    previous_start_date: string;
    previous_end_date: string;
    available_days: number;
    metrics: OverviewMetric[];
  };
  new_notes: {
    start_date: string;
    end_date: string;
    total: number;
    daily: Array<{ date: string; count: number }>;
    agencies: OverviewAgency[];
    source_synced_at: string | null;
  };
};
const SPU_OPTIONS: SPUOption[] = ["辅酶", "磷虾油"];

const PERIOD_OPTIONS: PeriodDays[] = [7, 14, 30];
const METRIC_COLORS: Record<string, string> = {
  spend: "#2f7d67",
  search_cost: "#c5545c",
  search_uv: "#2e6f9e",
  order_uv: "#b17b27"
};
const PLACEMENT_ORDER: string[] = ["信息流", "搜索", "视频内流"];
const PLACEMENT_COLORS: Record<string, string> = {
  信息流: "#c45a62",
  搜索: "#276f7c",
  视频内流: "#b17b27"
};
const PLACEMENT_FALLBACK_COLORS = ["#5f6f82", "#6f7153", "#8a5f72"];
const numberFormatter = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
const moneyFormatter = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

function compactNumber(value: number): string {
  if (Math.abs(value) >= 10000) return (value / 10000).toFixed(1) + "万";
  if (Math.abs(value) >= 1000) return (value / 1000).toFixed(1) + "k";
  return value.toFixed(value % 1 === 0 ? 0 : 1);
}

function metricValue(metric: OverviewMetric, value: number | null): string {
  if (value === null) return "--";
  return metric.unit === "currency" ? "¥" + moneyFormatter.format(value) : numberFormatter.format(value);
}

function periodLabel(metric: OverviewMetric): string {
  return metric.key === "search_cost" ? "本周期日均" : "本周期合计";
}

function ChangeBadge({ metric }: { metric: OverviewMetric }) {
  if (metric.change_pct === null) return <span className="overview-change neutral"><Minus size={13} />前周期暂无数据</span>;
  const rising = metric.change_pct > 0;
  const favorable = metric.key === "search_cost" ? !rising : rising;
  const Icon = rising ? ArrowUpRight : metric.change_pct < 0 ? ArrowDownRight : Minus;
  return <span className={"overview-change " + (metric.change_pct === 0 ? "neutral" : favorable ? "good" : "bad")}>
    <Icon size={13} />较前周期 {rising ? "+" : ""}{(metric.change_pct * 100).toFixed(1)}%
  </span>;
}

function TrendChart({ metric, days }: { metric: OverviewMetric; days: PeriodDays }) {
  const chartRef = useRef<HTMLDivElement>(null);
  const color = METRIC_COLORS[metric.key] ?? "#2f7d67";
  const option = useMemo<EChartsCoreOption>(() => ({
    animationDuration: 320,
    grid: { left: 54, right: 20, top: days === 7 ? 42 : 24, bottom: 38 },
    tooltip: {
      trigger: "axis",
      backgroundColor: "rgba(31, 35, 38, 0.94)",
      borderWidth: 0,
      padding: [8, 10],
      textStyle: { color: "#fff", fontSize: 11 },
      valueFormatter: (value: number) => metric.unit === "currency" ? "¥" + moneyFormatter.format(value) : numberFormatter.format(value)
    },
    xAxis: {
      type: "category",
      boundaryGap: false,
      data: metric.points.map((point) => point.date.slice(5)),
      axisLine: { lineStyle: { color: "#dfe3e5" } },
      axisTick: { show: false },
      axisLabel: { color: "#858c92", fontSize: 10, interval: days === 7 ? 0 : "auto" }
    },
    yAxis: {
      type: "value",
      scale: true,
      splitNumber: 4,
      axisLabel: { color: "#858c92", fontSize: 10, formatter: (value: number) => compactNumber(value) },
      splitLine: { lineStyle: { color: "#edf0f1", type: "dashed" } }
    },
    series: [{
      name: metric.label,
      type: "line",
      data: metric.points.map((point) => point.value),
      connectNulls: false,
      smooth: 0.18,
      showSymbol: days === 7,
      symbol: "circle",
      symbolSize: 7,
      lineStyle: { width: 3, color },
      itemStyle: { color, borderColor: "#fff", borderWidth: 2 },
      label: {
        show: days === 7,
        position: "top",
        distance: 7,
        color: "#50575d",
        fontSize: 9,
        formatter: (params: { value: number | null }) => params.value === null ? "" : compactNumber(params.value)
      },
      emphasis: { focus: "series", scale: 1.2 }
    }]
  }), [color, days, metric]);

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

  return <article className="overview-metric-card">
    <header>
      <div><span>{metric.label}</span><strong>{metricValue(metric, metric.current_value)}</strong><small>{periodLabel(metric)}</small></div>
      <ChangeBadge metric={metric} />
    </header>
    <div ref={chartRef} className="overview-chart-canvas" role="img" aria-label={metric.label + "折线图"} />
  </article>;
}

function OverlapTrendChart({ points, days }: { points: SearchUserOverlapPoint[]; days: PeriodDays }) {
  const chartRef = useRef<HTMLDivElement>(null);
  const placements = useMemo(() => {
    const names = new Set<string>();
    for (const point of points) {
      for (const item of point.placement_coefficients ?? []) {
        if (typeof item.coefficient === "number") names.add(item.placement);
      }
    }
    return [...names].sort((left, right) => {
      const leftIndex = PLACEMENT_ORDER.indexOf(left);
      const rightIndex = PLACEMENT_ORDER.indexOf(right);
      return (leftIndex < 0 ? Number.MAX_SAFE_INTEGER : leftIndex) - (rightIndex < 0 ? Number.MAX_SAFE_INTEGER : rightIndex)
        || left.localeCompare(right, "zh-CN");
    });
  }, [points]);
  const placementSeries = useMemo(() => placements.map((placement, index) => ({
    placement,
    label: placement + " ÷ SPU",
    color: PLACEMENT_COLORS[placement] ?? PLACEMENT_FALLBACK_COLORS[index % PLACEMENT_FALLBACK_COLORS.length]
  })), [placements]);
  const visiblePoints = useMemo(() => points.filter((point) => (point.placement_coefficients ?? []).some((item) => typeof item.coefficient === "number")), [points]);
  const option = useMemo<EChartsCoreOption>(() => ({
    animationDuration: 320,
    grid: { left: 54, right: 22, top: days === 7 ? 46 : 26, bottom: 38 },
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
      data: visiblePoints.map((point) => point.report_date.slice(5)),
      axisLine: { lineStyle: { color: "#dfe3e5" } },
      axisTick: { show: false },
      axisLabel: { color: "#858c92", fontSize: 10, interval: days === 7 ? 0 : "auto" }
    },
    yAxis: {
      type: "value",
      min: 0,
      splitNumber: 4,
      axisLabel: { color: "#858c92", fontSize: 10, formatter: (value: number) => `${value.toFixed(2)}×` },
      splitLine: { lineStyle: { color: "#edf0f1", type: "dashed" } }
    },
    series: placementSeries.map((series) => ({
      name: series.label,
      type: "line",
      data: visiblePoints.map((point) => point.placement_coefficients.find((item) => item.placement === series.placement)?.coefficient ?? null),
      connectNulls: true,
      smooth: 0.16,
      showSymbol: days === 7,
      symbol: "circle",
      symbolSize: 7,
      lineStyle: { width: 2.5, color: series.color },
      itemStyle: { color: series.color, borderColor: "#fff", borderWidth: 2 },
      tooltip: { valueFormatter: (value: number) => `${value.toFixed(2)}×` },
      label: {
        show: days === 7 && placementSeries.length === 1,
        position: "top",
        distance: 6,
        color: series.color,
        fontSize: 9,
        formatter: (params: { value: number | null }) => params.value === null ? "" : `${params.value.toFixed(2)}×`
      },
      emphasis: { focus: "series", scale: 1.15 }
    }))
  }), [days, placementSeries, visiblePoints]);

  useEffect(() => {
    if (!chartRef.current || visiblePoints.length === 0) return;
    const chart = echarts.init(chartRef.current, undefined, { renderer: "canvas" });
    chart.setOption(option);
    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(chartRef.current);
    return () => {
      observer.disconnect();
      chart.dispose();
    };
  }, [option, visiblePoints.length]);

  return <article className="overview-overlap-card">
    <header>
      <div><h3>场域 / SPU 回搜系数</h3><p>场域内子账户回搜人数合计 ÷ SPU 去重回搜人数</p></div>
      <div className="overview-overlap-legend">
        {placementSeries.map((series) => <span key={series.placement}><i style={{ background: series.color }} />{series.label}</span>)}
      </div>
    </header>
    {visiblePoints.length > 0
      ? <div ref={chartRef} className="overview-overlap-canvas" role="img" aria-label="场域与 SPU 回搜系数折线图" />
      : <div className="overview-overlap-empty">当前周期暂无场域系数</div>}
  </article>;
}

function NewNotesChart({ result }: { result: OverviewResult }) {
  const chartRef = useRef<HTMLDivElement>(null);
  const option = useMemo<EChartsCoreOption>(() => ({
    animationDuration: 320,
    grid: { left: 48, right: 18, top: 34, bottom: 38 },
    tooltip: {
      trigger: "axis",
      backgroundColor: "rgba(31, 35, 38, 0.94)",
      borderWidth: 0,
      textStyle: { color: "#fff", fontSize: 11 }
    },
    xAxis: {
      type: "category",
      data: result.new_notes.daily.map((item) => item.date.slice(5)),
      axisLine: { lineStyle: { color: "#dfe3e5" } },
      axisTick: { show: false },
      axisLabel: { color: "#858c92", fontSize: 10, interval: result.days === 7 ? 0 : "auto" }
    },
    yAxis: {
      type: "value",
      minInterval: 1,
      axisLabel: { color: "#858c92", fontSize: 10 },
      splitLine: { lineStyle: { color: "#edf0f1", type: "dashed" } }
    },
    series: [{
      name: "新增笔记",
      type: "bar",
      data: result.new_notes.daily.map((item) => item.count),
      barMaxWidth: 28,
      itemStyle: { color: "#c94e55", borderRadius: [3, 3, 0, 0] },
      label: { show: true, position: "top", color: "#596066", fontSize: 9 }
    }]
  }), [result]);

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

  return <div ref={chartRef} className="new-notes-chart" role="img" aria-label="每日新增笔记图" />;
}

function safeURL(value: string): string | null {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:" ? url.toString() : null;
  } catch {
    return null;
  }
}

function BusinessOverview({ serviceState }: { serviceState: ServiceState }) {
  const [days, setDays] = useState<PeriodDays>(7);
  const [spu, setSPU] = useState<SPUOption>("辅酶");
  const [result, setResult] = useState<OverviewResult | null>(null);
  const [selectedAgency, setSelectedAgency] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    const params = new URLSearchParams({ days: String(days), spu });
    fetch(import.meta.env.BASE_URL + "api/analytics/overview?" + params, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: OverviewResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "数据总览读取失败");
        setResult(payload.data);
        setSelectedAgency((current) => payload.data?.new_notes.agencies.some((agency) => agency.agency === current)
          ? current
          : payload.data?.new_notes.agencies.find((agency) => agency.count > 0)?.agency ?? payload.data?.new_notes.agencies[0]?.agency ?? "");
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "数据总览读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [days, spu]);

  const agency = result?.new_notes.agencies.find((item) => item.agency === selectedAgency) ?? result?.new_notes.agencies[0];

  return <>
    <section className="page-heading overview-page-heading">
      <div><h1>数据总览</h1><p>{spu} · Maituo 客户日报与蒲公英数据</p></div>
      <div className="overview-heading-actions">
        <div className="segmented-control overview-spu" aria-label="总览 SPU">
          {SPU_OPTIONS.map((option) => <button key={option} className={spu === option ? "active" : ""} onClick={() => setSPU(option)}>{option}</button>)}
        </div>
        <div className="segmented-control overview-period" aria-label="总览时间范围">
          {PERIOD_OPTIONS.map((option) => <button key={option} className={days === option ? "active" : ""} onClick={() => setDays(option)}>{option}日</button>)}
        </div>
        <div className="heading-status"><span className={"status-dot " + serviceState} />{serviceState === "online" ? "数据服务已连接" : serviceState === "offline" ? "数据服务未连接" : "正在检查连接"}</div>
      </div>
    </section>

    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    {loading && !result ? <div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在读取业务数据</div> : result ? <>
      <section className="overview-overlap-section">
        <header className="overview-section-heading">
          <div><h2>整体回搜系数</h2><p>{result.spu} · SPU 级场域口径，按日报日计算</p></div>
        </header>
        <OverlapTrendChart points={result.overlap_points ?? []} days={result.days} />
      </section>

      <section className="overview-section-heading">
        <div><h2>投放趋势</h2><p>{result.trend.start_date} - {result.trend.end_date} · 有效数据 {result.trend.available_days}/{result.days} 日</p></div>
        {loading ? <LoaderCircle size={17} className="spin" /> : <span>对比 {result.trend.previous_start_date} - {result.trend.previous_end_date}</span>}
      </section>
      <section className="overview-metric-grid">
        {result.trend.metrics.map((metric) => <TrendChart key={metric.key} metric={metric} days={result.days} />)}
      </section>

      <section className="overview-notes-section">
        <header>
          <div><h2>每日新增笔记</h2><p>{result.new_notes.start_date} - {result.new_notes.end_date} · 蒲公英</p></div>
          <strong>{result.new_notes.total}<span> 篇</span></strong>
        </header>
        <NewNotesChart result={result} />
      </section>

      <section className="agency-section">
        <header className="overview-section-heading">
          <div><h2>机构新增情况</h2><p>点击机构查看本周期笔记详情</p></div>
        </header>
        <div className="agency-grid">
          {result.new_notes.agencies.map((item) => <button key={item.agency} className={"agency-card " + (agency?.agency === item.agency ? "active" : "")} onClick={() => setSelectedAgency(item.agency)}>
            <span>{item.agency}</span><strong>{item.count}<small> 篇</small></strong>
            <div className="agency-tags">
              {item.audience_tags.length > 0 ? item.audience_tags.map((tag) => <em key={tag}>{tag}</em>) : <i>暂无匹配标签</i>}
            </div>
          </button>)}
        </div>

        <div className="agency-detail">
          <header><div><h3>{agency?.agency ?? "--"} · 笔记详情</h3><p>{agency?.count ?? 0} 篇，按发布时间倒序</p></div></header>
          <div className="agency-table-wrap"><table className="agency-table"><thead><tr><th>发布时间</th><th>笔记</th><th>达人 / 发布账号</th><th>人群标签</th><th>笔记类型</th><th>内容标签</th><th>链接</th></tr></thead><tbody>
            {agency?.notes.map((note) => {
              const href = safeURL(note.url);
              return <tr key={note.note_id}>
                <td>{note.published_date || "--"}</td>
                <td><strong title={note.title}>{note.title || note.note_id}</strong><small>{note.note_id}</small></td>
                <td>{note.author || "--"}</td>
                <td>{note.audience ? <span className="audience-badge">{note.audience}</span> : "--"}</td>
                <td>{note.note_type || "--"}</td>
                <td>{note.content_tag || "--"}</td>
                <td>{href ? <a href={href} target="_blank" rel="noreferrer" aria-label={"打开笔记 " + note.note_id}><ExternalLink size={15} /></a> : "--"}</td>
              </tr>;
            })}
            {agency && agency.notes.length === 0 ? <tr><td className="agency-empty" colSpan={7}>本周期暂无新增笔记</td></tr> : null}
          </tbody></table></div>
        </div>
      </section>
    </> : null}
  </>;
}

export default BusinessOverview;
