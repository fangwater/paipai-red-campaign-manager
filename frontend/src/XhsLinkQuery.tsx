import { useEffect, useMemo, useState } from "react";
import {
  AlertCircle, Building2, CalendarDays, CheckCircle2, ChevronLeft, ChevronRight,
  CircleDollarSign, CircleHelp, ChevronDown, ChevronUp, Lightbulb, LoaderCircle, MapPin, Megaphone, Rows3, Search, Smartphone, Tags, Target, Users, X
} from "lucide-react";
import TargetRegionMap from "./TargetRegionMap";

type Creativity = {
  creativity_id: number;
  creativity_name: string;
  creativity_enable: number;
  creativity_filter_state: number;
  material_type: number;
  conversion_type: number;
  note_id: string;
  item_id: string;
  audit_status: number;
  creativity_audit_state: number;
  creation_type: number;
  created_at?: string;
  updated_at?: string;
  synced_at: string;
};

type SearchKeyword = {
  keyword_id: number;
  keyword: string;
  bid: number;
  feed_bid: number;
  keyword_source: number;
  phrase_match_type: number;
};

type CrowdPackage = { value: string; name: string; group_id?: string; type?: string; tag?: string; status: number; sync_status: number; };
type PremiumCrowd = { id: string; name: string; ratio: string; };

type TargetConfig = {
  gender: string; age: string; city: string; area_code: string; device: string; device_price: string;
  intelligent_expansion: number; generalization_switch: number; search_city_intent: string;
  interest_keywords: string[]; behavior_keywords: string[]; excluded_crowds: string[];
  crowd_packages: CrowdPackage[]; content_interests: string[]; shopping_interests: string[];
  premium_crowds: PremiumCrowd[]; dandelion_crowds: string[];
  brand_interest_group: boolean; brand_recognition_group: boolean; category_interest_group: boolean; goods_interest_group: boolean;
};

type UnitDelivery = {
  target_template_id: number; keyword_gen_type: number; keyword_target_period: number; keyword_target_actions: number[];
  search_keyword_count: number; search_keywords: SearchKeyword[]; target: TargetConfig;
};

type Unit = {
  unit_id: number;
  unit_name: string;
  unit_enable: number;
  unit_filter_state: number;
  event_bid: number;
  target_type: number;
  not_available_status: number;
  creation_type: number;
  created_at?: string;
  updated_at?: string;
  synced_at: string;
  delivery?: UnitDelivery;
  creativities: Creativity[];
};

type Match = {
  advertiser_id: number;
  advertiser_name: string;
  campaign_id: number;
  campaign_name: string;
  campaign_filter_state: number;
  campaign_enable: number;
  marketing_target: number;
  placement: number;
  optimize_target: number;
  optimize_objective: number;
  deep_optimize_objective: number;
  promotion_target: number;
  bidding_strategy: number;
  campaign_day_budget: number;
  campaign_created_at?: string;
  campaign_updated_at?: string;
  start_date?: string;
  expire_date?: string;
  synced_at: string;
  units: Unit[];
};

type LinkItem = {
  note_id: string;
  placement: string;
  spend: number;
  search_users: number;
  search_cost: number;
  matches: Match[];
};

type LinkResult = {
  report_date: string;
  total: number;
  page: number;
  page_size: number;
  items: LinkItem[];
};

const EMPTY_RESULT: LinkResult = { report_date: "", total: 0, page: 1, page_size: 25, items: [] };
const money = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const count = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
const dateTime = new Intl.DateTimeFormat("zh-CN", {
  year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false
});

const campaignStates: Record<number, string> = {
  1: "有效", 2: "暂停", 3: "已删除", 4: "计划预算不足", 5: "现金余额不足",
  7: "账户日预算不足", 8: "暂停阶段"
};
const marketingTargets: Record<number, string> = {
  3: "商品销量", 4: "产品种草", 8: "直播推广", 9: "客资收集", 10: "抢占关键词",
  13: "种草直达", 14: "直播预热", 15: "店铺拉新", 16: "应用唤起", 20: "应用下载", 21: "小程序推广"
};
const campaignPlacements: Record<number, string> = { 1: "信息流", 2: "搜索推广", 4: "全站智投", 7: "视频内流" };
const promotionTargets: Record<number, string> = { 1: "笔记", 9: "落地页" };
const biddingStrategies: Record<number, string> = { 2: "手动出价", 3: "最大转化", 7: "稳定成本" };
const unitStates: Record<number, string> = {
  1: "已删除", 2: "未开始", 3: "已结束", 4: "暂停", 5: "暂停时段", 6: "被计划暂停",
  7: "现金余额不足", 8: "计划预算不足", 9: "所有未删除", 10: "有效",
  11: "账户日预算不足", 12: "广告组预算不足", 13: "广告组暂停"
};
const targetTypes: Record<number, string> = { 0: "默认定向", 1: "通投", 2: "智能定向", 3: "高级定向" };
const unitAvailability: Record<number, string> = { 0: "创意不为空", 1: "创意为空" };
const unitCreationTypes: Record<number, string> = { 0: "标准投", 1: "简单投", 2: "留资快投", 3: "R2", 4: "简单投半自动" };
const creativityStates: Record<number, string> = {
  1: "已删除", 2: "所有未删除", 3: "暂停", 4: "被单元暂停", 5: "被计划暂停",
  6: "审核拒绝", 7: "审核中", 8: "有效", 9: "商品状态异常", 10: "单元未开始",
  11: "单元已结束", 12: "单元暂停时段", 13: "计划预算不足", 14: "现金余额不足", 16: "账户日预算不足"
};
const materialTypes: Record<number, string> = { 1: "笔记", 2: "H5", 3: "商品", 13: "直播间" };
const conversionTypes: Record<number, string> = {
  0: "无组件", 1: "商品组件", 2: "落地页组件", 3: "私信组件", 4: "直播组件",
  5: "POI门店组件", 6: "商品/小程序组件", 7: "直播间", 8: "搜索组件", 9: "小程序组件",
  10: "留资组件", 11: "唤端组件", 12: "企微组件", 13: "下载组件", 14: "预约组件",
  15: "红书小程序组件", 16: "微信小程序组件", 20: "落地页", 23: "直播预热",
  30: "商品", 32: "种草直达落地页", 40: "直播", 50: "开屏广告", 78: "私信表单同投组件"
};
const auditStatuses: Record<number, string> = { 0: "创建待审核", 1: "审核通过", 2: "审核拒绝", 3: "修改待审核", 7: "审核通过（私密笔记）" };
const creativityAuditStates: Record<number, string> = { 1: "审核拒绝", 2: "审核中", 3: "审核通过", 4: "审核通过（私密）", 99: "不满足审核条件" };
const creativityCreationTypes: Record<number, string> = { 0: "标准投", 1: "简单投", 2: "留资快投", 4: "简单投半自动" };

const optimizeObjectives: Record<number, Record<number, string>> = {
  4: { 0: "点击量", 1: "互动量", 18: "站外转化量", 30: "种草人群规模", 31: "深度种草人群规模", 51: "点击份额（SOC）" },
  9: { 3: "表单提交量", 5: "私信进线量", 13: "私信开口量", 50: "私信留资量", 78: "线索流资量" },
  13: { 19: "组件点击（点击归因）", 21: "店铺成交（点击归因）", 44: "店铺访问（阅读归因）", 45: "店铺成交（阅读归因）" },
  16: { 0: "点击量", 35: "APP打开", 36: "APP进店", 37: "APP互动", 38: "APP支付", 39: "APP支付ROI", 43: "笔记唤端组件点击" },
  20: { 60: "APP下载按钮点击", 61: "APP激活", 62: "APP注册", 63: "APP关键行为", 64: "APP付费", 69: "APP预约下载按钮点击", 72: "APP预约下载" },
  21: { 65: "小程序打开", 67: "小程序支付订单数", 68: "小程序ROI", 73: "微信小游戏打开", 74: "微信小游戏激活", 75: "微信小游戏订单支付数" }
};

const optimizeObjectiveGroups = [
  { marketingTarget: 4, name: "产品种草", options: optimizeObjectives[4] },
  { marketingTarget: 9, name: "客资收集", options: optimizeObjectives[9] },
  { marketingTarget: 16, name: "应用唤起", options: optimizeObjectives[16] },
  { marketingTarget: 20, name: "应用下载", options: optimizeObjectives[20] },
  { marketingTarget: 21, name: "小程序推广", options: optimizeObjectives[21] },
  { marketingTarget: 13, name: "种草直达", options: optimizeObjectives[13] }
];

function itemKey(item: LinkItem): string {
  return `${item.note_id}\u0000${item.placement}`;
}

function enumLabel(values: Record<number, string>, value: number): string {
  return values[value] ?? `官方未定义（${value}）`;
}

function optimizeObjectiveLabel(marketingTarget: number, objective: number): string {
  return optimizeObjectives[marketingTarget]?.[objective] ?? `官方未定义（${objective}）`;
}

function campaignState(state: number): string {
  return enumLabel(campaignStates, state);
}

function campaignStateTone(state: number): string {
  return state === 1 ? "healthy" : state === 2 || state === 8 ? "paused" : "warning";
}

function unitStateTone(state: number): string {
  return state === 10 ? "healthy" : [2, 3, 4, 5, 6, 13].includes(state) ? "paused" : "warning";
}

function creativityStateTone(state: number): string {
  return state === 8 ? "healthy" : [3, 4, 5, 10, 11, 12].includes(state) ? "paused" : "warning";
}

function enableLabel(enable: number): string {
  return enable === 1 ? "开启" : enable === 0 ? "关闭" : `官方未定义（${enable}）`;
}

function formatDate(value?: string): string {
  if (!value) return "-";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? value.slice(0, 19) : dateTime.format(parsed);
}

function unitCount(item: LinkItem): number {
  return item.matches.reduce((total, match) => total + match.units.length, 0);
}

function creativityCount(item: LinkItem): number {
  return item.matches.reduce((total, match) =>
    total + match.units.reduce((sum, unit) => sum + unit.creativities.length, 0), 0);
}

const EMPTY_DELIVERY: UnitDelivery = {
  target_template_id: 0, keyword_gen_type: -1, keyword_target_period: 0, keyword_target_actions: [],
  search_keyword_count: 0, search_keywords: [],
  target: {
    gender: "", age: "", city: "", area_code: "", device: "", device_price: "",
    intelligent_expansion: 0, generalization_switch: 0, search_city_intent: "",
    interest_keywords: [], behavior_keywords: [], excluded_crowds: [], crowd_packages: [],
    content_interests: [], shopping_interests: [], premium_crowds: [], dandelion_crowds: [],
    brand_interest_group: false, brand_recognition_group: false, category_interest_group: false, goods_interest_group: false
  }
};

function genderLabel(value: string): string {
  return value === "0" ? "男" : value === "1" ? "女" : "不限";
}

function deviceLabel(value: string): string {
  return value === "ios" ? "苹果设备" : value === "android" ? "安卓设备" : "不限";
}

function separatedSummary(value: string, fallback: string, limit = 5): string {
  if (!value || value === "all" || value === "-1") return fallback;
  const parts = value.split("#").filter(Boolean);
  return parts.length > limit ? parts.slice(0, limit).join("、") + "等 " + parts.length + " 项" : parts.join("、");
}

function keywordGenLabel(value: number): string {
  return ({ [-1]: "未启用拓词", 0: "手动选词", 1: "智能拓词", 2: "手动 + 智能" } as Record<number, string>)[value] ?? "未配置";
}

function keywordActionLabel(actions: number[]): string {
  const labels: Record<number, string> = { 1: "搜索", 2: "互动", 3: "阅读" };
  return actions.map((action) => labels[action]).filter(Boolean).join("、");
}

function searchIntentLabel(value: string): string {
  return ({ "0": "居住地用户", "1": "居住或意图用户", "2": "所有用户" } as Record<string, string>)[value] ?? "未配置";
}

function phraseMatchLabel(value: number): string {
  return ({ 0: "精确匹配", 1: "短语匹配", 2: "智能匹配" } as Record<number, string>)[value] ?? "官方未定义（" + value + "）";
}

function DeliveryTagGroup({ label, values, meta }: { label: string; values: string[]; meta?: string }) {
  if (values.length === 0) return null;
  return <div className="delivery-tag-group"><div><strong>{label}</strong>{meta ? <span>{meta}</span> : null}</div><div>{values.map((value, index) => <span title={value} key={value + index}>{value}</span>)}</div></div>;
}

function UnitDeliveryDetail({ unit }: { unit: Unit }) {
  const [expanded, setExpanded] = useState(false);
  const delivery = unit.delivery ?? EMPTY_DELIVERY;
  const target = delivery.target;
  const keywords = expanded ? delivery.search_keywords : delivery.search_keywords.slice(0, 12);
  const crowdValues = [
    ...target.crowd_packages.map((crowd) => crowd.name || crowd.value),
    ...target.premium_crowds.map((crowd) => crowd.name + (crowd.ratio ? " · " + crowd.ratio + " 倍" : "")),
    ...target.dandelion_crowds
  ].filter(Boolean);
  const groupValues = [
    target.brand_interest_group ? "本品牌兴趣人群" : "", target.brand_recognition_group ? "本品牌认知人群" : "",
    target.category_interest_group ? "行业兴趣人群" : "", target.goods_interest_group ? "商品兴趣人群" : ""
  ].filter(Boolean);
  const industryValues = [...target.content_interests, ...target.shopping_interests];
  const action = keywordActionLabel(delivery.keyword_target_actions);
  const behaviorMeta = [delivery.keyword_target_period > 0 ? "近 " + delivery.keyword_target_period + " 天" : "", action].filter(Boolean).join(" · ");
  const regionCount = target.city && target.city !== "all" ? target.city.split("#").filter(Boolean).length : 0;

  return <section className="unit-delivery-panel" data-unit-id={unit.unit_id}>
    <header><div><span><Target size={16} /></span><div><h5>{unit.unit_name || "未命名单元"}</h5><p>单元 {unit.unit_id}</p></div></div><div>
      {delivery.target_template_id > 0 ? <span>定向包 {delivery.target_template_id}</span> : null}
      <span>{keywordGenLabel(delivery.keyword_gen_type)}</span><strong>{delivery.search_keyword_count} 个搜索词</strong>
    </div></header>
    <dl className="target-overview-grid">
      <div><dt>定向模式</dt><dd>{enumLabel(targetTypes, unit.target_type)}</dd></div>
      <div><dt><Users size={12} />性别</dt><dd>{genderLabel(target.gender)}</dd></div>
      <div><dt>年龄</dt><dd title={target.age}>{separatedSummary(target.age, "不限")}</dd></div>
      <div><dt><MapPin size={12} />地域</dt><dd>{target.city === "all" || regionCount === 0 ? "全国" : regionCount + " 个地域"}</dd></div>
      <div><dt><Smartphone size={12} />设备</dt><dd>{deviceLabel(target.device)}</dd></div>
      <div><dt>手机价格</dt><dd title={target.device_price}>{separatedSummary(target.device_price, "不限", 3)}</dd></div>
      <div><dt>智能扩量</dt><dd>{target.intelligent_expansion === 1 ? "开启" : "关闭"}</dd></div>
      <div><dt>定向拓宽</dt><dd>{target.generalization_switch === 1 ? "开启" : "关闭"}</dd></div>
      <div><dt>地域意图</dt><dd>{searchIntentLabel(target.search_city_intent)}</dd></div>
    </dl>
    <TargetRegionMap city={target.city} areaCode={target.area_code} />
    <div className="audience-groups">
      <DeliveryTagGroup label="人群包" values={crowdValues} />
      <DeliveryTagGroup label="平台人群" values={groupValues} />
      <DeliveryTagGroup label="行业兴趣" values={industryValues} />
      <DeliveryTagGroup label="兴趣关键词" values={target.interest_keywords} />
      <DeliveryTagGroup label="行为关键词" values={target.behavior_keywords} meta={behaviorMeta} />
      <DeliveryTagGroup label="排除人群" values={target.excluded_crowds} />
      {crowdValues.length + groupValues.length + industryValues.length + target.interest_keywords.length + target.behavior_keywords.length + target.excluded_crowds.length === 0 ? <div className="delivery-empty">未配置细分人群</div> : null}
    </div>
    <section className="search-keyword-section">
      <header><div><Tags size={14} /><h6>搜索竞价关键词</h6></div><span>{delivery.search_keyword_count} 个</span></header>
      {keywords.length > 0 ? <div className="search-keyword-table-wrap"><table><thead><tr><th>关键词</th><th>匹配方式</th><th>关键词出价</th><th>追投出价</th></tr></thead><tbody>{keywords.map((keyword) => <tr key={keyword.keyword_id || keyword.keyword}><td title={keyword.keyword}>{keyword.keyword}</td><td>{phraseMatchLabel(keyword.phrase_match_type)}</td><td>¥{money.format(keyword.bid / 100)}</td><td>{keyword.feed_bid > 0 ? "¥" + money.format(keyword.feed_bid / 100) : "-"}</td></tr>)}</tbody></table></div> : <div className="delivery-empty">未配置搜索竞价词</div>}
      {delivery.search_keyword_count > 12 ? <button type="button" className="delivery-expand-button" onClick={() => setExpanded((current) => !current)}>{expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}{expanded ? "收起关键词" : "展开全部 " + delivery.search_keyword_count + " 个关键词"}</button> : null}
    </section>
  </section>;
}

function XhsLinkQuery() {
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<LinkResult>(EMPTY_RESULT);
  const [selectedKey, setSelectedKey] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [objectiveHelpTarget, setObjectiveHelpTarget] = useState<number | null>(null);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearchQuery(searchInput.trim());
      setPage(1);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ page: String(page), page_size: "25" });
    if (searchQuery) params.set("q", searchQuery);
    setLoading(true);
    setError("");
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/xhs-links?${params}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: LinkResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "聚光关联数据读取失败");
        setResult(payload.data);
        setSelectedKey((current) => payload.data?.items.some((item) => itemKey(item) === current)
          ? current
          : payload.data?.items[0] ? itemKey(payload.data.items[0]) : "");
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "聚光关联数据读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [page, searchQuery]);

  useEffect(() => {
    if (objectiveHelpTarget === null) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setObjectiveHelpTarget(null);
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [objectiveHelpTarget]);

  const selected = result.items.find((item) => itemKey(item) === selectedKey) ?? result.items[0];
  const pageCount = Math.max(1, Math.ceil(result.total / result.page_size));
  const selectedStats = useMemo(() => selected ? {
    advertisers: new Set(selected.matches.map((match) => match.advertiser_id)).size,
    units: unitCount(selected),
    creativities: creativityCount(selected)
  } : { advertisers: 0, units: 0, creativities: 0 }, [selected]);

  return <>
    <section className="page-heading xhs-link-page-heading">
      <div><h1>聚光关联查询</h1><p>笔记ID + 场域 · 聚光实体关联</p></div>
      <div className="heading-status"><span className={`status-dot ${error ? "offline" : loading ? "checking" : ""}`} />
        {error ? "关联服务异常" : loading ? "正在读取数据" : "聚光数据已关联"}
      </div>
    </section>

    <section className="xhs-link-toolbar">
      <label className="analysis-search"><Search size={16} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索笔记、计划、广告主、单元或创意" /></label>
      <div className="link-scope"><CalendarDays size={15} /><span>最新日报</span><strong>{result.report_date || "-"}</strong><b>{result.total.toLocaleString()} 个关联组合</b></div>
    </section>

    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <section className="xhs-link-table-section">
      <header><div><h2>关联结果</h2><p>按最新日报消耗排序，点击一行查看全部聚光层级</p></div>{loading ? <LoaderCircle size={18} className="spin" /> : <CheckCircle2 size={18} />}</header>
      <div className="xhs-link-table-wrap"><table className="xhs-link-table"><thead><tr>
        <th>笔记ID</th><th>关联计划</th><th>场域</th><th>广告主</th><th>首个计划ID</th><th>计划状态</th>
        <th>日预算</th><th>单元</th><th>创意</th><th>消耗</th><th>回搜人数</th><th>回搜成本</th>
      </tr></thead><tbody>
        {result.items.map((item) => {
          const primary = item.matches[0];
          return <tr key={itemKey(item)} className={selected && itemKey(item) === itemKey(selected) ? "selected" : ""} onClick={() => setSelectedKey(itemKey(item))}>
            <td title={item.note_id}><strong>{item.note_id}</strong></td>
            <td><div className="xhs-linked-plan-summary"><strong>{item.matches.length} 个</strong><span title={primary?.campaign_name}>{primary?.campaign_name || "未关联"}</span></div></td>
            <td><span className={`placement-swatch placement-${item.placement}`}>{item.placement}</span></td>
            <td title={primary?.advertiser_name}>{primary?.advertiser_name || "-"}</td>
            <td>{primary?.campaign_id ?? "-"}</td>
            <td>{primary ? <span className={`entity-status ${campaignStateTone(primary.campaign_filter_state)}`}>{campaignState(primary.campaign_filter_state)}</span> : "-"}</td>
            <td>{primary ? `¥${money.format(primary.campaign_day_budget / 100)}` : "-"}</td>
            <td>{unitCount(item)}</td><td>{creativityCount(item)}</td>
            <td>¥{money.format(item.spend)}</td><td>{count.format(item.search_users)}</td><td>¥{money.format(item.search_cost)}</td>
          </tr>;
        })}
      </tbody></table>{!loading && result.items.length === 0 ? <div className="sync-empty">没有符合条件的聚光关联数据</div> : null}</div>
      <footer className="analysis-pagination"><span>第 {result.page}/{pageCount} 页</span><div>
        <button className="icon-button" title="上一页" aria-label="上一页" disabled={page <= 1 || loading} onClick={() => setPage((current) => Math.max(1, current - 1))}><ChevronLeft size={17} /></button>
        <button className="icon-button" title="下一页" aria-label="下一页" disabled={page >= pageCount || loading} onClick={() => setPage((current) => current + 1)}><ChevronRight size={17} /></button>
      </div></footer>
    </section>

    {selected ? <section className="xhs-link-detail">
      <header className="link-detail-heading">
        <div><span className={`placement-swatch placement-${selected.placement}`}>{selected.placement}</span><div><h2>{selected.note_id}</h2><p>{selected.matches.length} 个聚光计划关联</p></div></div>
        <div className="link-detail-counts"><span><Building2 size={14} />{selectedStats.advertisers} 个广告主</span><span><Rows3 size={14} />{selectedStats.units} 个单元</span><span><Lightbulb size={14} />{selectedStats.creativities} 个创意</span></div>
      </header>
      <div className="daily-link-strip">
        <div><span>当天消耗</span><strong>¥{money.format(selected.spend)}</strong></div>
        <div><span>回搜人数</span><strong>{count.format(selected.search_users)}</strong></div>
        <div><span>回搜成本</span><strong>¥{money.format(selected.search_cost)}</strong></div>
      </div>

      {selected.matches.map((match) => <article className="linked-campaign" key={`${match.advertiser_id}-${match.campaign_id}`}>
        <header><div className="linked-campaign-title"><span><Megaphone size={18} /></span><div><h3>{match.campaign_name}</h3><p>{match.advertiser_name} · 广告主 {match.advertiser_id}</p></div></div>
          <div><span className={`entity-status ${campaignStateTone(match.campaign_filter_state)}`}>{campaignState(match.campaign_filter_state)}</span><strong>计划 {match.campaign_id}</strong></div>
        </header>
        <dl className="campaign-field-grid">
          <div><dt>计划是否可用</dt><dd>{match.campaign_enable === 1 ? "可用" : "不可用"}</dd></div>
          <div><dt>营销目标</dt><dd>{enumLabel(marketingTargets, match.marketing_target)}</dd></div>
          <div><dt>聚光场域</dt><dd>{enumLabel(campaignPlacements, match.placement)}</dd></div>
          <div><dt>计划日预算</dt><dd>¥{money.format(match.campaign_day_budget / 100)}</dd></div>
          <div><dt className="field-label-with-help">优化目标<button type="button" className="field-help-button" aria-label="查看优化目标配置说明" title="查看优化目标配置说明" onClick={() => setObjectiveHelpTarget(match.marketing_target)}><CircleHelp size={13} /></button></dt><dd title={`optimize_objective 原始码值 ${match.optimize_objective}`}>{optimizeObjectiveLabel(match.marketing_target, match.optimize_objective)}</dd></div>
          <div><dt>深度优化目标</dt><dd title={`deep_optimize_objective 原始码值 ${match.deep_optimize_objective}`}>{match.deep_optimize_objective === -1 ? "未启用" : optimizeObjectiveLabel(match.marketing_target, match.deep_optimize_objective)}</dd></div>
          <div><dt>投放标的</dt><dd title={`原始码值 ${match.promotion_target}`}>{enumLabel(promotionTargets, match.promotion_target)}</dd></div>
          <div><dt>出价策略</dt><dd title={`原始码值 ${match.bidding_strategy}`}>{enumLabel(biddingStrategies, match.bidding_strategy)}</dd></div>
          <div><dt>开始日期</dt><dd>{match.start_date || "-"}</dd></div>
          <div><dt>结束日期</dt><dd>{match.expire_date || "-"}</dd></div>
          <div><dt>计划创建时间</dt><dd>{formatDate(match.campaign_created_at)}</dd></div>
          <div><dt>计划更新时间</dt><dd>{formatDate(match.campaign_updated_at)}</dd></div>
          <div><dt>数据同步时间</dt><dd>{formatDate(match.synced_at)}</dd></div>
        </dl>

        <section className="linked-entity-section">
          <header><div><Rows3 size={16} /><h4>关联单元</h4></div><span>{match.units.length} 个</span></header>
          <div className="linked-entity-table-wrap"><table className="linked-entity-table unit-table"><thead><tr><th>单元ID</th><th>单元名称</th><th>状态</th><th>出价</th><th>定向类型</th><th>不可用状态</th><th>创建类型</th><th>更新时间</th><th>同步时间</th></tr></thead><tbody>
            {match.units.map((unit) => <tr key={unit.unit_id}><td>{unit.unit_id}</td><td title={unit.unit_name}>{unit.unit_name || "-"}</td><td><span className={`entity-status ${unitStateTone(unit.unit_filter_state)}`} title={`开关：${enableLabel(unit.unit_enable)} · 原始状态：${unit.unit_filter_state}`}>{enumLabel(unitStates, unit.unit_filter_state)}</span></td><td>¥{money.format(unit.event_bid / 100)}</td><td title={`原始码值 ${unit.target_type}`}>{enumLabel(targetTypes, unit.target_type)}</td><td title={`原始码值 ${unit.not_available_status}`}>{enumLabel(unitAvailability, unit.not_available_status)}</td><td title={`原始码值 ${unit.creation_type}`}>{enumLabel(unitCreationTypes, unit.creation_type)}</td><td>{formatDate(unit.updated_at)}</td><td>{formatDate(unit.synced_at)}</td></tr>)}
          </tbody></table></div>
          <div className="unit-delivery-list">{match.units.map((unit) => <UnitDeliveryDetail unit={unit} key={unit.unit_id} />)}</div>
        </section>

        <section className="linked-entity-section">
          <header><div><Lightbulb size={16} /><h4>关联创意</h4></div><span>{match.units.reduce((total, unit) => total + unit.creativities.length, 0)} 个</span></header>
          <div className="linked-entity-table-wrap"><table className="linked-entity-table creativity-table"><thead><tr><th>创意ID</th><th>创意名称</th><th>单元ID</th><th>状态</th><th>素材类型</th><th>转化类型</th><th>审核状态</th><th>创意审核</th><th>创建类型</th><th>Item ID</th><th>更新时间</th><th>同步时间</th></tr></thead><tbody>
            {match.units.flatMap((unit) => unit.creativities.map((creativity) => <tr key={creativity.creativity_id}><td>{creativity.creativity_id}</td><td title={creativity.creativity_name}>{creativity.creativity_name || "-"}</td><td>{unit.unit_id}</td><td><span className={`entity-status ${creativityStateTone(creativity.creativity_filter_state)}`} title={`开关：${enableLabel(creativity.creativity_enable)} · 原始状态：${creativity.creativity_filter_state}`}>{enumLabel(creativityStates, creativity.creativity_filter_state)}</span></td><td title={`原始码值 ${creativity.material_type}`}>{enumLabel(materialTypes, creativity.material_type)}</td><td title={`原始码值 ${creativity.conversion_type}`}>{enumLabel(conversionTypes, creativity.conversion_type)}</td><td title={`原始码值 ${creativity.audit_status}`}>{enumLabel(auditStatuses, creativity.audit_status)}</td><td title={`原始码值 ${creativity.creativity_audit_state}`}>{enumLabel(creativityAuditStates, creativity.creativity_audit_state)}</td><td title={`原始码值 ${creativity.creation_type}`}>{enumLabel(creativityCreationTypes, creativity.creation_type)}</td><td>{creativity.item_id || "-"}</td><td>{formatDate(creativity.updated_at)}</td><td>{formatDate(creativity.synced_at)}</td></tr>))}
          </tbody></table></div>
        </section>
      </article>)}
    </section> : null}

    {objectiveHelpTarget !== null ? <div className="objective-help-backdrop" onMouseDown={() => setObjectiveHelpTarget(null)}>
      <section className="objective-help-dialog" role="dialog" aria-modal="true" aria-labelledby="objective-help-title" onMouseDown={(event) => event.stopPropagation()}>
        <header><div><h2 id="objective-help-title">优化目标可配置项</h2><p>不同营销目标对应不同的优化目标枚举</p></div><button type="button" className="icon-button" aria-label="关闭优化目标配置说明" title="关闭" onClick={() => setObjectiveHelpTarget(null)}><X size={17} /></button></header>
        <div className="objective-help-groups">{optimizeObjectiveGroups.map((group) => <section className={group.marketingTarget === objectiveHelpTarget ? "current" : ""} key={group.marketingTarget}>
          <header><h3>{group.name}</h3>{group.marketingTarget === objectiveHelpTarget ? <span>当前计划</span> : null}</header>
          <div>{Object.entries(group.options).map(([code, label]) => <span key={code}><b>{code}</b>{label}</span>)}</div>
        </section>)}</div>
      </section>
    </div> : null}
  </>;
}

export default XhsLinkQuery;
