import { useEffect, useMemo, useState, type FormEvent } from "react";
import {
  AlertCircle, ArrowLeft, CalendarDays, ChevronLeft, ChevronRight, CircleDollarSign,
  FileJson2, Layers3, Lightbulb, LoaderCircle, Megaphone, Rows3, Search, X
} from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import "./spotlight-campaigns.css";

type CampaignSummary = {
  advertiser_id: number; advertiser_name: string; campaign_id: number; campaign_name: string;
  campaign_filter_state: number; campaign_enable: number; marketing_target: number; placement: number;
  bidding_strategy: number; campaign_day_budget: number; start_date?: string; expire_date?: string;
  updated_at?: string; synced_at: string; unit_count: number; creativity_count: number;
};

type CampaignEntity = {
  id: number; name: string; campaign_id: number; unit_id?: number; enable: number; filter_state: number;
  created_at?: string; updated_at?: string; synced_at: string; raw_payload: Record<string, unknown>;
};

type CampaignList = { total: number; page: number; page_size: number; items: CampaignSummary[] };
type CampaignDetail = { campaign: CampaignSummary; raw_payload: Record<string, unknown>; units: CampaignEntity[]; creativities: CampaignEntity[] };

const EMPTY_LIST: CampaignList = { total: 0, page: 1, page_size: 25, items: [] };
const PAGE_SIZE = 25;
const integer = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
const money = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const dateTime = new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });

const campaignStates: Record<number, string> = { 1: "有效", 2: "暂停", 3: "已删除", 4: "计划预算不足", 5: "现金余额不足", 7: "账户日预算不足", 8: "暂停阶段", 10: "未投放" };
const unitStates: Record<number, string> = { 1: "已删除", 2: "未开始", 3: "已结束", 4: "暂停", 5: "暂停时段", 6: "被计划暂停", 7: "现金余额不足", 8: "计划预算不足", 9: "所有未删除", 10: "有效", 11: "账户日预算不足", 12: "广告组预算不足", 13: "广告组暂停" };
const creativityStates: Record<number, string> = { 1: "已删除", 2: "所有未删除", 3: "暂停", 4: "被单元暂停", 5: "被计划暂停", 6: "现金余额不足", 7: "计划预算不足", 8: "有效", 9: "审核中", 10: "单元已结束", 11: "单元暂停时段", 12: "单元暂停", 13: "计划预算不足", 14: "现金余额不足", 16: "账户日预算不足" };
const marketingTargets: Record<number, string> = { 3: "商品销量", 4: "产品种草", 8: "直播推广", 9: "客资收集", 10: "抢占关键词", 13: "种草直达", 14: "直播预热", 15: "店铺拉新", 16: "应用唤起", 20: "应用下载", 21: "小程序推广" };
const placements: Record<number, string> = { 1: "信息流", 2: "搜索推广", 4: "全站智投", 7: "视频内流" };
const biddingStrategies: Record<number, string> = { 2: "手动出价", 3: "最大转化", 7: "稳定成本" };

const fieldLabels: Record<string, string> = {
  campaign_id: "计划 ID", campaign_name: "计划名称", campaign_filter_state: "计划状态", campaign_enable: "计划开关", campaign_create_time: "计划创建时间", campaign_update_time: "计划更新时间",
  marketing_target: "营销目标", placement: "投放场域", optimize_target: "优化目标", optimize_objective: "优化目标", deep_optimize_objective: "深度优化目标", promotion_target: "投放标的",
  bidding_strategy: "出价策略", constraint_type: "成本约束类型", constraint_value: "成本约束值", limit_day_budget: "日预算限制", campaign_day_budget: "计划日预算", budget_state: "预算状态",
  smart_switch: "智能开关", platform: "平台", pacing_mode: "投放速度", start_time: "开始日期", expire_time: "结束日期", time_period: "投放时段", time_period_type: "投放时段类型",
  feed_flag: "信息流标记", search_flag: "搜索标记", search_bid_ratio: "搜索出价比例", build_type: "搭建类型", creativity_state: "创意状态", event_asset_id: "事件资产 ID", asset_event: "资产事件",
  asset_event_id: "资产事件 ID", page_category: "页面类别", deeplink_id: "Deep Link ID", universal_link_id: "Universal Link ID", detect_url_link: "监测链接", not_available_status: "不可用状态",
  creation_type: "创建类型", marketing_industry: "营销行业", id: "单元 ID", name: "单元名称", unit_filter_state: "单元状态", enable: "单元开关", event_bid: "单元出价", target_type: "定向类型",
  target_config: "定向配置", target_template_id: "定向模板 ID", targetTemplateId: "定向模板 ID", keyword_gen_type: "关键词生成方式", keyword_with_bids: "搜索竞价关键词", keyword_target_period: "关键词行为周期",
  keyword_target_action: "关键词行为类型", note_ids: "笔记 ID", note_rec_type: "笔记推荐类型", item_note_info: "笔记配置", creativity_id: "创意 ID", creativity_name: "创意名称",
  creativity_enable: "创意开关", creativity_filter_state: "创意状态", creativity_create_time: "创意创建时间", creativity_update_time: "创意更新时间", material_type: "素材类型", conversion_type: "转化类型",
  note_id: "笔记 ID", item_id: "Item ID", audit_status: "审核状态", creativity_audit_state: "创意审核状态", primary_title: "主标题", title: "标题", custom_title: "自定义标题", comment: "评论文案",
  image: "图片", jump_url: "跳转地址", action_button_content: "行动按钮文案"
};

function enumLabel(values: Record<number, string>, value: number): string { return values[value] ?? `官方码值 ${value}`; }
function formatDate(value?: string): string { if (!value) return "-"; const parsed = new Date(value); return Number.isNaN(parsed.getTime()) ? value.slice(0, 19) : dateTime.format(parsed); }
function campaignStateTone(value: number): string { if (value === 1) return "healthy"; return [2, 8, 10].includes(value) ? "paused" : "warning"; }
function unitStateTone(value: number): string { if (value === 10) return "healthy"; return [2, 3, 4, 5, 6, 13].includes(value) ? "paused" : "warning"; }
function creativityStateTone(value: number): string { if (value === 8) return "healthy"; return [3, 4, 5, 10, 11, 12].includes(value) ? "paused" : "warning"; }
function campaignPath(campaign: CampaignSummary): string { return "/delivery/campaigns?" + new URLSearchParams({ advertiser_id: String(campaign.advertiser_id), campaign_id: String(campaign.campaign_id) }).toString(); }
function valueSummary(value: unknown): string { if (Array.isArray(value)) return `${value.length} 项`; if (value && typeof value === "object") return `${Object.keys(value as Record<string, unknown>).length} 个字段`; return ""; }

function PrimitiveValue({ value }: { value: unknown }) {
  if (value === null || value === undefined || value === "") return <span className="spotlight-empty-value">未配置</span>;
  if (typeof value === "boolean") return <>{value ? "是" : "否"}</>;
  if (typeof value === "string" && /^https?:\/\//.test(value)) return <a href={value} target="_blank" rel="noreferrer">{value}</a>;
  return <>{String(value)}</>;
}

function RawPayload({ payload, title }: { payload: Record<string, unknown>; title: string }) {
  const entries = Object.entries(payload ?? {}).sort(([left], [right]) => (fieldLabels[left] ?? left).localeCompare(fieldLabels[right] ?? right, "zh-CN"));
  return <section className="spotlight-raw-payload" aria-label={title}>
    <header><FileJson2 size={15} /><div><strong>{title}</strong><span>{entries.length} 个聚光原始字段</span></div></header>
    {entries.length === 0 ? <div className="spotlight-empty">聚光未返回字段</div> : <dl className="spotlight-field-grid">{entries.map(([key, value]) => {
      const complex = Array.isArray(value) || (value !== null && typeof value === "object");
      return <div className={complex ? "complex" : ""} key={key}><dt>{fieldLabels[key] ?? key}<code>{key}</code></dt><dd>{complex ? <details><summary>{valueSummary(value)}</summary><pre>{JSON.stringify(value, null, 2)}</pre></details> : <PrimitiveValue value={value} />}</dd></div>;
    })}</dl>}
  </section>;
}

function CampaignListView({ searchParams, setSearchParams }: { searchParams: URLSearchParams; setSearchParams: ReturnType<typeof useSearchParams>[1] }) {
  const initialQuery = searchParams.get("q") ?? "";
  const initialPage = Math.max(Number(searchParams.get("page")) || 1, 1);
  const [input, setInput] = useState(initialQuery);
  const [query, setQuery] = useState(initialQuery);
  const [page, setPage] = useState(initialPage);
  const [result, setResult] = useState<CampaignList>(EMPTY_LIST);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError("");
    const params = new URLSearchParams({ page: String(page), page_size: String(PAGE_SIZE) });
    if (query) params.set("q", query);
    fetch(`${import.meta.env.BASE_URL}api/analytics/spotlight/campaigns?${params}`, { signal: controller.signal })
      .then(async (response) => { const payload = await response.json() as { success: boolean; data?: CampaignList; error?: string }; if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "无法读取聚光计划"); setResult(payload.data); })
      .catch((fetchError: unknown) => { if ((fetchError as { name?: string }).name !== "AbortError") setError(fetchError instanceof Error ? fetchError.message : "无法读取聚光计划"); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [page, query]);

  const submit = (event: FormEvent) => { event.preventDefault(); const next = input.trim(); setQuery(next); setPage(1); const params = new URLSearchParams(); if (next) params.set("q", next); setSearchParams(params); };
  const clear = () => { setInput(""); setQuery(""); setPage(1); setSearchParams(new URLSearchParams()); };
  const goToPage = (next: number) => { setPage(next); const params = new URLSearchParams(); if (query) params.set("q", query); if (next > 1) params.set("page", String(next)); setSearchParams(params); };
  const pages = Math.max(Math.ceil(result.total / PAGE_SIZE), 1);

  return <>
    <section className="page-heading spotlight-page-heading"><div><h1>计划详情</h1><p>投放管理 · 聚光计划配置与执行层级</p></div><div className="heading-status"><span className="status-dot" />聚光快照数据</div></section>
    <form className="spotlight-search-toolbar" role="search" onSubmit={submit}><label><Search size={16} /><input value={input} onChange={(event) => setInput(event.target.value)} placeholder="输入计划名称或计划 ID" aria-label="按计划名称或计划 ID 搜索" /></label>{input ? <button className="spotlight-clear" type="button" aria-label="清除计划搜索" title="清除" onClick={clear}><X size={15} /></button> : null}<button className="spotlight-search-button" type="submit"><Search size={14} />搜索</button><span>{loading ? "正在查询" : `共 ${integer.format(result.total)} 个计划`}</span></form>
    {error ? <div className="spotlight-error"><AlertCircle size={16} />{error}</div> : null}
    <section className="spotlight-list-section"><header><div><h2>聚光计划</h2><p>点击计划名称或 ID 查看聚光返回的全部配置字段</p></div><Megaphone size={18} /></header>
      {loading ? <div className="spotlight-loading"><LoaderCircle size={18} className="spin" />正在加载计划</div> : result.items.length === 0 ? <div className="spotlight-empty">没有符合条件的聚光计划</div> : <div className="spotlight-table-wrap"><table aria-label="聚光计划搜索结果"><thead><tr><th>计划</th><th>广告主</th><th>场域</th><th>营销目标</th><th>状态</th><th>日预算</th><th>单元</th><th>创意</th><th>更新时间</th></tr></thead><tbody>{result.items.map((item) => <tr key={`${item.advertiser_id}-${item.campaign_id}`}><td><Link className="spotlight-plan-link" to={campaignPath(item)}><strong>{item.campaign_name || "未命名计划"}</strong><code>{item.campaign_id}</code></Link></td><td><span>{item.advertiser_name || "未知广告主"}</span><code>{item.advertiser_id}</code></td><td>{enumLabel(placements, item.placement)}</td><td>{enumLabel(marketingTargets, item.marketing_target)}</td><td><span className={`spotlight-state ${campaignStateTone(item.campaign_filter_state)}`}>{enumLabel(campaignStates, item.campaign_filter_state)}</span></td><td>¥{money.format(item.campaign_day_budget / 100)}</td><td>{integer.format(item.unit_count)}</td><td>{integer.format(item.creativity_count)}</td><td>{formatDate(item.updated_at)}</td></tr>)}</tbody></table></div>}
      <footer><span>第 {page} / {pages} 页</span><div><button type="button" aria-label="上一页" disabled={page <= 1 || loading} onClick={() => goToPage(page - 1)}><ChevronLeft size={15} /></button><button type="button" aria-label="下一页" disabled={page >= pages || loading} onClick={() => goToPage(page + 1)}><ChevronRight size={15} /></button></div></footer>
    </section>
  </>;
}

function CampaignDetailView({ advertiserID, campaignID }: { advertiserID: number; campaignID: number }) {
  const [detail, setDetail] = useState<CampaignDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ advertiser_id: String(advertiserID), campaign_id: String(campaignID) });
    setLoading(true); setError("");
    fetch(`${import.meta.env.BASE_URL}api/analytics/spotlight/campaign-detail?${params}`, { signal: controller.signal })
      .then(async (response) => { const payload = await response.json() as { success: boolean; data?: CampaignDetail; error?: string }; if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "无法读取计划详情"); setDetail(payload.data); })
      .catch((fetchError: unknown) => { if ((fetchError as { name?: string }).name !== "AbortError") setError(fetchError instanceof Error ? fetchError.message : "无法读取计划详情"); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [advertiserID, campaignID]);

  const creativitiesByUnit = useMemo(() => { const grouped = new Map<number, CampaignEntity[]>(); for (const creativity of detail?.creativities ?? []) grouped.set(creativity.unit_id ?? 0, [...(grouped.get(creativity.unit_id ?? 0) ?? []), creativity]); return grouped; }, [detail]);
  if (loading) return <div className="spotlight-detail-loading"><LoaderCircle size={20} className="spin" />正在读取聚光计划全部维度</div>;
  if (error || !detail) return <><Link className="spotlight-back" to="/delivery/campaigns"><ArrowLeft size={15} />返回计划列表</Link><div className="spotlight-error"><AlertCircle size={16} />{error || "计划不存在"}</div></>;
  const campaign = detail.campaign;
  return <>
    <Link className="spotlight-back" to="/delivery/campaigns"><ArrowLeft size={15} />返回计划列表</Link>
    <section className="spotlight-detail-heading"><div className="spotlight-detail-icon"><Megaphone size={21} /></div><div><h1>{campaign.campaign_name || "未命名计划"}</h1><p>{campaign.advertiser_name} · 广告主 {campaign.advertiser_id} · 计划 {campaign.campaign_id}</p></div><span className={`spotlight-state ${campaignStateTone(campaign.campaign_filter_state)}`}>{enumLabel(campaignStates, campaign.campaign_filter_state)}</span></section>
    <section className="spotlight-summary-strip" aria-label="计划执行摘要"><div><CircleDollarSign size={15} /><span>计划日预算</span><strong>¥{money.format(campaign.campaign_day_budget / 100)}</strong></div><div><Layers3 size={15} /><span>投放场域</span><strong>{enumLabel(placements, campaign.placement)}</strong></div><div><Rows3 size={15} /><span>广告单元</span><strong>{integer.format(campaign.unit_count)}</strong></div><div><Lightbulb size={15} /><span>投放创意</span><strong>{integer.format(campaign.creativity_count)}</strong></div><div><CalendarDays size={15} /><span>执行周期</span><strong>{campaign.start_date || "-"} 至 {campaign.expire_date || "-"}</strong></div></section>
    <section className="spotlight-readable-section"><header><h2>计划执行配置</h2><span>聚光快照同步于 {formatDate(campaign.synced_at)}</span></header><dl><div><dt>营销目标</dt><dd>{enumLabel(marketingTargets, campaign.marketing_target)}</dd></div><div><dt>出价策略</dt><dd>{enumLabel(biddingStrategies, campaign.bidding_strategy)}</dd></div><div><dt>计划开关</dt><dd>{campaign.campaign_enable === 1 ? "开启" : "关闭"}</dd></div><div><dt>计划更新时间</dt><dd>{formatDate(campaign.updated_at)}</dd></div></dl></section>
    <RawPayload payload={detail.raw_payload} title="计划全部字段" />
    <section className="spotlight-hierarchy-section"><header><div><Rows3 size={17} /><div><h2>广告单元</h2><p>出价、定向、人群、地域、关键词与笔记配置</p></div></div><span>{detail.units.length} 个</span></header>{detail.units.length === 0 ? <div className="spotlight-empty">该计划没有有效广告单元</div> : <div className="spotlight-entity-list">{detail.units.map((unit) => <details className="spotlight-entity" open key={unit.id}><summary><div><strong>{unit.name || "未命名单元"}</strong><code>单元 {unit.id}</code></div><span className={`spotlight-state ${unitStateTone(unit.filter_state)}`}>{enumLabel(unitStates, unit.filter_state)}</span><small>{(creativitiesByUnit.get(unit.id) ?? []).length} 个创意</small></summary><div className="spotlight-entity-meta"><span>开关：{unit.enable === 1 ? "开启" : "关闭"}</span><span>创建：{formatDate(unit.created_at)}</span><span>更新：{formatDate(unit.updated_at)}</span><span>同步：{formatDate(unit.synced_at)}</span></div><RawPayload payload={unit.raw_payload} title={`单元 ${unit.id} 全部字段`} /></details>)}</div>}</section>
    <section className="spotlight-hierarchy-section"><header><div><Lightbulb size={17} /><div><h2>投放创意</h2><p>素材、笔记、标题、审核、跳转与转化配置</p></div></div><span>{detail.creativities.length} 个</span></header>{detail.creativities.length === 0 ? <div className="spotlight-empty">该计划没有有效投放创意</div> : <div className="spotlight-entity-list creativity">{detail.creativities.map((creativity) => <details className="spotlight-entity" key={creativity.id}><summary><div><strong>{creativity.name || "未命名创意"}</strong><code>创意 {creativity.id} · 单元 {creativity.unit_id || "-"}</code></div><span className={`spotlight-state ${creativityStateTone(creativity.filter_state)}`}>{enumLabel(creativityStates, creativity.filter_state)}</span><small>{String(creativity.raw_payload?.note_id || "无笔记 ID")}</small></summary><div className="spotlight-entity-meta"><span>开关：{creativity.enable === 1 ? "开启" : "关闭"}</span><span>创建：{formatDate(creativity.created_at)}</span><span>更新：{formatDate(creativity.updated_at)}</span><span>同步：{formatDate(creativity.synced_at)}</span></div><RawPayload payload={creativity.raw_payload} title={`创意 ${creativity.id} 全部字段`} /></details>)}</div>}</section>
  </>;
}

function SpotlightCampaigns() {
  const [searchParams, setSearchParams] = useSearchParams();
  const advertiserID = Number(searchParams.get("advertiser_id"));
  const campaignID = Number(searchParams.get("campaign_id"));
  const detail = Number.isSafeInteger(advertiserID) && advertiserID > 0 && Number.isSafeInteger(campaignID) && campaignID > 0;
  return detail ? <CampaignDetailView advertiserID={advertiserID} campaignID={campaignID} /> : <CampaignListView searchParams={searchParams} setSearchParams={setSearchParams} />;
}

export default SpotlightCampaigns;
