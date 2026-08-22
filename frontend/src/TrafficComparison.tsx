import { useEffect, useMemo, useRef, useState } from "react";
import { LineChart } from "echarts/charts";
import { GridComponent, LegendComponent, TooltipComponent } from "echarts/components";
import * as echarts from "echarts/core";
import type { EChartsCoreOption } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import {
  AlertCircle, ArrowDownWideNarrow, ChevronLeft, ChevronRight, GitCompareArrows,
  LoaderCircle, Search, SlidersHorizontal, TrendingUp
} from "lucide-react";
import { targetProvinceNames } from "./target-regions";
import "./traffic-comparison.css";

echarts.use([LineChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer]);

type WindowOption = "3d" | "7d" | "all";
type ServiceState = "checking" | "online" | "offline";

type ComparisonPoint = {
  report_date: string;
  spend: number;
  search_users: number;
  search_cost: number;
  has_search_cost: boolean;
};

type ComparisonCampaign = {
  campaign_name: string;
  first_report_date: string;
  last_report_date: string;
  active_days: number;
  latest_spend: number;
  latest_search_users: number;
  latest_search_cost: number;
  has_latest_search_cost: boolean;
  total_spend: number;
  total_search_users: number;
  points: ComparisonPoint[];
};

type ComparisonItem = {
  note_id: string;
  placement: string;
  campaign_count: number;
  comparable_campaign_count: number;
  latest_search_cost_min: number;
  latest_search_cost_max: number;
  search_cost_gap: number;
  latest_spend: number;
  latest_search_users: number;
  campaigns: ComparisonCampaign[];
};

type ComparisonResult = {
  window: WindowOption;
  report_dates: string[];
  latest_date: string;
  total: number;
  page: number;
  page_size: number;
  items: ComparisonItem[];
};

type DeliverySearchKeyword = { keyword: string; bid: number; feed_bid: number; phrase_match_type: number; };
type DeliveryCrowd = { name: string; value: string; };
type DeliveryTarget = {
  gender: string; age: string; city: string; area_code: string; device: string; device_price: string;
  intelligent_expansion: number; generalization_switch: number; search_city_intent: string;
  interest_keywords: string[]; behavior_keywords: string[]; excluded_crowds: string[];
  crowd_packages: DeliveryCrowd[]; content_interests: string[]; shopping_interests: string[];
  premium_crowds: Array<{ name: string; ratio: string }>; dandelion_crowds: string[];
  brand_interest_group: boolean; brand_recognition_group: boolean; category_interest_group: boolean; goods_interest_group: boolean;
};
type DeliveryUnit = {
  unit_name: string; event_bid: number; target_type: number;
  delivery?: { search_keywords: DeliverySearchKeyword[]; target: DeliveryTarget };
};
type DeliveryMatch = {
  advertiser_name: string; campaign_day_budget: number; bidding_strategy: number;
  marketing_target: number; optimize_objective: number; units: DeliveryUnit[];
};
type DeliveryCampaign = { campaign_name: string; matches: DeliveryMatch[]; };
type DeliveryResult = { report_date: string; note_id: string; placement: string; campaigns: DeliveryCampaign[]; };
type DeliveryDiffRow = { key: string; label: string; values: string[][]; different: boolean; };

const EMPTY_RESULT: ComparisonResult = {
  window: "7d", report_dates: [], latest_date: "", total: 0, page: 1, page_size: 25, items: []
};
const WINDOW_OPTIONS: Array<{ value: WindowOption; label: string }> = [
  { value: "3d", label: "3D" },
  { value: "7d", label: "7D" },
  { value: "all", label: "全部" }
];
const COLORS = ["#2f7d67", "#c94e55", "#b5852d", "#377da2", "#7b67a8", "#4f8f91", "#a45f78", "#6b7f43"];
const money = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const integer = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
const biddingStrategies: Record<number, string> = { 2: "手动出价", 3: "最大转化", 7: "稳定成本" };
const targetTypes: Record<number, string> = { 0: "默认定向", 1: "通投", 2: "智能定向", 3: "高级定向" };
const marketingTargets: Record<number, string> = {
  3: "商品销量", 4: "产品种草", 8: "直播推广", 9: "客资收集", 10: "抢占关键词",
  13: "种草直达", 14: "直播预热", 15: "店铺拉新", 16: "应用唤起", 20: "应用下载", 21: "小程序推广"
};
const optimizeObjectives: Record<number, Record<number, string>> = {
  4: { 0: "点击量", 1: "互动量", 18: "站外转化量", 30: "种草人群规模", 31: "深度种草人群规模", 51: "点击份额（SOC）" },
  9: { 3: "表单提交量", 5: "私信进线量", 13: "私信开口量", 50: "私信留资量", 78: "线索流资量" },
  13: { 19: "组件点击（点击归因）", 21: "店铺成交（点击归因）", 44: "店铺访问（阅读归因）", 45: "店铺成交（阅读归因）" },
  16: { 0: "点击量", 35: "APP打开", 36: "APP进店", 37: "APP互动", 38: "APP支付", 39: "APP支付ROI", 43: "笔记唤端组件点击" },
  20: { 60: "APP下载按钮点击", 61: "APP激活", 62: "APP注册", 63: "APP关键行为", 64: "APP付费", 69: "APP预约下载按钮点击", 72: "APP预约下载" },
  21: { 65: "小程序打开", 67: "小程序支付订单数", 68: "小程序ROI", 73: "微信小游戏打开", 74: "微信小游戏激活", 75: "微信小游戏订单支付数" }
};

function itemKey(item: ComparisonItem): string {
  return `${item.note_id}\u0000${item.placement}`;
}

function unique(values: Array<string | undefined>): string[] {
  return [...new Set(values.map((value) => value?.trim()).filter((value): value is string => Boolean(value)))].sort((a, b) => a.localeCompare(b, "zh-CN"));
}

function targetParts(value: string, fallback: string): string[] {
  if (!value || value === "all" || value === "-1") return [fallback];
  return unique(value.split("#"));
}

function officialLabel(values: Record<number, string>, value: number): string {
  return values[value] ?? `官方未定义（${value}）`;
}

function objectiveLabel(marketingTarget: number, objective: number): string {
  return `${officialLabel(marketingTargets, marketingTarget)} · ${optimizeObjectives[marketingTarget]?.[objective] ?? `官方未定义（${objective}）`}`;
}

function buildDeliveryRows(item: ComparisonItem, result: DeliveryResult | null): DeliveryDiffRow[] {
  const detailByCampaign = new Map((result?.campaigns ?? []).map((campaign) => [campaign.campaign_name, campaign]));
  const profiles = item.campaigns.map((campaign) => detailByCampaign.get(campaign.campaign_name));
  const fromMatches = (profile: DeliveryCampaign | undefined, collect: (match: DeliveryMatch) => string[]) =>
    !profile || profile.matches.length === 0 ? ["未匹配聚光配置"] : unique(profile.matches.flatMap(collect)).length > 0 ? unique(profile.matches.flatMap(collect)) : ["未配置"];
  const fromUnits = (profile: DeliveryCampaign | undefined, collect: (unit: DeliveryUnit) => string[]) =>
    fromMatches(profile, (match) => match.units.flatMap(collect));
  const definitions: Array<[string, string, (profile: DeliveryCampaign | undefined) => string[]]> = [
    ["strategy", "出价策略", (profile) => fromMatches(profile, (match) => [officialLabel(biddingStrategies, match.bidding_strategy)])],
    ["objective", "营销与优化目标", (profile) => fromMatches(profile, (match) => [objectiveLabel(match.marketing_target, match.optimize_objective)])],
    ["unit", "投放单元", (profile) => fromUnits(profile, (unit) => [unit.unit_name || "未命名单元"])],
    ["event-bid", "单元出价", (profile) => fromUnits(profile, (unit) => [unit.event_bid > 0 ? `${unit.unit_name || "未命名单元"} · ¥${money.format(unit.event_bid / 100)}` : `${unit.unit_name || "未命名单元"} · 未配置`])],
    ["target-type", "定向模式", (profile) => fromUnits(profile, (unit) => [officialLabel(targetTypes, unit.target_type)])],
    ["gender", "性别", (profile) => fromUnits(profile, (unit) => [unit.delivery?.target.gender === "0" ? "男" : unit.delivery?.target.gender === "1" ? "女" : "不限"])],
    ["age", "年龄", (profile) => fromUnits(profile, (unit) => targetParts(unit.delivery?.target.age ?? "", "不限"))],
    ["region", "省级地域", (profile) => fromUnits(profile, (unit) => targetProvinceNames(unit.delivery?.target.city ?? ""))],
    ["device", "设备", (profile) => fromUnits(profile, (unit) => [unit.delivery?.target.device === "ios" ? "苹果设备" : unit.delivery?.target.device === "android" ? "安卓设备" : "不限"])],
    ["device-price", "手机价格", (profile) => fromUnits(profile, (unit) => targetParts(unit.delivery?.target.device_price ?? "", "不限"))],
    ["expansion", "定向扩量", (profile) => fromUnits(profile, (unit) => [`智能扩量${unit.delivery?.target.intelligent_expansion === 1 ? "开启" : "关闭"}`, `定向拓宽${unit.delivery?.target.generalization_switch === 1 ? "开启" : "关闭"}`])],
    ["crowd", "人群包", (profile) => fromUnits(profile, (unit) => {
      const target = unit.delivery?.target;
      if (!target) return [];
      return [
        ...target.crowd_packages.map((crowd) => crowd.name || crowd.value),
        ...target.premium_crowds.map((crowd) => crowd.name + (crowd.ratio ? ` · ${crowd.ratio}倍` : "")),
        ...target.dandelion_crowds,
        target.brand_interest_group ? "本品牌兴趣人群" : "", target.brand_recognition_group ? "本品牌认知人群" : "",
        target.category_interest_group ? "行业兴趣人群" : "", target.goods_interest_group ? "商品兴趣人群" : ""
      ];
    })],
    ["industry", "行业兴趣", (profile) => fromUnits(profile, (unit) => [...(unit.delivery?.target.content_interests ?? []), ...(unit.delivery?.target.shopping_interests ?? [])])],
    ["interest", "兴趣关键词", (profile) => fromUnits(profile, (unit) => unit.delivery?.target.interest_keywords ?? [])],
    ["behavior", "行为关键词", (profile) => fromUnits(profile, (unit) => unit.delivery?.target.behavior_keywords ?? [])],
    ["excluded", "排除人群", (profile) => fromUnits(profile, (unit) => unit.delivery?.target.excluded_crowds ?? [])],
    ["search-keywords", "搜索关键词", (profile) => fromUnits(profile, (unit) => (unit.delivery?.search_keywords ?? []).map((keyword) => keyword.keyword))],
    ["keyword-bids", "关键词出价", (profile) => fromUnits(profile, (unit) => (unit.delivery?.search_keywords ?? []).map((keyword) => {
      const matchType = ({ 0: "精确", 1: "短语", 2: "智能" } as Record<number, string>)[keyword.phrase_match_type] ?? `类型${keyword.phrase_match_type}`;
      return `${keyword.keyword} · ${matchType} · ¥${money.format(keyword.bid / 100)}${keyword.feed_bid > 0 ? ` · 追投¥${money.format(keyword.feed_bid / 100)}` : ""}`;
    }))]
  ];
  return definitions.map(([key, label, collect]) => {
    const values = profiles.map(collect).map((items) => unique(items).length > 0 ? unique(items) : ["未配置"]);
    return { key, label, values, different: new Set(values.map((items) => JSON.stringify(items))).size > 1 };
  });
}

function DeliveryValues({ values }: { values: string[] }) {
  const [expanded, setExpanded] = useState(false);
  const visible = expanded ? values : values.slice(0, 8);
  return <div className="delivery-diff-values">
    <div>{visible.map((value) => <span className={value === "未配置" || value === "未匹配聚光配置" ? "empty" : ""} key={value} title={value}>{value}</span>)}</div>
    {values.length > 8 ? <button type="button" onClick={() => setExpanded((current) => !current)}>{expanded ? "收起" : `展开全部 ${values.length} 项`}</button> : null}
  </div>;
}

function differenceOnlyValues(row: DeliveryDiffRow): string[][] {
  if (!row.different || row.values.length < 2) return row.values;
  const common = new Set(row.values[0].filter((value) => row.values.slice(1).every((items) => items.includes(value))));
  return row.values.map((items) => {
    const different = items.filter((value) => !common.has(value));
    return different.length > 0 ? different : ["无独有配置"];
  });
}

function DeliveryDifference({ item }: { item: ComparisonItem }) {
  const [result, setResult] = useState<DeliveryResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showSame, setShowSame] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ note_id: item.note_id, placement: item.placement });
    setLoading(true);
    setError("");
    setResult(null);
    setShowSame(false);
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/traffic-comparison-delivery?${params}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: DeliveryResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "投流配置读取失败");
        setResult(payload.data);
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "投流配置读取失败");
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [item.note_id, item.placement]);

  const rows = useMemo(() => buildDeliveryRows(item, result), [item, result]);
  const differentRows = rows.filter((row) => row.different);
  const visibleRows = showSame ? rows : differentRows;
  const bestCostIndex = item.comparable_campaign_count >= 2
    ? item.campaigns.reduce((best, campaign, index, campaigns) => !campaign.has_latest_search_cost
      ? best
      : best < 0 || campaign.latest_search_cost < campaigns[best].latest_search_cost ? index : best, -1)
    : -1;
  const matchedCount = item.campaigns.filter((campaign) => result?.campaigns.some((detail) => detail.campaign_name === campaign.campaign_name && detail.matches.length > 0)).length;

  return <section className="delivery-difference-section">
    <header><div><SlidersHorizontal size={15} /><div><h3>投流配置差异</h3><p>{loading ? "正在关联聚光计划与单元" : `${differentRows.length} 项配置不同 · ${matchedCount}/${item.campaigns.length} 个计划已关联聚光`}</p></div></div>
      <label className="show-same-control"><input type="checkbox" checked={showSame} onChange={(event) => setShowSame(event.target.checked)} /><span>显示相同项</span></label>
    </header>
    {error ? <div className="delivery-diff-state"><AlertCircle size={15} />{error}</div>
      : loading ? <div className="delivery-diff-state"><LoaderCircle size={17} className="spin" />正在读取投流配置</div>
        : visibleRows.length === 0 ? <div className="delivery-diff-state">当前计划的已关联投流配置没有差异</div>
          : <div className="delivery-diff-table-wrap"><table className="delivery-diff-table" style={{ minWidth: Math.max(860, 170 + item.campaigns.length * 280) }}>
            <thead><tr><th>投流维度</th>{item.campaigns.map((campaign, index) => <th className={index === bestCostIndex ? "best-cost-column" : ""} key={campaign.campaign_name}><div className="delivery-plan-heading"><span className="campaign-color" style={{ backgroundColor: COLORS[index % COLORS.length] }} /><div><strong title={campaign.campaign_name}>{campaign.campaign_name}</strong><span>{campaign.has_latest_search_cost ? `当天成本 ¥${money.format(campaign.latest_search_cost)}` : "当天暂无有效成本"} · 消耗 ¥{money.format(campaign.latest_spend)}</span></div></div></th>)}</tr></thead>
            <tbody>{visibleRows.map((row) => <tr className={row.different ? "different" : "same"} key={row.key}><th><span>{row.label}</span>{row.different ? <strong>不同</strong> : <small>相同</small>}</th>{(showSame ? row.values : differenceOnlyValues(row)).map((values, index) => <td className={index === bestCostIndex ? "best-cost-column" : ""} key={`${row.key}-${item.campaigns[index].campaign_name}`}><DeliveryValues values={values} /></td>)}</tr>)}</tbody>
          </table></div>}
  </section>;
}

function CostComparisonChart({ item, dates }: { item: ComparisonItem; dates: string[] }) {
  const chartRef = useRef<HTMLDivElement>(null);
  const option = useMemo<EChartsCoreOption>(() => ({
    animationDuration: 360,
    color: COLORS,
    grid: { left: 52, right: 22, top: 20, bottom: item.campaigns.length > 1 ? 68 : 38 },
    legend: {
      show: item.campaigns.length > 1,
      type: "scroll",
      bottom: 12,
      left: 18,
      right: 18,
      itemWidth: 15,
      itemHeight: 7,
      textStyle: { color: "#66706c", fontSize: 9 },
      formatter: (name: string) => name.length > 24 ? name.slice(0, 24) + "…" : name
    },
    tooltip: {
      trigger: "axis",
      backgroundColor: "rgba(31, 36, 38, .95)",
      borderWidth: 0,
      padding: [8, 10],
      textStyle: { color: "#fff", fontSize: 10 },
      valueFormatter: (value: unknown) => value == null ? "-" : "¥" + money.format(Number(value))
    },
    xAxis: {
      type: "category",
      boundaryGap: false,
      data: dates.map((date) => date.slice(5)),
      axisLine: { lineStyle: { color: "#dfe4e2" } },
      axisTick: { show: false },
      axisLabel: { color: "#858d89", fontSize: 9, interval: dates.length > 12 ? "auto" : 0 }
    },
    yAxis: {
      type: "value",
      min: 0,
      axisLabel: { color: "#858d89", fontSize: 9, formatter: (value: number) => "¥" + value },
      splitLine: { lineStyle: { color: "#edf0ef", type: "dashed" } }
    },
    series: item.campaigns.map((campaign, index) => ({
      name: campaign.campaign_name,
      type: "line",
      data: campaign.points.map((point) => point.has_search_cost ? point.search_cost : null),
      smooth: 0.18,
      connectNulls: false,
      showSymbol: dates.length <= 7,
      symbol: "circle",
      symbolSize: 6,
      lineStyle: { width: index < 2 ? 3 : 2 },
      itemStyle: { borderColor: "#fff", borderWidth: 2 },
      emphasis: { focus: "series" }
    }))
  }), [dates, item]);

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

  return <div className="comparison-cost-chart" ref={chartRef} role="img" aria-label="不同计划回搜成本对比折线图" />;
}

function ComparisonDetail({ item, dates, latestDate }: { item: ComparisonItem; dates: string[]; latestDate: string }) {
  const hasComparison = item.comparable_campaign_count >= 2;
  const gapLabel = item.campaign_count === 1 ? "无计划差异" : hasComparison ? "¥" + money.format(item.search_cost_gap) : "有效成本不足";
  return <section className="comparison-detail">
    <header className="comparison-detail-heading">
      <div>
        <span className={`placement-swatch placement-${item.placement}`}>{item.placement}</span>
        <div><h2>{item.note_id}</h2><p>{latestDate} · {item.campaign_count} 个计划</p></div>
      </div>
      <div className={`comparison-gap-summary ${hasComparison ? "has-gap" : ""}`}>
        <span>{item.campaign_count > 1 ? "当天成本差异" : "单计划"}</span>
        <strong>{gapLabel}</strong>
      </div>
    </header>
    <div className="comparison-kpis">
      <div><span>最低回搜成本</span><strong>{item.comparable_campaign_count > 0 ? "¥" + money.format(item.latest_search_cost_min) : "-"}</strong></div>
      <div><span>最高回搜成本</span><strong>{item.comparable_campaign_count > 0 ? "¥" + money.format(item.latest_search_cost_max) : "-"}</strong></div>
      <div><span>当天总消耗</span><strong>¥{money.format(item.latest_spend)}</strong></div>
      <div><span>当天回搜人数</span><strong>{integer.format(item.latest_search_users)}</strong></div>
    </div>
    <DeliveryDifference item={item} />
    <section className="comparison-chart-section">
      <header><div><TrendingUp size={15} /><h3>回搜成本走势</h3></div><span>按日报日对比，不补周末日期</span></header>
      <CostComparisonChart item={item} dates={dates} />
    </section>
    <section className="comparison-campaign-section">
      <header><h3>计划指标对比</h3><span>按当天回搜成本从高到低</span></header>
      <div className="comparison-campaign-table-wrap"><table className="comparison-campaign-table">
        <thead><tr><th>计划</th><th>当天回搜成本</th><th>较最低成本</th><th>当天消耗</th><th>当天回搜人数</th><th>区间消耗</th><th>区间回搜人数</th><th>有效报表日</th></tr></thead>
        <tbody>{item.campaigns.map((campaign, index) => {
          const difference = campaign.has_latest_search_cost ? campaign.latest_search_cost - item.latest_search_cost_min : 0;
          return <tr key={campaign.campaign_name}>
            <td><span className="campaign-color" style={{ backgroundColor: COLORS[index % COLORS.length] }} /><strong title={campaign.campaign_name}>{campaign.campaign_name}</strong></td>
            <td>{campaign.has_latest_search_cost ? "¥" + money.format(campaign.latest_search_cost) : "暂无有效成本"}</td>
            <td className={difference > 0 ? "cost-above-min" : ""}>{!campaign.has_latest_search_cost ? "-" : difference > 0 ? "+¥" + money.format(difference) : "-"}</td>
            <td>¥{money.format(campaign.latest_spend)}</td>
            <td>{integer.format(campaign.latest_search_users)}</td>
            <td>¥{money.format(campaign.total_spend)}</td>
            <td>{integer.format(campaign.total_search_users)}</td>
            <td>{campaign.active_days}/{dates.length}</td>
          </tr>;
        })}</tbody>
      </table></div>
    </section>
  </section>;
}

export default function TrafficComparison({ serviceState }: { serviceState: ServiceState }) {
  const [windowOption, setWindowOption] = useState<WindowOption>("7d");
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<ComparisonResult>(EMPTY_RESULT);
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
    const params = new URLSearchParams({ window: windowOption, page: String(page), page_size: "25" });
    if (searchQuery) params.set("q", searchQuery);
    setLoading(true);
    setError("");
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/traffic-comparisons?${params}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: ComparisonResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "投流对比数据读取失败");
        setResult(payload.data);
        setSelectedKey((current) => payload.data?.items.some((item) => itemKey(item) === current)
          ? current
          : payload.data?.items[0] ? itemKey(payload.data.items[0]) : "");
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "投流对比数据读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [page, searchQuery, windowOption]);

  const selected = result.items.find((item) => itemKey(item) === selectedKey) ?? result.items[0];
  const pageCount = Math.max(1, Math.ceil(result.total / result.page_size));
  const dateRange = result.report_dates.length > 0
    ? `${result.report_dates[0]} - ${result.report_dates[result.report_dates.length - 1]}`
    : "暂无报表日期";

  return <>
    <section className="page-heading analysis-page-heading">
      <div><h1>投流情况对比</h1><p>笔记ID + 场域 · 不同计划回搜成本</p></div>
      <div className="heading-status"><span className={`status-dot ${serviceState}`} />{serviceState === "online" ? "分析服务已连接" : serviceState === "offline" ? "分析服务未连接" : "正在检查连接"}</div>
    </section>
    <section className="analysis-toolbar">
      <label className="analysis-search"><Search size={16} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索笔记ID、计划或场域" /></label>
      <div className="analysis-range"><span>{dateRange} · 当天 {result.latest_date || "-"}</span><div className="segmented-control" aria-label="投流对比时间范围">
        {WINDOW_OPTIONS.map((option) => <button key={option.value} className={windowOption === option.value ? "active" : ""} onClick={() => { setWindowOption(option.value); setPage(1); }}>{option.label}</button>)}
      </div></div>
    </section>
    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <div className="comparison-detail-wrap">
      {loading && !selected ? <div className="analysis-loading"><LoaderCircle size={20} className="spin" />正在读取投流对比</div>
        : selected ? <ComparisonDetail item={selected} dates={result.report_dates} latestDate={result.latest_date} />
          : <div className="analysis-loading">暂无投流数据</div>}
    </div>

    <section className="analysis-table-section comparison-list-section">
      <header>
        <div className="analysis-table-title"><h2>笔记场域列表</h2><p>{result.total.toLocaleString()} 个组合 · 差异优先，成本第二</p></div>
        <div className="analysis-table-actions"><ArrowDownWideNarrow size={15} /><span>固定排序</span>{loading ? <LoaderCircle size={18} className="spin" /> : null}</div>
      </header>
      <div className="analysis-table-wrap"><table className="comparison-list-table">
        <thead><tr><th>笔记ID</th><th>场域</th><th>计划数</th><th>当天成本差异</th><th>最低回搜成本</th><th>最高回搜成本</th><th>当天总消耗</th><th>当天回搜人数</th><th /></tr></thead>
        <tbody>{result.items.map((item) => <tr key={itemKey(item)} className={selected && itemKey(item) === itemKey(selected) ? "selected" : ""} onClick={() => setSelectedKey(itemKey(item))}>
          <td><strong title={item.note_id}>{item.note_id}</strong></td>
          <td><span className={`placement-swatch placement-${item.placement}`}>{item.placement}</span></td>
          <td>{item.campaign_count}</td>
          <td><span className={`comparison-gap-badge ${item.comparable_campaign_count >= 2 ? "has-gap" : ""}`}>{item.campaign_count === 1 ? "无比较" : item.comparable_campaign_count >= 2 ? "¥" + money.format(item.search_cost_gap) : "有效成本不足"}</span></td>
          <td>{item.comparable_campaign_count > 0 ? "¥" + money.format(item.latest_search_cost_min) : "-"}</td>
          <td>{item.comparable_campaign_count > 0 ? "¥" + money.format(item.latest_search_cost_max) : "-"}</td>
          <td>¥{money.format(item.latest_spend)}</td>
          <td>{integer.format(item.latest_search_users)}</td>
          <td><GitCompareArrows size={15} /></td>
        </tr>)}</tbody>
      </table></div>
      <footer className="analysis-pagination"><span>第 {result.page}/{pageCount} 页</span><div>
        <button className="icon-button" title="上一页" aria-label="上一页" disabled={page <= 1 || loading} onClick={() => setPage((current) => Math.max(1, current - 1))}><ChevronLeft size={17} /></button>
        <button className="icon-button" title="下一页" aria-label="下一页" disabled={page >= pageCount || loading} onClick={() => setPage((current) => current + 1)}><ChevronRight size={17} /></button>
      </div></footer>
    </section>
  </>;
}
