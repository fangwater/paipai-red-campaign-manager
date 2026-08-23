export type SpotlightDocLevel = "campaign" | "unit" | "creativity";
export type SpotlightDocEvidence = "official" | "verified" | "observed";

export type SpotlightOptionDoc = {
  code: number | string;
  label: string;
  meaning: string;
  decision?: string;
};

export type SpotlightFieldDoc = {
  field: string;
  label: string;
  levels: SpotlightDocLevel[];
  configurable: boolean;
  evidence: SpotlightDocEvidence;
  summary: string;
  interpretation: string;
  decision: string;
  applies: string;
  options?: SpotlightOptionDoc[];
  related?: string[];
  source: "campaign-create" | "campaign-list" | "unit-list" | "creativity-list";
  keywords?: string[];
};

export const SPOTLIGHT_DOC_SOURCES = {
  "campaign-create": {
    label: "官方：创建计划",
    url: "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=2722"
  },
  "campaign-list": {
    label: "官方：查询计划",
    url: "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3150"
  },
  "unit-list": {
    label: "官方：获取单元列表",
    url: "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3044"
  },
  "creativity-list": {
    label: "官方：创意查询",
    url: "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3158"
  }
} as const;

const campaignStates: SpotlightOptionDoc[] = [
  { code: 1, label: "有效", meaning: "计划满足投放条件并处于可执行状态。", decision: "继续观察消耗、成本和预算利用率。" },
  { code: 2, label: "暂停", meaning: "计划被人工关闭，或当前计划开关为关闭。", decision: "确认是否为主动暂停，再决定是否恢复。" },
  { code: 3, label: "已删除", meaning: "计划已删除，属于终态。", decision: "不要尝试恢复，需新建计划承接。" },
  { code: 4, label: "计划预算不足", meaning: "计划自身预算条件阻止继续投放。", decision: "检查日预算、已消耗和预算策略。" },
  { code: 5, label: "现金余额不足", meaning: "广告账户现金余额不足。", decision: "充值或调整账户资金后再观察恢复。" },
  { code: 7, label: "账户日预算不足", meaning: "账户层日预算已成为投放约束。", decision: "检查账户层预算，而不是只改计划预算。" },
  { code: 8, label: "暂停阶段", meaning: "计划处于平台暂停处理阶段。", decision: "等待状态收敛并核对最近启停操作。" },
  { code: 10, label: "未投放", meaning: "当前快照未观察到计划进入有效投放。", decision: "检查排期、审核、单元和创意是否完整。" }
];

const creationTypes: SpotlightOptionDoc[] = [
  { code: 0, label: "标准投放", meaning: "按计划、单元、创意三层结构完整搭建。", decision: "适合需要精细控制预算、定向、关键词与素材的常规运营。" },
  { code: 1, label: "简单投放", meaning: "由简化搭建入口生成，平台隐藏或代管部分配置。", decision: "复盘时不要假设所有标准投放字段都由人工明确填写。" },
  { code: 2, label: "留资快投", meaning: "面向线索收集场景的快速搭建来源。", decision: "重点核对表单、私信或留资组件及线索口径。" },
  { code: 3, label: "R2", meaning: "平台特定版本或链路生成的对象类型。", decision: "属于来源标记；修改前应回到原搭建链路核对能力。" },
  { code: 4, label: "简单投放半自动", meaning: "简化入口与部分自动化能力共同生成。", decision: "检查哪些字段由平台托管，避免把自动字段当成人工配置。" }
];

const creativityStates: SpotlightOptionDoc[] = [
  { code: 1, label: "已删除", meaning: "创意已删除，属于终态。", decision: "需要继续投放时重新创建创意。" },
  { code: 2, label: "所有未删除", meaning: "查询接口用于筛选全部未删除创意的聚合状态。", decision: "通常是查询条件，不应当作单条创意的执行结论。" },
  { code: 3, label: "暂停", meaning: "创意自身开关被关闭。", decision: "检查暂停原因与素材表现后决定是否恢复。" },
  { code: 4, label: "被单元暂停", meaning: "创意自身可能正常，但上级单元已暂停。", decision: "处理单元状态，不必逐条修改创意。" },
  { code: 5, label: "被计划暂停", meaning: "创意受上级计划暂停影响。", decision: "优先处理计划层状态。" },
  { code: 6, label: "审核拒绝", meaning: "素材或组件未通过平台审核。", decision: "读取审核原因，修改素材、文案、资质或组件后重提。" },
  { code: 7, label: "审核中", meaning: "创意仍在平台审核流程中。", decision: "等待审核完成，不要重复创建相同创意。" },
  { code: 8, label: "有效", meaning: "创意审核通过且上级对象允许投放。", decision: "可结合曝光、点击、转化判断是否需要优化。" },
  { code: 9, label: "商品状态异常", meaning: "创意关联商品存在不可投状态。", decision: "检查商品上下架、库存、资质和关联关系。" },
  { code: 10, label: "单元未开始", meaning: "上级单元尚未进入开始时间。", decision: "核对单元排期，无需修改创意。" },
  { code: 11, label: "单元已结束", meaning: "上级单元已过结束时间。", decision: "需要继续投放时调整排期或新建单元。" },
  { code: 12, label: "单元暂停时段", meaning: "当前时间不在单元设置的投放时段内。", decision: "核对分时投放设置。" },
  { code: 13, label: "计划预算不足", meaning: "创意受上级计划预算限制。", decision: "从计划预算与消耗侧处理。" },
  { code: 14, label: "现金余额不足", meaning: "创意受账户现金余额限制。", decision: "从账户资金侧处理。" },
  { code: 16, label: "账户日预算不足", meaning: "创意受账户层日预算限制。", decision: "检查账户日预算和当日累计消耗。" }
];

export const SPOTLIGHT_FIELD_DOCS: SpotlightFieldDoc[] = [
  {
    field: "bidding_strategy", label: "出价策略", levels: ["campaign"], configurable: true, evidence: "official",
    summary: "决定系统如何在消耗规模、转化量和成本稳定性之间取舍。",
    interpretation: "出价策略不是单纯的价格字段。手动出价把控制权留给投手；最大转化优先在预算内争取更多结果；稳定成本则围绕成本约束寻找量级。",
    decision: "先明确当前阶段更看重放量还是成本稳定，再选择策略。策略切换会影响学习过程，不宜仅因短期波动频繁切换。",
    applies: "具体可选项受营销目标、优化目标、账户白名单和平台能力影响。",
    options: [
      { code: 2, label: "手动出价", meaning: "由投手设置优化事件出价，系统据此参与竞价。", decision: "控制最直接，适合有历史成本基准或需要严格测试出价梯度的场景。" },
      { code: 3, label: "最大转化", meaning: "系统在预算与投放条件内优先争取更多优化结果。", decision: "适合放量；短期成本可能波动，应结合预算消耗和学习阶段判断。" },
      { code: 7, label: "稳定成本", meaning: "系统围绕成本约束兼顾转化量和成本稳定。", decision: "适合已有可接受成本目标的成熟场景；过紧约束可能限制跑量。" }
    ], related: ["constraint_type", "constraint_value", "event_bid", "optimize_objective", "campaign_day_budget"], source: "campaign-create", keywords: ["手动出价", "最大转化", "稳定成本", "成本控制"]
  },
  {
    field: "creation_type", label: "创建类型", levels: ["campaign", "unit", "creativity"], configurable: false, evidence: "verified",
    summary: "记录对象从哪种聚光搭建链路产生，是来源属性，不是普通启停配置。",
    interpretation: "相同码值在计划、单元和创意层表达同一类搭建来源。创意查询已收录 0、1、2、4；单元查询还可能返回 3。",
    decision: "它影响你如何理解字段完整度：简化或半自动搭建可能由平台代管部分字段。通常不应在详情页直接修改创建类型。",
    applies: "用于查询、归因和兼容不同搭建链路；实际可见码值以对象层级和账户能力为准。",
    options: creationTypes, related: ["build_type", "programmatic"], source: "unit-list", keywords: ["标准投", "简单投", "留资快投", "R2", "半自动"]
  },
  {
    field: "creativity_filter_state", label: "创意执行状态", levels: ["creativity"], configurable: false, evidence: "official",
    summary: "聚光综合创意开关、审核、商品、上级计划/单元、预算和排期计算出的最终状态。",
    interpretation: "它不是单一开关。看到暂停或不可投时，应按状态来源处理对应层级，而不是直接重建创意。",
    decision: "先区分自身状态、审核状态、上级状态和资金状态，再采取动作。只有码值 3 通常直接指向创意自身暂停。",
    applies: "查询创意快照与诊断投放阻塞时使用；码值 2 主要是查询筛选语义。",
    options: creativityStates, related: ["creativity_enable", "audit_status", "creativity_audit_state", "unit_filter_state", "campaign_filter_state"], source: "creativity-list", keywords: ["创意状态", "审核拒绝", "审核中", "有效", "预算不足"]
  },
  {
    field: "creativity_state", label: "计划创意状态限制", levels: ["campaign"], configurable: false, evidence: "observed",
    summary: "计划快照中的附加创意状态字段，与单条创意的 creativity_filter_state 不同。",
    interpretation: "当前真实快照主要观察到 0，表示未设置附加创意状态限制。不能用它替代单条创意执行状态。",
    decision: "判断创意能否投放时，以 creativity_filter_state、审核状态和上级状态为主。",
    applies: "用于保留计划原始响应和字段漂移观察。",
    options: [{ code: 0, label: "无附加限制", meaning: "当前计划未设置额外创意状态限制。" }], related: ["creativity_filter_state"], source: "campaign-list"
  },
  {
    field: "campaign_filter_state", label: "计划执行状态", levels: ["campaign"], configurable: false, evidence: "official",
    summary: "聚光计算出的计划最终执行状态，不等同于 campaign_enable。",
    interpretation: "开关开启并不保证有效；预算、余额、排期等都可能使计划不可投。",
    decision: "按状态定位阻塞来源，不要只反复切换计划开关。",
    applies: "计划查询与投放诊断。", options: campaignStates, related: ["campaign_enable", "budget_state"], source: "campaign-list"
  },
  {
    field: "campaign_enable", label: "计划开关", levels: ["campaign"], configurable: true, evidence: "official",
    summary: "人工控制计划是否允许执行。",
    interpretation: "开关只表达人工意图；最终能否投放仍由 campaign_filter_state 决定。",
    decision: "排障时同时查看开关和执行状态。",
    applies: "创建计划、修改计划状态。", options: [{ code: 0, label: "关闭", meaning: "计划暂停。" }, { code: 1, label: "开启", meaning: "允许计划在其他条件满足时投放。" }], related: ["campaign_filter_state"], source: "campaign-create"
  },
  {
    field: "marketing_target", label: "营销目标", levels: ["campaign"], configurable: true, evidence: "official",
    summary: "定义计划要解决的业务问题，并约束优化目标和可用组件。",
    interpretation: "营销目标是计划结构的上游选择，不能脱离优化目标、场域和投放标的单独判断。",
    decision: "以最终业务结果选择目标，不要仅按历史流量规模选择。",
    applies: "不同账户与行业可能只开放部分目标。",
    options: [[3, "商品销量"], [4, "产品种草"], [8, "直播推广"], [9, "客资收集"], [10, "抢占关键词"], [13, "种草直达"], [14, "直播预热"], [15, "店铺拉新"], [16, "应用唤起"], [20, "应用下载"], [21, "小程序推广"]].map(([code, label]) => ({ code, label: String(label), meaning: `计划以${label}为核心业务目标。` })),
    related: ["optimize_objective", "placement", "promotion_target"], source: "campaign-create"
  },
  {
    field: "placement", label: "投放场域", levels: ["campaign", "unit"], configurable: true, evidence: "official",
    summary: "决定计划参与哪类流量竞争。",
    interpretation: "场域决定流量意图、竞价环境与素材适配方式。",
    decision: "信息流侧重兴趣触达，搜索侧重主动意图，全站智投由系统跨流量分配。",
    applies: "具体场域需与营销目标和账户能力兼容。",
    options: [
      { code: 1, label: "信息流", meaning: "在浏览信息流中触达潜在人群。" },
      { code: 2, label: "搜索推广", meaning: "承接用户主动搜索意图。" },
      { code: 4, label: "全站智投", meaning: "由平台跨可用流量进行智能分配。" },
      { code: 7, label: "视频内流", meaning: "在视频内容消费链路中投放。" }
    ], related: ["feed_flag", "search_flag", "search_bid_ratio"], source: "campaign-create"
  },
  {
    field: "target_type", label: "定向类型", levels: ["unit"], configurable: true, evidence: "official",
    summary: "定义单元使用通投、智能还是高级定向。",
    interpretation: "定向越细不一定越好；过窄会限制模型学习和跑量。",
    decision: "新计划优先保证足够覆盖，再通过单变量实验收窄。",
    applies: "可选项与营销目标、场域及账户定向能力有关。",
    options: [{ code: 0, label: "默认定向", meaning: "使用当前链路默认定向。" }, { code: 1, label: "通投", meaning: "不增加细分人群限制。" }, { code: 2, label: "智能定向", meaning: "由平台模型选择潜在人群。" }, { code: 3, label: "高级定向", meaning: "人工配置年龄、地域、兴趣等条件。" }], related: ["target_config"], source: "unit-list"
  },
  {
    field: "not_available_status", label: "不可用状态", levels: ["campaign", "unit", "creativity"], configurable: false, evidence: "verified",
    summary: "平台对对象是否存在不可投原因的辅助判断。",
    interpretation: "单元层文档字典区分创意是否为空；计划与创意层的 0 在当前快照中表示无不可用原因。",
    decision: "非 0 时继续查看执行状态、审核与上级对象，不能只凭此字段推断全部原因。",
    applies: "投放阻塞排查。", options: [{ code: 0, label: "正常可用", meaning: "未观察到不可用原因；单元层表示创意不为空。" }, { code: 1, label: "创意为空", meaning: "单元没有可用创意，暂不能投放。" }], related: ["unit_filter_state", "creativity_filter_state"], source: "unit-list"
  },
  {
    field: "audit_status", label: "素材审核状态", levels: ["creativity"], configurable: false, evidence: "official",
    summary: "记录创意素材提交后的审核结论。",
    interpretation: "它描述审核流程，不等同于最终投放状态。",
    decision: "拒绝时必须结合审核原因处理素材、资质或组件。",
    applies: "创意查询。", options: [{ code: 0, label: "创建待审核", meaning: "首次提交等待审核。" }, { code: 1, label: "审核通过", meaning: "素材审核通过。" }, { code: 2, label: "审核拒绝", meaning: "素材未通过审核。" }, { code: 3, label: "修改待审核", meaning: "修改后再次等待审核。" }, { code: 7, label: "审核通过（私密笔记）", meaning: "私密笔记场景下审核通过。" }], related: ["creativity_audit_state", "creativity_filter_state"], source: "creativity-list"
  },
  {
    field: "material_type", label: "素材类型", levels: ["creativity"], configurable: true, evidence: "official",
    summary: "创意使用的内容载体。",
    interpretation: "素材类型会影响可用组件、审核要求和用户承接链路。",
    decision: "根据推广标的和承接链路选择，不要只按素材存量选择。",
    applies: "具体类型受营销目标限制。", options: [{ code: 1, label: "笔记", meaning: "以小红书笔记作为素材。" }, { code: 2, label: "H5", meaning: "以 H5 页面承接。" }, { code: 3, label: "商品", meaning: "以商品资产作为素材或标的。" }, { code: 13, label: "直播间", meaning: "以直播间作为素材或承接。" }], related: ["conversion_type", "promotion_target"], source: "creativity-list"
  },
  {
    field: "conversion_type", label: "转化组件类型", levels: ["creativity"], configurable: true, evidence: "official",
    summary: "决定用户点击创意后使用哪类组件完成下一步动作。",
    interpretation: "组件必须与营销目标、素材类型、落地资产和资质匹配。",
    decision: "优先选择最接近最终业务结果且可稳定归因的承接方式。",
    applies: "账户未必开放全部组件。",
    options: [[0, "无组件"], [1, "商品组件"], [2, "落地页组件"], [3, "私信组件"], [4, "直播组件"], [5, "POI 门店组件"], [6, "商品 / 小程序组件"], [7, "直播间"], [8, "搜索组件"], [9, "小程序组件"], [10, "留资组件"], [11, "唤端组件"], [12, "企微组件"], [13, "下载组件"], [14, "预约组件"], [15, "红书小程序组件"], [16, "微信小程序组件"], [20, "落地页"], [23, "直播预热"], [30, "商品"], [32, "种草直达落地页"], [40, "直播"], [50, "开屏广告"], [78, "私信表单同投组件"]].map(([code, label]) => ({ code, label: String(label), meaning: `使用${label}承接用户转化。` })),
    related: ["material_type", "jump_url"], source: "creativity-list"
  },
  {
    field: "pacing_mode", label: "投放速度", levels: ["campaign"], configurable: true, evidence: "official",
    summary: "控制预算在投放周期内的消耗节奏。",
    interpretation: "匀速偏向平滑覆盖，加速偏向尽快争取流量。",
    decision: "大促或短周期可考虑加速；常规持续投放更关注匀速和全天稳定性。",
    applies: "实际选项可能受出价策略和场域影响。", options: [{ code: 0, label: "系统默认", meaning: "使用当前链路默认节奏。" }, { code: 1, label: "匀速投放", meaning: "尽量在投放时段内平滑消耗。" }, { code: 2, label: "加速投放", meaning: "更积极地争取可用流量。" }], related: ["campaign_day_budget", "time_period"], source: "campaign-create"
  },
  {
    field: "campaign_day_budget", label: "计划日预算", levels: ["campaign"], configurable: true, evidence: "official",
    summary: "计划每天允许消耗的预算上限，接口原始单位为分。",
    interpretation: "预算决定可争取的量级，但不直接保证消耗或效果。",
    decision: "结合历史消耗、目标成本、学习需求和账户总预算设置。",
    applies: "创建接口有最低预算等校验，具体阈值以账户和官方页面为准。", related: ["limit_day_budget", "budget_state", "pacing_mode"], source: "campaign-create"
  },
  {
    field: "time_period_type", label: "投放时段类型", levels: ["campaign"], configurable: true, evidence: "official",
    summary: "决定全天投放还是使用自定义分时表。",
    interpretation: "自定义时段会直接限制可参与竞价的时间窗口。",
    decision: "只有明确的营业、直播或转化时段证据时再收窄，避免损失模型学习样本。",
    applies: "自定义时需同时核对 time_period 码表。", options: [{ code: 0, label: "全天投放", meaning: "在有效日期内不额外限制小时。" }, { code: 1, label: "自定义时段", meaning: "仅在分时表允许的小时投放。" }], related: ["time_period", "start_time", "expire_time"], source: "campaign-create"
  }
];

export const SPOTLIGHT_LEVEL_LABELS: Record<SpotlightDocLevel, string> = {
  campaign: "计划",
  unit: "单元",
  creativity: "创意"
};

export const SPOTLIGHT_EVIDENCE_LABELS: Record<SpotlightDocEvidence, string> = {
  official: "官方文档枚举",
  verified: "项目已验证",
  observed: "真实快照观察"
};
