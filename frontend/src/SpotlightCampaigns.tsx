import { useEffect, useMemo, useState, type FormEvent } from "react";
import {
  AlertCircle, ArrowLeft, BookOpenCheck, CalendarDays, ChevronLeft, ChevronRight, CircleDollarSign, CircleHelp,
  FileJson2, Layers3, Lightbulb, LoaderCircle, Megaphone, Rows3, Search, X
} from "lucide-react";
import { Link, useSearchParams } from "react-router-dom";
import { SPOTLIGHT_FIELD_DOCS } from "./spotlight-config-docs";
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
type EntityLevel = "campaign" | "unit" | "creativity";
type FieldGuide = {
  description: string;
  options?: Record<number, string>;
  configurable?: boolean;
  format?: "fen" | "ratio";
  numericKind?: "identifier";
};

const EMPTY_LIST: CampaignList = { total: 0, page: 1, page_size: 25, items: [] };
const PAGE_SIZE = 25;
const integer = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
const money = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const dateTime = new Intl.DateTimeFormat("zh-CN", { year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });

const campaignStates: Record<number, string> = { 1: "有效", 2: "暂停", 3: "已删除", 4: "计划预算不足", 5: "现金余额不足", 7: "账户日预算不足", 8: "暂停阶段", 10: "未投放" };
const unitStates: Record<number, string> = { 1: "已删除", 2: "未开始", 3: "已结束", 4: "暂停", 5: "暂停时段", 6: "被计划暂停", 7: "现金余额不足", 8: "计划预算不足", 9: "所有未删除", 10: "有效", 11: "账户日预算不足", 12: "广告组预算不足", 13: "广告组暂停" };
const creativityStates: Record<number, string> = { 1: "已删除", 2: "所有未删除", 3: "暂停", 4: "被单元暂停", 5: "被计划暂停", 6: "审核拒绝", 7: "审核中", 8: "有效", 9: "商品状态异常", 10: "单元未开始", 11: "单元已结束", 12: "单元暂停时段", 13: "计划预算不足", 14: "现金余额不足", 16: "账户日预算不足" };
const marketingTargets: Record<number, string> = { 3: "商品销量", 4: "产品种草", 8: "直播推广", 9: "客资收集", 10: "抢占关键词", 13: "种草直达", 14: "直播预热", 15: "店铺拉新", 16: "应用唤起", 20: "应用下载", 21: "小程序推广" };
const placements: Record<number, string> = { 1: "信息流", 2: "搜索推广", 4: "全站智投", 7: "视频内流" };
const biddingStrategies: Record<number, string> = { 2: "手动出价", 3: "最大转化", 7: "稳定成本" };
const switchOptions: Record<number, string> = { 0: "关闭", 1: "开启" };
const promotionTargets: Record<number, string> = { 1: "笔记", 9: "落地页" };
const optimizeObjectives: Record<number, Record<number, string>> = {
  4: { 0: "点击量", 1: "互动量", 18: "站外转化量", 30: "种草人群规模", 31: "深度种草人群规模", 51: "点击份额（SOC）" },
  9: { 3: "表单提交量", 5: "私信进线量", 13: "私信开口量", 50: "私信留资量", 78: "线索留资量" },
  13: { 19: "组件点击（点击归因）", 21: "店铺成交（点击归因）", 44: "店铺访问（阅读归因）", 45: "店铺成交（阅读归因）" },
  16: { 0: "点击量", 35: "APP 打开", 36: "APP 进店", 37: "APP 互动", 38: "APP 支付", 39: "APP 支付 ROI", 43: "笔记唤端组件点击" },
  20: { 60: "APP 下载按钮点击", 61: "APP 激活", 62: "APP 注册", 63: "APP 关键行为", 64: "APP 付费", 69: "APP 预约下载按钮点击", 72: "APP 预约下载" },
  21: { 65: "小程序打开", 67: "小程序支付订单数", 68: "小程序 ROI", 73: "微信小游戏打开", 74: "微信小游戏激活", 75: "微信小游戏订单支付数" }
};
const targetTypes: Record<number, string> = { 0: "默认定向", 1: "通投", 2: "智能定向", 3: "高级定向" };
const unitCreationTypes: Record<number, string> = { 0: "标准投放", 1: "简单投放", 2: "留资快投", 3: "R2", 4: "简单投放半自动" };
const materialTypes: Record<number, string> = { 1: "笔记", 2: "H5", 3: "商品", 13: "直播间" };
const conversionTypes: Record<number, string> = { 0: "无组件", 1: "商品组件", 2: "落地页组件", 3: "私信组件", 4: "直播组件", 5: "POI 门店组件", 6: "商品 / 小程序组件", 7: "直播间", 8: "搜索组件", 9: "小程序组件", 10: "留资组件", 11: "唤端组件", 12: "企微组件", 13: "下载组件", 14: "预约组件", 15: "红书小程序组件", 16: "微信小程序组件", 20: "落地页", 23: "直播预热", 30: "商品", 32: "种草直达落地页", 40: "直播", 50: "开屏广告", 78: "私信表单同投组件" };
const auditStatuses: Record<number, string> = { 0: "创建待审核", 1: "审核通过", 2: "审核拒绝", 3: "修改待审核", 7: "审核通过（私密笔记）" };
const creativityAuditStates: Record<number, string> = { 1: "审核拒绝", 2: "审核中", 3: "审核通过", 4: "审核通过（私密）", 99: "不满足审核条件" };
const creativityCreationTypes: Record<number, string> = { 0: "标准投放", 1: "简单投放", 2: "留资快投", 4: "简单投放半自动" };
const unitAvailability: Record<number, string> = { 0: "创意不为空，可正常参与投放", 1: "创意为空，暂不可投放" };
const documentedFields = new Set(SPOTLIGHT_FIELD_DOCS.map((doc) => doc.field));
const compactFields: Record<EntityLevel, Set<string>> = {
  campaign: new Set(["campaign_id", "campaign_name", "campaign_create_time", "campaign_update_time", "start_time", "expire_time"]),
  unit: new Set(["id", "name", "campaign_id", "advertiser_id", "create_time", "update_time"]),
  creativity: new Set(["creativity_id", "creativity_name", "campaign_id", "unit_id", "advertiser_id", "creativity_create_time", "creativity_update_time"])
};

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
  image: "图片", jump_url: "跳转地址", action_button_content: "行动按钮文案", keywords: "关键词", interest_keywords: "兴趣关键词", target_age: "年龄范围", target_city: "投放城市",
  target_device: "设备类型", target_gender: "性别", targetAreaCode: "地域编码", target_city_type: "地域选择方式", target_device_price: "设备价格", intelligent_expansion: "智能扩量",
  premium_target_type: "高级定向类型", searchTargetCityIntent: "搜索地域意图", target_generalization_switch: "定向泛化", reverseConversionType: "排除转化类型", reverseConversionDuration: "排除转化周期",
  haveBrandAIGroup: "品牌智能人群", haveCategoryAIGroup: "品类智能人群", haveBrandInterestGroup: "品牌兴趣人群", haveGoodsInterestGroup: "商品兴趣人群", haveBrandRecognitionGroup: "品牌认知人群",
  haveCategoryInterestGroup: "品类兴趣人群", haveReverseBloggerFanTarget: "排除博主粉丝", haveReverseBloggerPurchasedTarget: "排除博主购买人群"
};

const commonFieldGuides: Record<string, FieldGuide> = {
  campaign_id: { description: "聚光计划的唯一标识，用于检索和关联，不是配置枚举。", configurable: false, numericKind: "identifier" },
  id: { description: "广告单元的唯一标识，用于关联计划与创意。", configurable: false, numericKind: "identifier" },
  unit_id: { description: "广告单元的唯一标识，用于关联计划与创意。", configurable: false, numericKind: "identifier" },
  creativity_id: { description: "投放创意的唯一标识。", configurable: false, numericKind: "identifier" },
  event_asset_id: { description: "承接转化事件的资产 ID；0 表示当前未绑定。", options: { 0: "未绑定事件资产" }, configurable: false },
  asset_event_id: { description: "归因事件 ID；0 表示当前未绑定。", options: { 0: "未绑定归因事件" }, configurable: false },
  deeplink_id: { description: "应用 Deep Link 资产标识；0 表示当前未绑定。", options: { 0: "未绑定 Deep Link" }, configurable: false },
  universal_link_id: { description: "Universal Link 资产标识；0 表示当前未绑定。", options: { 0: "未绑定 Universal Link" }, configurable: false },
  target_template_id: { description: "复用的定向模板 ID；0 表示未使用模板。", options: { 0: "未使用定向模板" }, configurable: false },
  targetTemplateId: { description: "复用的定向模板 ID；0 表示未使用模板。", options: { 0: "未使用定向模板" }, configurable: false },
  campaign_day_budget: { description: "计划每天可消耗的预算上限，聚光原始单位为分。", format: "fen" },
  constraint_value: { description: "出价策略对应的成本控制值，聚光原始单位为分。", format: "fen" },
  event_bid: { description: "广告单元对当前优化事件的出价，聚光原始单位为分。", format: "fen" },
  search_bid_ratio: { description: "搜索场域相对于基准出价的倍率。", format: "ratio" },
  marketing_target: { description: "计划要解决的核心营销任务，会影响可选优化目标。", options: marketingTargets },
  placement: { description: "广告参与流量竞争的投放场域。", options: placements },
  promotion_target: { description: "广告最终推广和承接的对象类型。", options: promotionTargets },
  bidding_strategy: { description: "预算消耗与成本控制所采用的竞价方式。", options: biddingStrategies },
  campaign_enable: { description: "计划层级的人工启停开关。", options: switchOptions },
  enable: { description: "广告单元层级的人工启停开关。", options: switchOptions },
  creativity_enable: { description: "创意层级的人工启停开关。", options: switchOptions },
  limit_day_budget: { description: "是否对计划设置明确的日预算上限。", options: { 0: "不限日预算", 1: "限制日预算" } },
  budget_state: { description: "聚光判断当前预算是否足以继续投放。", options: { 0: "预算不足", 1: "预算充足" }, configurable: false },
  pacing_mode: { description: "预算在投放周期内的消耗速度。", options: { 0: "系统默认", 1: "匀速投放", 2: "加速投放" } },
  time_period_type: { description: "计划在一周内使用全天投放还是自定义时段。", options: { 0: "全天投放", 1: "自定义时段" } },
  feed_flag: { description: "计划是否参与信息流场域投放。", options: switchOptions },
  search_flag: { description: "计划是否参与搜索场域投放；不适用时聚光可能返回 -1。", options: { [-1]: "当前计划不适用", 0: "关闭", 1: "开启" } },
  smart_switch: { description: "是否启用聚光的智能预算调节。", options: switchOptions },
  platform: { description: "该计划的创建与执行平台来源。", options: { 1: "聚光投放平台" }, configurable: false },
  asset_event: { description: "计划绑定的资产事件；0 表示当前未配置。", options: { 0: "未配置资产事件" }, configurable: false },
  page_category: { description: "落地承接页面的分类；0 表示当前未配置。", options: { 0: "未配置页面类别" }, configurable: false },
  marketing_industry: { description: "聚光营销行业分类；0 表示当前未单独设置。", options: { 0: "未设置营销行业" }, configurable: false },
  creativity_state: { description: "计划对创意状态的附加限制；0 表示未设置附加限制。", options: { 0: "无附加创意状态限制" }, configurable: false },
  target_type: { description: "单元选择人群和地域的定向方式。", options: targetTypes },
  creation_type: { description: "该对象在聚光中采用的搭建模式。", options: unitCreationTypes, configurable: false },
  unit_filter_state: { description: "聚光综合开关、预算、排期后计算的单元执行状态。", options: unitStates, configurable: false },
  campaign_filter_state: { description: "聚光综合开关、预算、排期后计算的计划执行状态。", options: campaignStates, configurable: false },
  creativity_filter_state: { description: "聚光综合审核与上级对象状态后计算的创意执行状态。", options: creativityStates, configurable: false },
  material_type: { description: "创意使用的素材载体类型。", options: materialTypes },
  conversion_type: { description: "创意挂载的转化承接组件。", options: conversionTypes },
  audit_status: { description: "创意素材当前的审核结论。", options: auditStatuses, configurable: false },
  creativity_audit_state: { description: "聚光创意审核流程的当前状态。", options: creativityAuditStates, configurable: false },
  custom_mask: { description: "是否使用自定义封面。", options: switchOptions },
  custom_title: { description: "是否使用自定义标题。", options: switchOptions },
  programmatic: { description: "是否启用程序化创意组合。", options: switchOptions },
  intelligent_expansion: { description: "高级定向之外是否允许系统智能扩量。", options: switchOptions },
  target_generalization_switch: { description: "是否开启定向泛化扩量。", options: switchOptions },
  target_city_type: { description: "地域是不限还是按已选城市投放。", options: { 0: "不限地域", 1: "指定地域" } },
  phrase_match_type_upgrade: { description: "搜索关键词是否使用升级后的短语匹配。", options: switchOptions },
  time_period: { description: "一周投放时段编码；由投放日历生成，不建议直接修改码串。", configurable: false },
  target_config: { description: "单元的人群、地域、设备、兴趣和智能扩量配置集合。" },
  keyword_with_bids: { description: "搜索关键词及每个关键词对应的出价配置。" },
  keyword_target_period: { description: "关键词人群行为回溯周期，按天计算。", options: { 0: "未设置回溯周期", 3: "近 3 天", 7: "近 7 天", 15: "近 15 天", 30: "近 30 天" } },
  keyword_gen_type: { description: "搜索关键词的生成方式；未收录的码值保留原样。" },
  constraint_type: { description: "成本控制所约束的转化口径；当前聚光码值保留用于核对。" }
};

function fieldGuide(level: EntityLevel, key: string, payload: Record<string, unknown>, marketingTarget: number): FieldGuide | undefined {
  if (["optimize_target", "optimize_objective", "deep_optimize_objective"].includes(key)) {
    const options = optimizeObjectives[Number(payload.marketing_target) || marketingTarget];
    return { description: key === "deep_optimize_objective" ? "在主要优化目标之后继续衡量的深层转化目标。" : "算法围绕该结果进行出价和流量优化；选项受营销目标限制。", options: options ? { [-1]: "未配置", ...options } : undefined };
  }
  if (key === "not_available_status") return level === "unit"
    ? { description: "单元是否因缺少创意而不可投放，由聚光自动判定。", options: unitAvailability, configurable: false }
    : { description: "对象当前是否存在阻止投放的不可用原因，由聚光自动判定。", options: { 0: "正常可用，无不可用原因" }, configurable: false };
  if (key === "creation_type" && level === "creativity") return { ...commonFieldGuides.creation_type, options: creativityCreationTypes };
  return commonFieldGuides[key];
}

function enumLabel(values: Record<number, string>, value: number): string { return values[value] ?? `官方码值 ${value}`; }
function formatDate(value?: string): string { if (!value) return "-"; const parsed = new Date(value); return Number.isNaN(parsed.getTime()) ? value.slice(0, 19) : dateTime.format(parsed); }
function campaignStateTone(value: number): string { if (value === 1) return "healthy"; return [2, 8, 10].includes(value) ? "paused" : "warning"; }
function unitStateTone(value: number): string { if (value === 10) return "healthy"; return [2, 3, 4, 5, 6, 13].includes(value) ? "paused" : "warning"; }
function creativityStateTone(value: number): string { if (value === 8) return "healthy"; return [3, 4, 5, 10, 11, 12].includes(value) ? "paused" : "warning"; }
function campaignPath(campaign: CampaignSummary): string { return "/delivery/campaigns?" + new URLSearchParams({ advertiser_id: String(campaign.advertiser_id), campaign_id: String(campaign.campaign_id) }).toString(); }
function helperPath(field: string, level: EntityLevel): string { return "/delivery/helper?" + new URLSearchParams({ field, level }).toString(); }
function valueSummary(value: unknown): string { if (Array.isArray(value)) return `${value.length} 项`; if (value && typeof value === "object") return `${Object.keys(value as Record<string, unknown>).length} 个字段`; return ""; }

function PrimitiveValue({ field, value, payload, level, marketingTarget }: { field: string; value: unknown; payload: Record<string, unknown>; level: EntityLevel; marketingTarget: number }) {
  if (value === null || value === undefined || value === "") return <span className="spotlight-empty-value">未配置</span>;
  if (typeof value === "boolean") return <>{value ? "是" : "否"}</>;
  if (typeof value === "string" && /^https?:\/\//.test(value)) return <a href={value} target="_blank" rel="noreferrer">{value}</a>;
  const guide = fieldGuide(level, field, payload, marketingTarget);
  const numericValue = typeof value === "number" ? value : typeof value === "string" && value.length <= 15 && /^-?\d+(\.\d+)?$/.test(value) ? Number(value) : null;
  if (guide?.format === "fen" && numericValue !== null) return <div className="spotlight-interpreted-value"><strong>¥{money.format(numericValue / 100)}</strong><code>原始值 {numericValue} 分</code></div>;
  if (guide?.format === "ratio" && numericValue !== null) return <div className="spotlight-interpreted-value"><strong>{numericValue} 倍</strong><code>原始值 {numericValue}</code></div>;
  if (guide?.options && numericValue !== null) {
    const current = guide.options[numericValue] ?? "未收录码值";
    const options = Object.entries(guide.options);
    return <div className="spotlight-enum-value"><div className="spotlight-interpreted-value"><strong>{current}</strong><code>码值 {numericValue}</code></div><div className="spotlight-option-guide"><span>{guide.configurable === false ? `状态码说明 ${options.length} 项` : `可配置 ${options.length} 项`}</span><div className="spotlight-option-items">{options.map(([code, label]) => <span className={Number(code) === numericValue ? "current" : ""} key={code}><code>{code}</code>{label}</span>)}</div></div></div>;
  }
  if (numericValue !== null && guide?.numericKind === "identifier") return <div className="spotlight-interpreted-value"><strong>{numericValue}</strong><code>唯一标识，不是枚举</code></div>;
  if (numericValue !== null) return <div className="spotlight-unmapped-value"><strong>尚未收录该码值含义</strong><code>原始值 {numericValue}</code></div>;
  return <>{String(value)}</>;
}

function ComplexValue({ value, level, marketingTarget }: { value: unknown; level: EntityLevel; marketingTarget: number }) {
  if (Array.isArray(value)) return <div className="spotlight-json-value"><span>{valueSummary(value)}</span><pre>{JSON.stringify(value, null, 2)}</pre></div>;
  const nestedPayload = value as Record<string, unknown>;
  const entries = Object.entries(nestedPayload).sort(([left], [right]) => (fieldLabels[left] ?? left).localeCompare(fieldLabels[right] ?? right, "zh-CN"));
  return <div className="spotlight-object-value"><span>{valueSummary(value)}</span><dl>{entries.map(([key, nestedValue]) => {
    const nestedComplex = Array.isArray(nestedValue) || (nestedValue !== null && typeof nestedValue === "object");
    const guide = fieldGuide(level, key, nestedPayload, marketingTarget);
    return <div key={key}><dt><div className="spotlight-field-title"><span>{fieldLabels[key] ?? key}</span>{documentedFields.has(key) ? <Link to={helperPath(key, level)} title="在配置助手中查看" aria-label={`查看${fieldLabels[key] ?? key}说明`}><CircleHelp size={12} /></Link> : null}</div><code>{key}</code>{guide?.description ? <small>{guide.description}</small> : null}</dt><dd>{nestedComplex ? <pre>{JSON.stringify(nestedValue, null, 2)}</pre> : <PrimitiveValue field={key} value={nestedValue} payload={nestedPayload} level={level} marketingTarget={marketingTarget} />}</dd></div>;
  })}</dl></div>;
}

function PayloadField({ field, value, payload, level, marketingTarget }: { field: string; value: unknown; payload: Record<string, unknown>; level: EntityLevel; marketingTarget: number }) {
  const complex = Array.isArray(value) || (value !== null && typeof value === "object");
  const guide = fieldGuide(level, field, payload, marketingTarget);
  return <div className={complex ? "complex" : ""}><dt><div className="spotlight-field-title"><span>{fieldLabels[field] ?? field}</span>{documentedFields.has(field) ? <Link to={helperPath(field, level)} title="在配置助手中查看" aria-label={`查看${fieldLabels[field] ?? field}说明`}><CircleHelp size={12} /></Link> : null}</div><code>{field}</code>{guide?.description ? <small>{guide.description}</small> : null}</dt><dd>{complex ? <ComplexValue value={value} level={level} marketingTarget={marketingTarget} /> : <PrimitiveValue field={field} value={value} payload={payload} level={level} marketingTarget={marketingTarget} />}</dd></div>;
}

function RawPayload({ payload, title, level, marketingTarget }: { payload: Record<string, unknown>; title: string; level: EntityLevel; marketingTarget: number }) {
  const entries = Object.entries(payload ?? {}).sort(([left], [right]) => (fieldLabels[left] ?? left).localeCompare(fieldLabels[right] ?? right, "zh-CN"));
  const basics = entries.filter(([key]) => compactFields[level].has(key));
  const configurations = entries.filter(([key]) => !compactFields[level].has(key));
  return <section className="spotlight-raw-payload" aria-label={title}>
    <header><FileJson2 size={15} /><div><strong>{title}</strong><span>{configurations.length} 个决策配置 · {basics.length} 个基础字段 · 候选项已全部展开</span></div><Link className="spotlight-helper-link" to="/delivery/helper"><BookOpenCheck size={13} />配置助手</Link></header>
    {entries.length === 0 ? <div className="spotlight-empty">聚光未返回字段</div> : <><dl className="spotlight-compact-fields" aria-label={`${title}基础信息`}>{basics.map(([key, value]) => <div key={key}><dt>{fieldLabels[key] ?? key}<code>{key}</code></dt><dd><PrimitiveValue field={key} value={value} payload={payload} level={level} marketingTarget={marketingTarget} /></dd></div>)}</dl><dl className="spotlight-field-grid">{configurations.map(([key, value]) => <PayloadField field={key} value={value} payload={payload} level={level} marketingTarget={marketingTarget} key={key} />)}</dl></>}
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
    <RawPayload payload={detail.raw_payload} title="计划全部字段" level="campaign" marketingTarget={campaign.marketing_target} />
    <section className="spotlight-hierarchy-section"><header><div><Rows3 size={17} /><div><h2>广告单元</h2><p>出价、定向、人群、地域、关键词与笔记配置</p></div></div><span>{detail.units.length} 个</span></header>{detail.units.length === 0 ? <div className="spotlight-empty">该计划没有有效广告单元</div> : <div className="spotlight-entity-list">{detail.units.map((unit) => <details className="spotlight-entity" open key={unit.id}><summary><div><strong>{unit.name || "未命名单元"}</strong><code>单元 {unit.id}</code></div><span className={`spotlight-state ${unitStateTone(unit.filter_state)}`}>{enumLabel(unitStates, unit.filter_state)}</span><small>{(creativitiesByUnit.get(unit.id) ?? []).length} 个创意</small></summary><div className="spotlight-entity-meta"><span>开关：{unit.enable === 1 ? "开启" : "关闭"}</span><span>创建：{formatDate(unit.created_at)}</span><span>更新：{formatDate(unit.updated_at)}</span><span>同步：{formatDate(unit.synced_at)}</span></div><RawPayload payload={unit.raw_payload} title={`单元 ${unit.id} 全部字段`} level="unit" marketingTarget={campaign.marketing_target} /></details>)}</div>}</section>
    <section className="spotlight-hierarchy-section"><header><div><Lightbulb size={17} /><div><h2>投放创意</h2><p>素材、笔记、标题、审核、跳转与转化配置</p></div></div><span>{detail.creativities.length} 个</span></header>{detail.creativities.length === 0 ? <div className="spotlight-empty">该计划没有有效投放创意</div> : <div className="spotlight-entity-list creativity">{detail.creativities.map((creativity) => <details className="spotlight-entity" key={creativity.id}><summary><div><strong>{creativity.name || "未命名创意"}</strong><code>创意 {creativity.id} · 单元 {creativity.unit_id || "-"}</code></div><span className={`spotlight-state ${creativityStateTone(creativity.filter_state)}`}>{enumLabel(creativityStates, creativity.filter_state)}</span><small>{String(creativity.raw_payload?.note_id || "无笔记 ID")}</small></summary><div className="spotlight-entity-meta"><span>开关：{creativity.enable === 1 ? "开启" : "关闭"}</span><span>创建：{formatDate(creativity.created_at)}</span><span>更新：{formatDate(creativity.updated_at)}</span><span>同步：{formatDate(creativity.synced_at)}</span></div><RawPayload payload={creativity.raw_payload} title={`创意 ${creativity.id} 全部字段`} level="creativity" marketingTarget={campaign.marketing_target} /></details>)}</div>}</section>
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
