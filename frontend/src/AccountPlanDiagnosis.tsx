import { useEffect, useMemo, useRef, useState } from "react";
import { LineChart } from "echarts/charts";
import { GridComponent, LegendComponent, TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { AlertCircle, ExternalLink, LoaderCircle, Stethoscope, X } from "lucide-react";
import "./account-plan-diagnosis.css";

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer]);

type ServiceState = "checking" | "online" | "offline";
type PlanTab = "over" | "enlarge" | "stop";
type PeriodDays = 7 | 14 | 30;

type DiagnosisPoint = {
  report_date: string;
  spend: number | null;
  search_users: number | null;
  cost: number | null;
  original_cost: number | null;
  correction_coefficient: number | null;
  search_rate_pct: number | null;
  cpc: number | null;
  ctr_pct: number | null;
  note_count: number | null;
};

type DandelionSupplement = {
  title: string;
  author: string;
  note_type: string;
  content_tag: string;
  published_date: string;
  data_updated_date: string;
  dandelion_amount: number;
  impressions: number;
  reads: number;
  interactions: number;
  read_cost: number;
  interaction_cost: number;
};

type PlanDiagnosis = {
  note_id: string;
  note_url: string;
  campaign_name: string;
  spend: number;
  cost: number | null;
  original_cost: number | null;
  correction_coefficient: number | null;
  cost_metric: string;
  kpi: number;
  over_kpi: boolean;
  action: "inactive" | "enlarge" | "observe" | "stop";
  consecutive_over_kpi: number;
  dandelion?: DandelionSupplement;
};

type AccountDiagnosis = {
  account: string;
  placement: string;
  spend: number;
  search_users: number;
  cost: number | null;
  original_cost: number | null;
  correction_coefficient: number | null;
  search_rate_pct: number | null;
  cpc: number | null;
  ctr_pct: number | null;
  note_count: number;
  cost_metric: string;
  previous_cost: number | null;
  change_pct: number | null;
  kpi: number;
  status: "good" | "over" | "unattributed";
  over_plans: number;
  enlarge_plans: number;
  stop_plans: number;
  points: DiagnosisPoint[];
  plans: PlanDiagnosis[];
};

type AccountOverviewPoint = {
  report_date: string;
  total_spend: number | null;
  search_spend: number | null;
  search_cost: number | null;
  search_cpc: number | null;
  search_ctr_pct: number | null;
  search_rate_pct: number | null;
  feed_spend: number | null;
  feed_cost: number | null;
  feed_cpc: number | null;
  feed_ctr_pct: number | null;
  feed_search_rate_pct: number | null;
};

type AccountOverview = {
  account: string;
  current_total_spend: number;
  points: AccountOverviewPoint[];
};

type DiagnosisResult = {
  report_date: string;
  spu: string;
  account_kpi: number;
  plan_kpis: Record<string, number>;
  dandelion_synced_at: string;
  dandelion_matched: number;
  dandelion_missing: number;
  account_overviews: AccountOverview[];
  accounts: AccountDiagnosis[];
};

const EMPTY_RESULT: DiagnosisResult = {
  report_date: "", spu: "辅酶", account_kpi: 70, plan_kpis: { 搜索: 30, 信息流: 70 },
  dandelion_synced_at: "", dandelion_matched: 0, dandelion_missing: 0, account_overviews: [], accounts: []
};
const money = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

const integer = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
function accountKey(account: AccountDiagnosis): string {
  return `${account.account}\u0000${account.placement}`;
}

function shortDate(value: string): string {
  const parts = value.split("-");
  if (parts.length !== 3) return "-";
  return `${Number(parts[1])}月${Number(parts[2])}日`;
}

function normalizeNoteURL(value: string): string {
  const markdown = value.match(/^\[[^\]]*\]\((https?:\/\/[^)]+)\)$/);
  if (markdown) return markdown[1];
  return /^https?:\/\//.test(value) ? value : "";
}

function statusLabel(status: AccountDiagnosis["status"]): string {
  if (status === "good") return "达标";
  if (status === "over") return "超标";
  return "归因未形成";
}

function actionLabel(action: PlanDiagnosis["action"]): string {
  if (action === "enlarge") return "放大";
  if (action === "stop") return "停止";
  if (action === "observe") return "正常观察";
  return "今日未投放";
}

function Sparkline({ points }: { points: DiagnosisPoint[] }) {
  const validPoints = points.filter((point): point is DiagnosisPoint & { cost: number } => point.cost !== null);
  if (validPoints.length === 0) return <span className="diagnosis-no-trend">无数据</span>;
  const values = validPoints.map((point) => point.cost);
  const minimum = Math.min(...values);
  const maximum = Math.max(...values);
  const coordinates = validPoints.map((point, index) => ({
    x: 3 + index * (86 / Math.max(validPoints.length - 1, 1)),
    y: maximum === minimum ? 14 : 3 + ((maximum - point.cost) / (maximum - minimum)) * 22
  }));
  const path = coordinates.map((coordinate, index) => `${index === 0 ? "M" : "L"}${coordinate.x},${coordinate.y}`).join(" ");
  return <svg className="diagnosis-sparkline" viewBox="0 0 92 28" role="img" aria-label="7日成本趋势">
    <path d={path} />
    {coordinates.map((coordinate, index) => <circle key={validPoints[index].report_date} cx={coordinate.x} cy={coordinate.y} r="2"><title>{validPoints[index].report_date}：¥{money.format(validPoints[index].cost)}</title></circle>)}
  </svg>;
}

function planFilter(plans: PlanDiagnosis[], tab: PlanTab): PlanDiagnosis[] {
  if (tab === "over") return plans.filter((plan) => plan.over_kpi);
  return plans.filter((plan) => plan.action === tab);
}

type TrendUnit = "currency" | "percent";
type TrendPointKey = Exclude<keyof AccountOverviewPoint, "report_date">;
type TrendMetric = {
  key: string;
  label: string;
  unit: TrendUnit;
  color: string;
  axis: 0 | 1;
  values: (number | null)[];
};
type TrendChartConfig = {
  key: string;
  title: string;
  metrics: TrendMetric[];
};

const PERIOD_OPTIONS: PeriodDays[] = [7, 14, 30];

function compactNumber(value: number): string {
  if (Math.abs(value) >= 10000) return `${(value / 10000).toFixed(1)}万`;
  if (Math.abs(value) >= 1000) return `${(value / 1000).toFixed(1)}k`;
  return value.toFixed(value % 1 === 0 ? 0 : 1);
}

function trendValue(unit: TrendUnit, value: number | null): string {
  if (value === null) return "--";
  if (unit === "currency") return `¥${money.format(value)}`;
  return `${money.format(value)}%`;
}

function pointLabel(unit: TrendUnit, value: number): string {
  if (unit === "percent") return `${value.toFixed(2)}%`;
  return `¥${value.toFixed(value >= 1000 ? 0 : 2)}`;
}

function accountTrendCharts(points: AccountOverviewPoint[]): TrendChartConfig[] {
  const values = (key: TrendPointKey) => points.map((point) => point[key]);
  const metric = (
    key: string,
    label: string,
    unit: TrendUnit,
    color: string,
    axis: 0 | 1,
    pointKey: TrendPointKey
  ): TrendMetric => ({ key, label, unit, color, axis, values: values(pointKey) });
  return [
    {
      key: "total-spend",
      title: "总消耗",
      metrics: [metric("total-spend", "搜索 + 信息流", "currency", "#2f7d67", 0, "total_spend")]
    },
    {
      key: "search-delivery",
      title: "搜索消耗与修正后回搜成本",
      metrics: [
        metric("search-spend", "搜索消耗", "currency", "#36765f", 0, "search_spend"),
        metric("search-cost", "修正后回搜成本", "currency", "#c34f57", 1, "search_cost")
      ]
    },
    {
      key: "feed-delivery",
      title: "信息流消耗与修正后成本",
      metrics: [
        metric("feed-spend", "信息流消耗", "currency", "#3d6f9e", 0, "feed_spend"),
        metric("feed-cost", "修正后预计回流成本", "currency", "#a66a2c", 1, "feed_cost")
      ]
    },
    {
      key: "search-quality",
      title: "搜索 CPC 与 CTR",
      metrics: [
        metric("search-cpc", "搜索 CPC", "currency", "#a66a2c", 0, "search_cpc"),
        metric("search-ctr", "搜索 CTR", "percent", "#3d6f9e", 1, "search_ctr_pct")
      ]
    },
    {
      key: "feed-quality",
      title: "信息流 CPC 与 CTR",
      metrics: [
        metric("feed-cpc", "信息流 CPC", "currency", "#9a6a2f", 0, "feed_cpc"),
        metric("feed-ctr", "信息流 CTR", "percent", "#576fa8", 1, "feed_ctr_pct")
      ]
    },
    {
      key: "search-rate",
      title: "搜索 / 信息流回搜率",
      metrics: [
        metric("search-rate", "搜索回搜率", "percent", "#2f7d67", 0, "search_rate_pct"),
        metric("feed-rate", "信息流回搜率", "percent", "#c45a62", 0, "feed_search_rate_pct")
      ]
    }
  ];
}

function AccountTrendChart({ config, points, days }: { config: TrendChartConfig; points: Array<{ report_date: string }>; days: PeriodDays }) {
  const chartRef = useRef<HTMLDivElement>(null);
  const hasData = config.metrics.some((metric) => metric.values.some((value) => value !== null));
  const hasRightAxis = config.metrics.some((metric) => metric.axis === 1);
  const option = useMemo<EChartsCoreOption>(() => {
    const visiblePointIndexes = points
      .map((_, index) => index)
      .filter((index) => config.metrics.some((metric) => typeof metric.values[index] === "number"));
    const axisFor = (axis: 0 | 1) => config.metrics.find((metric) => metric.axis === axis) ?? config.metrics[0];
    const yAxis = ([0, ...(hasRightAxis ? [1] : [])] as Array<0 | 1>).map((axis) => {
      const metric = axisFor(axis);
      return {
        type: "value",
        scale: true,
        boundaryGap: ["18%", "18%"],
        position: axis === 0 ? "left" : "right",
        splitNumber: 4,
        axisLabel: {
          color: "#858c92",
          fontSize: 9,
          formatter: (value: number) => metric.unit === "percent" ? `${compactNumber(value)}%` : compactNumber(value)
        },
        splitLine: { show: axis === 0, lineStyle: { color: "#edf0f1", type: "dashed" } }
      };
    });
    return {
      animationDuration: 320,
      color: config.metrics.map((metric) => metric.color),
      grid: { left: 54, right: hasRightAxis ? 54 : 22, top: 52, bottom: 36 },
      legend: {
        top: 9,
        right: 12,
        itemWidth: 16,
        itemHeight: 7,
        textStyle: { color: "#697177", fontSize: 9 }
      },
      tooltip: {
        trigger: "axis",
        backgroundColor: "rgba(31, 35, 38, 0.94)",
        borderWidth: 0,
        padding: [8, 10],
        textStyle: { color: "#fff", fontSize: 10 }
      },
      xAxis: {
        type: "category",
        boundaryGap: false,
        data: visiblePointIndexes.map((index) => points[index].report_date.slice(5)),
        axisLine: { lineStyle: { color: "#dfe3e5" } },
        axisTick: { show: false },
        axisLabel: { color: "#858c92", fontSize: 9, interval: days === 7 ? 0 : "auto" }
      },
      yAxis,
      series: config.metrics.map((metric, index) => ({
        name: metric.label,
        type: "line",
        yAxisIndex: metric.axis,
        data: visiblePointIndexes.map((pointIndex) => metric.values[pointIndex]),
        connectNulls: true,
        smooth: 0.16,
        showSymbol: days === 7,
        symbol: "circle",
        symbolSize: 7,
        lineStyle: { width: 2.5, color: metric.color },
        itemStyle: { color: metric.color, borderColor: "#fff", borderWidth: 2 },
        tooltip: { valueFormatter: (value: number) => trendValue(metric.unit, value) },
        label: {
          show: days === 7,
          position: config.key === "search-rate" || index % 2 === 0 ? "top" : "bottom",
          distance: 6,
          color: metric.color,
          fontSize: 8,
          formatter: (params: { value: number | null }) => params.value === null ? "" : pointLabel(metric.unit, params.value)
        },
        emphasis: { focus: "series", scale: 1.15 }
      }))
    };
  }, [config, days, hasRightAxis, points]);

  useEffect(() => {
    if (!chartRef.current || !hasData) return;
    const chart = echarts.init(chartRef.current, undefined, { renderer: "canvas" });
    chart.setOption(option);
    const observer = new ResizeObserver(() => chart.resize());
    observer.observe(chartRef.current);
    return () => {
      observer.disconnect();
      chart.dispose();
    };
  }, [hasData, option]);

  return <article className="diagnosis-trend-card">
    <header>
      <h3>{config.title}</h3>
      <dl>
        {config.metrics.map((metric) => <div key={metric.key}><dt><i style={{ background: metric.color }} />{metric.label}</dt><dd>{trendValue(metric.unit, metric.values.at(-1) ?? null)}</dd></div>)}
      </dl>
    </header>
    {hasData
      ? <div ref={chartRef} className="diagnosis-trend-canvas" role="img" aria-label={`${config.title}趋势图`} />
      : <div className="diagnosis-trend-empty">当前周期暂无数据</div>}
  </article>;
}

function CorrectedCostValue({ originalCost, coefficient, correctedCost, metric }: {
  originalCost: number | null;
  coefficient: number | null;
  correctedCost: number | null;
  metric: string;
}) {
  if (correctedCost === null) return <span title={originalCost === null ? metric : `${metric}缺少当日修正系数`}>-</span>;
  const detail = originalCost !== null && coefficient !== null
    ? `原 ¥${money.format(originalCost)} × ${coefficient.toFixed(2)}`
    : "";
  return <span className="diagnosis-cost-comparison" title={`${metric}（修正后）`}>
    <strong>¥{money.format(correctedCost)}</strong>
    {detail ? <small>{detail}</small> : null}
  </span>;
}

function AccountCostValue({ originalCost, coefficient, correctedCost, metric }: {
  originalCost: number | null;
  coefficient: number | null;
  correctedCost: number | null;
  metric: string;
}) {
  if (originalCost === null) return <span title={metric}>-</span>;
  const correction = correctedCost !== null
    ? `修正后 ¥${money.format(correctedCost)}`
    : "暂无修正值";
  const title = correctedCost !== null && coefficient !== null
    ? `${metric}原始值；修正后 ¥${money.format(correctedCost)}（× ${coefficient.toFixed(2)}）`
    : `${metric}原始值`;
  return <span className="diagnosis-cost-comparison" title={title}>
    <strong>¥{money.format(originalCost)}</strong>
    <small>{correction}</small>
  </span>;
}

function PlanDrawer({ account, onClose }: { account: AccountDiagnosis; onClose: () => void }) {
  const [tab, setTab] = useState<PlanTab>("over");
  const counts = {
    over: planFilter(account.plans, "over").length,
    enlarge: planFilter(account.plans, "enlarge").length,
    stop: planFilter(account.plans, "stop").length
  };
  const plans = planFilter(account.plans, tab);
  const labels: Record<PlanTab, string> = { over: "成本超标", enlarge: "建议放大", stop: "建议停止" };

  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => { if (event.key === "Escape") onClose(); };
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [onClose]);

  return <>
    <button className="diagnosis-drawer-backdrop" type="button" aria-label="关闭计划明细" onClick={onClose} />
    <aside className="diagnosis-drawer" aria-label={`${account.account}计划诊断`}>
      <header className="diagnosis-drawer-head">
        <div><h2>{account.account}</h2><p>{account.placement} · 修正后{account.cost_metric} · 计划 KPI {money.format(account.placement === "信息流" ? 70 : 30)}</p></div>
        <button className="icon-button" type="button" title="关闭" aria-label="关闭" onClick={onClose}><X size={19} /></button>
      </header>
      <div className="diagnosis-drawer-tabs" aria-label="计划诊断分类">
        {(Object.keys(labels) as PlanTab[]).map((value) => <button type="button" className={tab === value ? "active" : ""} key={value} onClick={() => setTab(value)}>{labels[value]} <span>{counts[value]}</span></button>)}
      </div>
      <div className="diagnosis-drawer-body">
        <div className="diagnosis-drawer-summary">以下 KPI 均按修正后诊断成本判断；修正后诊断成本 = 原始成本 × 笔记/SPU 综合重合系数。连续 3 个有效报表日超标时建议停止。</div>
        {plans.length === 0 ? <div className="diagnosis-drawer-empty">该分类暂无计划</div> : <div className="diagnosis-plan-table-wrap"><table className="diagnosis-plan-table">
          <thead><tr><th>计划名</th><th>蒲公英数据</th><th>消耗</th><th>修正后诊断成本</th><th>KPI</th><th>超标</th><th>动作</th><th>连续天数</th></tr></thead>
          <tbody>{plans.map((plan) => {
            const noteURL = normalizeNoteURL(plan.note_url);
            const excess = plan.cost === null ? null : (plan.cost / plan.kpi - 1) * 100;
            return <tr key={`${plan.note_id}-${plan.campaign_name}`}>
              <td><div className="diagnosis-plan-name">{noteURL ? <a href={noteURL} target="_blank" rel="noreferrer" title={plan.campaign_name}>{plan.campaign_name}<ExternalLink size={12} /></a> : <strong title={plan.campaign_name}>{plan.campaign_name}</strong>}<span>{plan.note_id}</span></div></td>
              <td>{plan.dandelion ? <div className="diagnosis-dandelion-note" title={`发布 ${plan.dandelion.published_date || "-"} · 数据更新 ${plan.dandelion.data_updated_date || "-"} · 阅读单价 ¥${money.format(plan.dandelion.read_cost)} · 互动单价 ¥${money.format(plan.dandelion.interaction_cost)}`}>
                <strong>{plan.dandelion.title || "未命名笔记"}</strong>
                <span>{[plan.dandelion.author, plan.dandelion.note_type, plan.dandelion.content_tag].filter(Boolean).join(" · ") || "-"}</span>
                <small>曝光 {integer.format(plan.dandelion.impressions)} · 阅读 {integer.format(plan.dandelion.reads)} · 互动 {integer.format(plan.dandelion.interactions)} · 合作 ¥{money.format(plan.dandelion.dandelion_amount)}</small>
              </div> : <span className="diagnosis-dandelion-missing">未匹配</span>}</td>
              <td className="num">¥{money.format(plan.spend)}</td>
              <td className="num"><CorrectedCostValue originalCost={plan.original_cost} coefficient={plan.correction_coefficient} correctedCost={plan.cost} metric={plan.cost_metric} /></td>
              <td className="num">¥{money.format(plan.kpi)}</td>
              <td className={`num ${excess !== null && excess >= 0 ? "diagnosis-over-value" : ""}`}>{excess === null ? "-" : `${excess >= 0 ? "+" : ""}${excess.toFixed(0)}%`}</td>
              <td><span className={`diagnosis-action ${plan.action}`}>{actionLabel(plan.action)}</span></td>
              <td className="num">{plan.consecutive_over_kpi}</td>
            </tr>;
          })}</tbody>
        </table></div>}
      </div>
    </aside>
  </>;
}

function AccountPlanDiagnosis({ serviceState }: { serviceState: ServiceState }) {
  const [result, setResult] = useState<DiagnosisResult>(EMPTY_RESULT);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selectedKey, setSelectedKey] = useState("");
  const [overviewAccount, setOverviewAccount] = useState("");
  const [days, setDays] = useState<PeriodDays>(7);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/account-plan-diagnosis?spu=${encodeURIComponent("辅酶")}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: DiagnosisResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "子账户诊断读取失败");
        setResult(payload.data);
        setOverviewAccount((current) => payload.data?.account_overviews.some((item) => item.account === current)
          ? current
          : payload.data?.account_overviews[0]?.account ?? "");
        setError("");
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "子账户诊断读取失败");
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, []);

  const selected = useMemo(() => result.accounts.find((account) => accountKey(account) === selectedKey) ?? null, [result.accounts, selectedKey]);
  const overview = useMemo(() => result.account_overviews.find((account) => account.account === overviewAccount) ?? result.account_overviews[0] ?? null, [overviewAccount, result.account_overviews]);
  const overviewPoints = useMemo(() => overview?.points.slice(-days) ?? [], [days, overview]);
  const overviewCharts = useMemo(() => accountTrendCharts(overviewPoints), [overviewPoints]);
  const overviewDateRange = overviewPoints.length > 0 ? `${overviewPoints[0].report_date} - ${overviewPoints.at(-1)?.report_date}` : "";
  const totalPlans = result.accounts.reduce((total, account) => total + account.plans.length, 0);
  const dandelionTotal = result.dandelion_matched + result.dandelion_missing;
  const dandelionDate = result.dandelion_synced_at ? shortDate(result.dandelion_synced_at.slice(0, 10)) : "-";

  return <>
    <section className="page-heading diagnosis-page-heading">
      <div><h1>子账户与计划诊断</h1><p>辅酶Q10 · 子账户 KPI 70 · 计划 KPI：搜索 30、信息流 70 · KPI 均按修正后成本判断</p></div>
      <div className="heading-status"><span className={`status-dot ${serviceState}`} />{result.report_date ? `数据截至 ${shortDate(result.report_date)}` : "等待日报数据"}</div>
    </section>
    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}
    <section className="diagnosis-overview-heading">
      <div><h2>子账户数据总览</h2><p>{overview ? `${overview.account} · ${overviewDateRange}` : "等待子账户数据"}</p></div>
      <div className="diagnosis-overview-controls">
        <label className="diagnosis-account-select"><span>子账户</span><select aria-label="选择子账户" value={overview?.account ?? ""} onChange={(event) => setOverviewAccount(event.target.value)}>
          {result.account_overviews.map((account) => <option key={account.account} value={account.account}>{account.account}</option>)}
        </select></label>
        <div className="segmented-control diagnosis-period" aria-label="总览时间范围">
          {PERIOD_OPTIONS.map((option) => <button type="button" key={option} className={days === option ? "active" : ""} onClick={() => setDays(option)}>{option}日</button>)}
        </div>
      </div>
    </section>
    {loading ? <div className="analysis-loading"><LoaderCircle size={19} className="spin" />正在读取子账户趋势</div>
      : overview ? <section className="diagnosis-trend-grid">
        {overviewCharts.map((chart) => <AccountTrendChart key={chart.key} config={chart} points={overviewPoints} days={days} />)}
      </section>
        : <div className="analysis-loading">当前 SPU 暂无子账户趋势数据</div>}
    <section className="diagnosis-table-section">
      <header><div><Stethoscope size={18} /><span><strong>子账户诊断</strong><small>{result.accounts.length} 个子账户场域 · {totalPlans} 条计划明细</small></span></div><p>蒲公英 {result.dandelion_matched}/{dandelionTotal} · 更新 {dandelionDate}</p></header>
      {loading ? <div className="diagnosis-loading"><LoaderCircle size={19} className="spin" />正在生成诊断</div>
        : result.accounts.length === 0 ? <div className="diagnosis-loading">当前 SPU 暂无可诊断数据</div>
          : <div className="diagnosis-table-wrap"><table className="diagnosis-account-table">
            <thead><tr><th>子账户</th><th>场域</th><th>消耗</th><th>原始日报成本</th><th>较昨日</th><th>KPI</th><th>状态</th><th>超标计划</th><th>放大</th><th>停止</th><th>7日修正后成本</th></tr></thead>
            <tbody>{result.accounts.map((account) => <tr key={accountKey(account)}>
              <td><button type="button" className="diagnosis-account-button" title="查看计划诊断" onClick={() => setSelectedKey(accountKey(account))}>{account.account}</button></td>
              <td><span className={`placement-swatch placement-${account.placement}`}>{account.placement}</span></td>
              <td className="num">¥{money.format(account.spend)}</td>
              <td className="num"><AccountCostValue originalCost={account.original_cost} coefficient={account.correction_coefficient} correctedCost={account.cost} metric={account.cost_metric} /></td>
              <td className={`num ${account.change_pct !== null && account.change_pct > 0 ? "diagnosis-over-value" : account.change_pct !== null ? "diagnosis-good-value" : ""}`}>{account.change_pct === null ? "-" : `${account.change_pct >= 0 ? "+" : ""}${(account.change_pct * 100).toFixed(1)}%`}</td>
              <td className="num">¥{money.format(account.kpi)}</td>
              <td><span className={`diagnosis-status ${account.status}`}>{statusLabel(account.status)}</span></td>
              <td className="num"><span className="diagnosis-count">{account.over_plans}</span></td>
              <td className="num"><span className="diagnosis-count">{account.enlarge_plans}</span></td>
              <td className="num"><span className="diagnosis-count">{account.stop_plans}</span></td>
              <td><Sparkline points={account.points} /></td>
            </tr>)}</tbody>
          </table></div>}
    </section>
    {selected ? <PlanDrawer account={selected} onClose={() => setSelectedKey("")} /> : null}
  </>;
}

export default AccountPlanDiagnosis;
