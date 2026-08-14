import { useState } from "react";
import {
  AlertTriangle, ArrowRight, BadgeCheck, Ban, BarChart3, Bot, BrainCircuit,
  Braces, Check, CheckCircle2, ChevronRight, CircleDollarSign, ClipboardCheck, Clock3,
  Code2, Database, ExternalLink, FileCheck2, FileText, Filter, Flag, Gauge, GitBranch,
  KeyRound, Layers3, Lightbulb, ListChecks, LockKeyhole, Network, PauseCircle,
  Play, RefreshCw, Route, Search, ServerCog, ShieldCheck, SlidersHorizontal, Sparkles,
  Target, TestTube2, UserCheck, Users, WandSparkles, Workflow, XCircle, type LucideIcon
} from "lucide-react";
import "./self-serve-delivery-plan.css";

type ViewKey = "decision" | "workflow" | "api" | "intelligence" | "architecture" | "rollout";
type StatusTone = "ready" | "conditional" | "planned" | "blocked";

type View = {
  key: ViewKey;
  label: string;
  icon: LucideIcon;
};

type APIRow = {
  domain: string;
  internal: string;
  upstream: string;
  method: "GET" | "POST";
  purpose: string;
  status: StatusTone;
  evidence: string;
};

const OFFICIAL_HOME = "https://ad-market.xiaohongshu.com/";
const CAMPAIGN_CREATE_DOC = "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=2722";
const CAMPAIGN_LIST_DOC = "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3150";
const UNIT_LIST_DOC = "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3044";
const CREATIVITY_LIST_DOC = "https://ad-market.xiaohongshu.com/docs-center?bizType=943&articleId=3158";
const SDK_EVIDENCE = "https://github.com/jundaychan/spotlight-mapi";

const VIEWS: View[] = [
  { key: "decision", label: "可行性结论", icon: BadgeCheck },
  { key: "workflow", label: "完整工作流", icon: Workflow },
  { key: "api", label: "API 目标", icon: Braces },
  { key: "intelligence", label: "AI 与算法", icon: BrainCircuit },
  { key: "architecture", label: "系统架构", icon: Network },
  { key: "rollout", label: "实施路线", icon: Flag }
];

const STATUS: Record<StatusTone, { label: string; note: string }> = {
  ready: { label: "中台已实现", note: "内部契约、鉴权和持久化已经可用" },
  conditional: { label: "待上游验收", note: "中台适配已实现，仍需真实账号契约冒烟" },
  planned: { label: "后续阶段", note: "不属于本轮后端基础能力" },
  blocked: { label: "禁止直连", note: "没有护栏前不得自动执行" }
};

const API_ROWS: APIRow[] = [
  {
    domain: "权限探测", internal: "/v1/delivery/capabilities", upstream: "OAuth 授权与操作目录",
    method: "GET", purpose: "返回授权状态、广告主范围、读写 scope、操作目录、写开关与上游契约版本",
    status: "ready", evidence: "余额和白名单不做静态承诺，由 validate 在每次审批或发布前实时复核"
  },
  {
    domain: "本地资产", internal: "/v1/delivery/assets", upstream: "PostgreSQL 稿件与历史表现",
    method: "GET", purpose: "按广告主检索本地稿件候选、内容标签与历史效果特征",
    status: "ready", evidence: "已实现广告主隔离、搜索、分页和推荐候选数据源"
  },
  {
    domain: "平台资产", internal: "/v1/delivery/assets/platform", upstream: "/api/open/jg/note/list · spu/list · data/qual/info · event/list",
    method: "POST", purpose: "查询可投笔记、SPU、资质和事件资产，保留原始平台响应",
    status: "conditional", evidence: "只读适配和审计已实现；字段口径仍需登录态官方文档及真实响应确认"
  },
  {
    domain: "定向字典", internal: "/v1/delivery/target-options", upstream: "/api/open/jg/target/get_available_target_info",
    method: "POST", purpose: "按营销目标返回年龄、性别、地域、兴趣、人群包等平台允许值",
    status: "conditional", evidence: "候选值必须来自广告主实时可选项，不能由 LLM 编造"
  },
  {
    domain: "关键词工具", internal: "/v1/delivery/keyword-candidates", upstream: "/api/open/jg/keyword/common/recommend · keyword/word/bag/list",
    method: "POST", purpose: "以稿件卖点和种子词获取平台词，再做意图聚类、去重、否词与出价建议",
    status: "conditional", evidence: "公开 SDK 有工具接口映射；白名单与限流待实测"
  },
  {
    domain: "人群预估", internal: "/v1/delivery/audience-estimates", upstream: "/api/open/jg/crowd/estimate",
    method: "POST", purpose: "预估年龄、地域、兴趣组合覆盖，拦截过窄或不可投定向",
    status: "conditional", evidence: "公开 SDK 有接口映射；返回口径与最小人群阈值待确认"
  },
  {
    domain: "方案草稿", internal: "/v1/delivery/drafts", upstream: "无",
    method: "POST", purpose: "创建版本化草稿，固定业务目标、预算上限、笔记候选与实验假设",
    status: "ready", evidence: "已实现规范化、版本、哈希、乐观锁、幂等和广告主级隔离"
  },
  {
    domain: "AI 建议", internal: "/v1/delivery/drafts/{id}/recommendations", upstream: "仅读取工具接口",
    method: "POST", purpose: "生成稿件排序、关键词簇、年龄/地域实验单元及逐项证据",
    status: "ready", evidence: "已实现严格 JSON 输出、确定性降级和独立排序器；结果不可执行"
  },
  {
    domain: "确定性校验", internal: "/v1/delivery/drafts/{id}/validate", upstream: "名称校验 · 余额 · 人群预估",
    method: "POST", purpose: "验证字段组合、预算、资质、笔记状态、重名、余额和当前 scope",
    status: "ready", evidence: "已实现本地规则、能力、余额、白名单、重名、笔记和人群预估校验"
  },
  {
    domain: "审批", internal: "/v1/delivery/drafts/{id}/approve", upstream: "无",
    method: "POST", purpose: "记录运营与预算责任人双人确认、差异快照、预算额度和过期时间",
    status: "ready", evidence: "已实现运营与预算责任人双人审批、追加历史、过期和版本失效"
  },
  {
    domain: "创建计划", internal: "/v1/delivery/drafts/{id}/publish", upstream: "/api/open/jg/campaign/create",
    method: "POST", purpose: "先以暂停态创建计划，写入目标、场域、时间、预算及出价策略",
    status: "conditional", evidence: "官方创建计划文档入口与 ad_manage scope 已确认；创建端点尚未做暂停态冒烟"
  },
  {
    domain: "创建单元", internal: "由 publish 编排", upstream: "/api/open/jg/unit/create",
    method: "POST", purpose: "绑定笔记/SPU，提交出价、年龄、性别、地域、兴趣、行为词和搜索词",
    status: "conditional", evidence: "平台投放管理支持增删改查；具体字段以官方文档验收为准"
  },
  {
    domain: "创建创意", internal: "由 publish 编排", upstream: "/api/open/jg/creativity/create",
    method: "POST", purpose: "绑定最终笔记、转化组件、置顶文案、监测链接和资质",
    status: "conditional", evidence: "公开 SDK 可佐证端点；资质组合和审核规则必须实测"
  },
  {
    domain: "读回核对", internal: "/v1/delivery/jobs/{id}", upstream: "计划/单元/创意查询接口",
    method: "GET", purpose: "逐层查询新 ID，比较上游实际值与审批快照，异常时保持暂停",
    status: "conditional", evidence: "异步作业、三层 ID/父子关系/暂停状态/关键字段 diff 已实现；待真实写入读回验收"
  },
  {
    domain: "启停控制", internal: "/v1/delivery/entities/{type}/{id}/status", upstream: "三层 status update",
    method: "POST", purpose: "人工启停、熔断和审批后批量动作；全部动作写审计日志",
    status: "conditional", evidence: "平台有修改状态能力；默认不开放给模型"
  },
  {
    domain: "效果回传", internal: "/v1/delivery/performance", upstream: "/api/open/jg/data/report/realtime/* · offline/*",
    method: "GET", purpose: "拉取账户、计划、单元、创意、关键词报表并关联站外、蒲公英和薯量结果",
    status: "conditional", evidence: "账户、计划、单元、创意、关键词实时/离线适配及快照已实现；待真实口径验收"
  }
];

const DRAFT_EXAMPLE = `{
  "spec": {
    "advertiser_id": 123456,
    "objective": "offsite_activation",
    "placement": "search",
    "budget": { "daily_limit_fen": 30000, "total_limit_fen": 210000, "max_bid_fen": 5000 },
    "notes": ["0123456789abcdef01234567"],
    "campaign": {
      "local_key": "campaign", "name": "辅酶搜索测试", "marketing_target": 4,
      "placement": 2, "promotion_target": 1, "enable": 0, "time_type": 0,
      "time_period_type": 0, "bidding_strategy": 2, "limit_day_budget": 1,
      "day_budget_fen": 30000, "optimize_target": 1
    },
    "units": [{
      "local_key": "unit-1", "name": "23-27 岁测试", "event_bid_fen": 1000,
      "note_ids": ["0123456789abcdef01234567"], "promotion_target": 1,
      "target_type": 3, "target": { "age": "23-27", "gender": "all", "device": "all", "cities": "all" },
      "keywords": [{ "keyword": "辅酶q10", "bid_fen": 800, "phrase_match_type": 0 }],
      "creativities": [{ "local_key": "creative-1", "name": "辅酶创意", "note_id": "0123456789abcdef01234567" }]
    }],
    "experiment": { "primary_metric": "offsite_15d_cost", "variables": ["age_segment"] }
  },
  "idempotency_key": "advertiser:objective:2026w34:v1"
}`;

const RECOMMENDATION_EXAMPLE = `{
  "schema_version": "delivery-recommendation/v1",
  "ranked_notes": [{ "note_id": "note_id_1", "score": 0.78, "evidence": ["稿件场景匹配", "历史站外成本样本"] }],
  "keyword_clusters": [{ "intent": "功效认知", "positive": ["辅酶q10作用"], "negative": ["批发"] }],
  "audience_tests": [{ "age": "23-27", "hypothesis": "职场熬夜场景匹配", "estimated_size": 182000 }],
  "uncertainties": ["该年龄段有效转化样本不足 100"],
  "requires_human_review": true
}`;

function StatusPill({ tone }: { tone: StatusTone }) {
  return <span className={`delivery-status-pill ${tone}`}>{STATUS[tone].label}</span>;
}

function SourceLink({ href, children }: { href: string; children: string }) {
  return <a className="delivery-source-link" href={href} target="_blank" rel="noreferrer">
    {children}<ExternalLink size={13} />
  </a>;
}

function SectionHeading({ index, title, description }: { index: string; title: string; description: string }) {
  return <header className="delivery-section-heading">
    <span>{index}</span>
    <div><h2>{title}</h2><p>{description}</p></div>
  </header>;
}

function DecisionView() {
  return <div className="delivery-view">
    <section className="delivery-decision-band">
      <div className="decision-score"><strong>条件可行</strong><span>GO / NO-GO</span></div>
      <div className="decision-copy">
        <h2>可以做，但不能从“推荐”直接跳到“花钱”</h2>
        <p>小红书开放平台公开说明 Marketing API 支持推广计划、单元、创意的批量增删改查；当前中台已具备 OAuth、版本化草稿、双人审批、三层暂停态发布、读回校验、报表和算法建议接口。生产 Token 已包含 ad_manage、ad_query、report_service、account_manage；真实媒体写入仍由全局开关关闭，等待专用广告主冒烟。</p>
      </div>
      <div className="decision-evidence">
        <SourceLink href={OFFICIAL_HOME}>官方能力说明</SourceLink>
        <SourceLink href={CAMPAIGN_CREATE_DOC}>创建计划文档</SourceLink>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="01" title="当前事实与能力边界" description="把已运行的能力、平台能力和未验证授权分开" />
      <div className="readiness-grid">
        <article className="readiness-column ready">
          <header><CheckCircle2 size={18} /><div><strong>已具备</strong><span>可以直接复用</span></div></header>
          <ul>
            <li>OAuth 授权、Token 刷新与本机凭据托管</li>
            <li>有效 Token 已含 ad_manage / ad_query / report_service / account_manage</li>
            <li>当前授权广告主 59 个，可按广告主做能力与数据隔离</li>
            <li>计划、单元、创意查询及全量/增量同步</li>
            <li>笔记、稿件、Maituo、蒲公英、薯量关联数据</li>
            <li>草稿、建议、校验、双审批、异步 Saga、读回与审计后端</li>
          </ul>
        </article>
        <article className="readiness-column conditional">
          <header><Clock3 size={18} /><div><strong>需验收</strong><span>通过后才进入写开发</span></div></header>
          <ul>
            <li>ad_manage 下的 create、update、status 端点是否逐项放行</li>
            <li>目标广告主是否在白名单且余额、资质、SPU、笔记可投</li>
            <li>创建接口最新字段、枚举、频控与错误码的真实响应</li>
            <li>关键词、人群预估、报表和转化回传权限是否一并开通</li>
          </ul>
        </article>
        <article className="readiness-column blocked">
          <header><Ban size={18} /><div><strong>首版禁止</strong><span>不交给模型执行</span></div></header>
          <ul>
            <li>LLM 直接调用媒体写接口或持有 Access Token</li>
            <li>未经审批自动扩预算、提高出价或开启计划</li>
            <li>用自然互动代替付费增量、站外成本或成交判断</li>
            <li>跨广告主共享训练明细、提示词原文或敏感人群数据</li>
          </ul>
        </article>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="02" title="上线前的五个硬闸门" description="任一项不满足，系统只能生成草稿，不能发布" />
      <div className="gate-list">
        {[
          [KeyRound, "权限闸门", "用真实广告主逐项探测 create / update / status / report scope，并保存带时间的能力快照。"],
          [FileCheck2, "契约闸门", "从登录后的官方文档固化 JSON Schema、枚举和错误码，建立上游契约测试。"],
          [CircleDollarSign, "资金闸门", "账户余额、单计划日限额、广告主日总额、草稿总额和异常熔断必须同时生效。"],
          [UserCheck, "审批闸门", "运营确认投放结构，预算责任人确认金额；审批后发生任何资金字段变化都要重新审批。"],
          [BarChart3, "测量闸门", "先接实时/离线报表与业务转化，定义可追溯指标，才允许进入算法调优。"]
        ].map(([Icon, title, detail], index) => {
          const GateIcon = Icon as LucideIcon;
          return <article key={String(title)}><span>{index + 1}</span><GateIcon size={19} /><div><strong>{String(title)}</strong><p>{String(detail)}</p></div></article>;
        })}
      </div>
    </section>

    <section className="delivery-section evidence-section">
      <SectionHeading index="03" title="证据等级" description="规划不等于生产授权，验收时按从强到弱取证" />
      <div className="evidence-levels">
        <div><b>A</b><strong>真实账号响应</strong><span>最高证据：白名单、权限、创建后读回和报表数据</span></div>
        <div><b>B</b><strong>官方登录态文档</strong><span>字段、枚举、约束、频控、错误码与更新日期</span></div>
        <div><b>C</b><strong>官方公开页面</strong><span>证明产品方向与能力存在，不证明当前账号有权限</span></div>
        <div><b>D</b><strong>公开 SDK 映射</strong><span>只用于发现端点和字段候选，不作为最终生产契约</span></div>
      </div>
      <div className="evidence-links">
        <SourceLink href={CAMPAIGN_LIST_DOC}>官方查询计划</SourceLink>
        <SourceLink href={UNIT_LIST_DOC}>官方单元列表</SourceLink>
        <SourceLink href={CREATIVITY_LIST_DOC}>官方创意查询</SourceLink>
        <SourceLink href={SDK_EVIDENCE}>公开 SDK 辅助证据</SourceLink>
      </div>
    </section>
  </div>;
}

const FLOW_STEPS: Array<{ icon: LucideIcon; title: string; owner: string; detail: string; output: string }> = [
  { icon: Target, title: "定义目标", owner: "运营", detail: "广告主、营销目标、场域、总预算、主指标、保护指标与实验变量", output: "投放 brief v1" },
  { icon: Database, title: "汇总候选", owner: "系统", detail: "同步可投笔记、稿件内容、历史投放、关键词工具、定向字典和资质", output: "候选资产快照" },
  { icon: Sparkles, title: "生成建议", owner: "AI + ML", detail: "排序稿件，聚类关键词，提出年龄/地域实验；每项附证据和不确定性", output: "结构化建议 v1" },
  { icon: SlidersHorizontal, title: "编辑方案", owner: "运营", detail: "选择计划、单元、创意组合，只允许平台实时可选字段", output: "版本化草稿" },
  { icon: ShieldCheck, title: "规则校验", owner: "系统", detail: "权限、余额、预算、资质、重名、人群规模、笔记状态和字段组合校验", output: "校验报告" },
  { icon: Users, title: "双人审批", owner: "运营 + 财务", detail: "对比建议与最终值，确认预算上限、风险和启用方式", output: "不可变审批快照" },
  { icon: ServerCog, title: "暂停态创建", owner: "编排器", detail: "按计划 → 单元 → 创意执行 Saga，逐层保存上游 ID 和原始响应", output: "媒体对象树" },
  { icon: RefreshCw, title: "读回核对", owner: "系统", detail: "查询三层对象并与审批快照做字段 diff；不一致立即停止", output: "发布验收单" },
  { icon: Play, title: "人工启用", owner: "投手", detail: "首版只允许授权投手启用；自动启停需另行灰度和审批", output: "运行中实验" },
  { icon: Gauge, title: "监控与复盘", owner: "系统 + 运营", detail: "小时级成本熔断、日级调优建议、7/15 天效果归因与实验结论", output: "特征与标签" }
];

function WorkflowView() {
  return <div className="delivery-view">
    <section className="delivery-section first">
      <SectionHeading index="01" title="从需求到投放的状态机" description="每个状态都可审计、可超时、可取消；只有审批后的不可变版本可发布" />
      <div className="state-machine" role="img" aria-label="自建投流状态机">
        {[
          ["DRAFT", "草稿"], ["VALIDATED", "已校验"], ["PENDING_APPROVAL", "待审批"],
          ["APPROVED", "已批准"], ["PUBLISHING", "创建中"], ["PAUSED", "待启用"], ["ACTIVE", "投放中"]
        ].map(([code, label], index, items) => <div className="state-node-wrap" key={code}>
          <div className={`state-node state-${index}`}><code>{code}</code><strong>{label}</strong></div>
          {index < items.length - 1 ? <ChevronRight size={18} /> : null}
        </div>)}
      </div>
      <div className="state-exits">
        <span><XCircle size={15} />校验失败 → DRAFT</span>
        <span><Ban size={15} />审批拒绝 → REJECTED</span>
        <span><PauseCircle size={15} />部分创建失败 → FAILED_PAUSED</span>
        <span><AlertTriangle size={15} />命中熔断 → PAUSED_BY_GUARDRAIL</span>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="02" title="端到端操作流" description="LLM 负责候选和解释，编排器负责状态，规则引擎决定能否继续" />
      <ol className="workflow-timeline">
        {FLOW_STEPS.map((step, index) => <li key={step.title}>
          <span className="workflow-number">{String(index + 1).padStart(2, "0")}</span>
          <div className="workflow-icon"><step.icon size={19} /></div>
          <div className="workflow-main"><header><h3>{step.title}</h3><span>{step.owner}</span></header><p>{step.detail}</p></div>
          <div className="workflow-output"><small>输出</small><strong>{step.output}</strong></div>
        </li>)}
      </ol>
    </section>

    <section className="delivery-section">
      <SectionHeading index="03" title="发布编排与失败处理" description="媒体 API 不是数据库事务，必须按 Saga 管理部分成功" />
      <div className="saga-flow">
        <div><span>1</span><MegaphoneIcon /><strong>创建计划</strong><small>enable=0</small></div><ArrowRight size={18} />
        <div><span>2</span><Layers3 size={18} /><strong>创建单元</strong><small>逐个落 ID</small></div><ArrowRight size={18} />
        <div><span>3</span><Lightbulb size={18} /><strong>创建创意</strong><small>逐个验资质</small></div><ArrowRight size={18} />
        <div><span>4</span><Search size={18} /><strong>三层读回</strong><small>字段 diff=0</small></div>
      </div>
      <div className="failure-grid">
        <article><strong>可重试错误</strong><p>超时、429、5xx：指数退避并复用同一幂等键；重试前先按内部映射和名称查询，避免重复创建。</p></article>
        <article><strong>业务错误</strong><p>资质、字段、笔记不可投：停止后续步骤，计划保持暂停，返回精确字段和官方 request_id。</p></article>
        <article><strong>部分成功</strong><p>不盲目删除已创建对象。保存对象树，默认全部暂停，由人工选择补齐或执行显式清理。</p></article>
        <article><strong>状态不一致</strong><p>读回值与审批快照不同：标记 drift，禁止启用，并保留请求、响应、操作者与 diff。</p></article>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="04" title="首版运营表单" description="表单按计划、单元、创意三层展开，动态字段由平台字典驱动" />
      <div className="form-blueprint">
        <article><header><span>计划</span><strong>为什么投、投多少、投多久</strong></header><ul><li>广告主 / 营销目标 / 信息流或搜索</li><li>开始结束日期 / 分时段 / 投放速率</li><li>预算类型 / 日预算 / 总预算上限</li><li>出价策略 / 优化目标 / 事件资产</li></ul></article>
        <article><header><span>单元</span><strong>投给谁、用什么词、出多少钱</strong></header><ul><li>笔记或 SPU 标的 / 单元名称 / 目标成本</li><li>通投、智能或高级定向</li><li>年龄 / 性别 / 地域 / 设备 / 兴趣 / 人群包</li><li>关键词簇 / 匹配方式 / 单词出价 / 否定词</li></ul></article>
        <article><header><span>创意</span><strong>最终展示什么、如何转化</strong></header><ul><li>最终笔记 / 创意名称 / 标题与封面优选</li><li>转化组件 / 按钮文案 / 置顶评论</li><li>落地页或监测链接 / 资质组合</li><li>审核状态 / 失败原因 / 可替换候选</li></ul></article>
      </div>
    </section>
  </div>;
}

function MegaphoneIcon() {
  return <Route size={18} />;
}

function APIView() {
  const [filter, setFilter] = useState<"all" | StatusTone>("all");
  const rows = filter === "all" ? API_ROWS : API_ROWS.filter((row) => row.status === filter);
  return <div className="delivery-view">
    <section className="delivery-section first">
      <SectionHeading index="01" title="接口台账" description="内部稳定契约已经落地；状态区分中台完成度与聚光真实账号验收" />
      <div className="api-filter" role="group" aria-label="接口状态筛选">
        <button className={filter === "all" ? "active" : ""} onClick={() => setFilter("all")}>全部 {API_ROWS.length}</button>
        {(Object.keys(STATUS) as StatusTone[]).map((tone) => <button className={filter === tone ? "active" : ""} key={tone} onClick={() => setFilter(tone)}>{STATUS[tone].label} {API_ROWS.filter((row) => row.status === tone).length}</button>)}
      </div>
      <div className="api-table-wrap">
        <table className="api-ledger">
          <thead><tr><th>域</th><th>中台目标接口</th><th>上游候选</th><th>用途与返回目标</th><th>判断</th></tr></thead>
          <tbody>{rows.map((row) => <tr key={`${row.domain}-${row.internal}`}>
            <td><strong>{row.domain}</strong><span className={`method ${row.method.toLowerCase()}`}>{row.method}</span></td>
            <td><code>{row.internal}</code></td>
            <td><code>{row.upstream}</code></td>
            <td><p>{row.purpose}</p><small>{row.evidence}</small></td>
            <td><StatusPill tone={row.status} /></td>
          </tr>)}</tbody>
        </table>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="02" title="内部 API 设计原则" description="前端和 AI 永远不拼上游原始请求" />
      <div className="principle-grid">
        <article><LockKeyhole size={18} /><strong>Token 隔离</strong><p>Access Token 只在适配器进程内存中出现，日志、数据库、浏览器和模型上下文全部脱敏。</p></article>
        <article><GitBranch size={18} /><strong>版本与幂等</strong><p>草稿每次修改生成新版本；所有写请求要求 Idempotency-Key，媒体 ID 与内部版本一一映射。</p></article>
        <article><ClipboardCheck size={18} /><strong>审批快照</strong><p>审批的是规范化 JSON 哈希；预算、出价、笔记、定向或关键词变化会自动使审批失效。</p></article>
        <article><FileText size={18} /><strong>原始证据</strong><p>保存 request_id、请求摘要、原始响应、字段 diff 和错误分类，但不保存明文 Token。</p></article>
        <article><Route size={18} /><strong>异步作业</strong><p>发布返回 202 + job_id；客户端查询阶段进度，不用一个长 HTTP 请求承载三层创建。</p></article>
        <article><PauseCircle size={18} /><strong>默认暂停</strong><p>新计划一律暂停态创建。读回一致、审核通过且人工确认后才启用。</p></article>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="03" title="三层创建 payload 映射" description="字段是当前接入目标；必填组合、枚举和金额范围以 P0 登录态官方契约为最终依据" />
      <div className="payload-map">
        <article>
          <header><span>01</span><div><strong>Campaign</strong><code>POST /api/open/jg/campaign/create</code></div></header>
          <dl>
            <div><dt>身份</dt><dd>advertiser_id · campaign_name</dd></div>
            <div><dt>目标</dt><dd>marketing_target · placement · promotion_target · optimize_target</dd></div>
            <div><dt>周期</dt><dd>time_type · start_time · expire_time · time_period</dd></div>
            <div><dt>资金</dt><dd>bidding_strategy · limit_day_budget · campaign_day_budget · pacing_mode</dd></div>
            <div><dt>资产</dt><dd>event_asset_id · asset_event · page_category · tracking fields</dd></div>
          </dl>
          <footer><span>创建时强制 enable=0</span><strong>返回 campaign_id</strong></footer>
        </article>
        <article>
          <header><span>02</span><div><strong>Unit</strong><code>POST /api/open/jg/unit/create</code></div></header>
          <dl>
            <div><dt>归属</dt><dd>advertiser_id · campaign_id · unit_name</dd></div>
            <div><dt>标的</dt><dd>note_ids 或 spu_note_info · promotion_target</dd></div>
            <div><dt>竞价</dt><dd>event_bid · keyword_with_bid · match type · keyword_gen_type</dd></div>
            <div><dt>定向</dt><dd>target_type · age · gender · city · device · interest · crowd</dd></div>
            <div><dt>落地</dt><dd>page_id · landing_page_url · external_page_url · target_template_id</dd></div>
          </dl>
          <footer><span>选项来自广告主实时字典</span><strong>返回 unit_id</strong></footer>
        </article>
        <article>
          <header><span>03</span><div><strong>Creativity</strong><code>POST /api/open/jg/creativity/create</code></div></header>
          <dl>
            <div><dt>归属</dt><dd>advertiser_id · unit_id · creativity_name</dd></div>
            <div><dt>内容</dt><dd>note_id · mask_prefer · title_mask_prefer</dd></div>
            <div><dt>转化</dt><dd>conversion_type · bar_content · conversion_component_types</dd></div>
            <div><dt>链接</dt><dd>jump_url · landing_page_type · click_urls · expo_urls</dd></div>
            <div><dt>资质</dt><dd>apply_id · product_qual_id_list · brand_qual_id_list</dd></div>
          </dl>
          <footer><span>创建后等待审核并读回</span><strong>返回 creativity_id</strong></footer>
        </article>
      </div>
    </section>

    <section className="delivery-section contract-section">
      <SectionHeading index="04" title="草稿与建议契约示例" description="模型必须通过 JSON Schema；未知字段拒绝，金额统一使用分" />
      <div className="contract-grid">
        <article><header><Code2 size={17} /><strong>POST /v1/delivery/drafts</strong></header><pre>{DRAFT_EXAMPLE}</pre></article>
        <article><header><WandSparkles size={17} /><strong>recommendation/v1</strong></header><pre>{RECOMMENDATION_EXAMPLE}</pre></article>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="05" title="发布接口的验收用例" description="只有这些场景全部通过，才从草稿模式开放真实写入" />
      <div className="acceptance-list">
        {[
          "同一幂等键连续提交两次，只生成一套计划、单元和创意",
          "计划成功、第二个单元失败时，已创建对象保持暂停且可从断点补齐",
          "审批后修改一分钱预算，publish 返回 409 approval_stale",
          "Token 过期可刷新一次；刷新失败不降级使用失效 Token",
          "上游 429/5xx 重试有上限，所有 request_id 可检索",
          "余额不足、笔记失效、资质缺失、人群过窄在媒体写入前被拦截",
          "发布后读回字段与审批快照不一致时，永不自动启用",
          "任一日志、错误响应和模型 trace 中都找不到明文 Token"
        ].map((item) => <div key={item}><Check size={15} /><span>{item}</span></div>)}
      </div>
    </section>
  </div>;
}

const INTELLIGENCE_ROWS = [
  {
    decision: "稿件选择", llm: "抽取人群、场景、痛点、卖点、证据、商业强度与合规风险；生成可读理由",
    ml: "硬过滤后用 LambdaMART / LightGBM 排序，目标为分层后的转化概率、站外成本或 ROI",
    guardrail: "笔记必须可投且资质完整；冷启动只给候选，不自动排除新题材"
  },
  {
    decision: "关键词", llm: "从稿件生成种子意图，聚类平台返回词，标注品牌/品类/痛点/功效/竞品与否定意图",
    ml: "按历史 CTR、CVR、CPC、站外成本、搜索量和不确定性预测价值，分配匹配方式与初始出价",
    guardrail: "提交词必须来自平台推荐或校验通过；敏感、医疗承诺和无关词由规则硬拦截"
  },
  {
    decision: "年龄 / 地域", llm: "把稿件中的生活阶段和使用场景转成实验假设，不直接给确定答案",
    ml: "分层贝叶斯或 uplift 模型估计各人群增量；样本不足时向全量均值收缩并优先宽定向",
    guardrail: "只使用平台枚举；禁止根据敏感属性推断个人，按单变量实验验证"
  },
  {
    decision: "出价 / 预算", llm: "解释为什么建议调价，生成运营摘要和异常排查顺序",
    ml: "响应曲线 + 约束优化，在日预算、账户余额、目标成本和学习期约束内给阶梯建议",
    guardrail: "LLM 不产出最终金额；首版人工确认，单次和单日变动比例均有限额"
  },
  {
    decision: "创意组合", llm: "检查标题、封面、正文和转化组件一致性，提出单变量测试版本",
    ml: "多任务模型预测点击、互动、回搜、站外激活和成交，不把单一互动率当最终目标",
    guardrail: "资质与禁用词由确定性规则判断；模型不能修改已发布笔记内容"
  },
  {
    decision: "运行调优", llm: "总结异常、解释建议和生成复盘，不负责自动启停",
    ml: "先固定阈值熔断，再做贝叶斯实验；数据成熟后才引入约束 contextual bandit",
    guardrail: "保留控制组、最小样本和最长观察窗；重大预算动作始终需要审批"
  }
];

function IntelligenceView() {
  return <div className="delivery-view">
    <section className="delivery-section first">
      <SectionHeading index="01" title="AI 与算法的职责分工" description="生成式模型处理语义和解释，机器学习做排序与预测，规则引擎守住可投与资金边界" />
      <div className="intelligence-legend">
        <span><Bot size={15} />LLM：语义候选</span><span><BarChart3 size={15} />ML：数据排序</span><span><ShieldCheck size={15} />规则：硬约束</span><span><UserCheck size={15} />人工：最终责任</span>
      </div>
      <div className="intelligence-table-wrap">
        <table className="intelligence-table"><thead><tr><th>决策点</th><th>LLM 使用</th><th>机器学习 / 统计</th><th>上线护栏</th></tr></thead><tbody>
          {INTELLIGENCE_ROWS.map((row) => <tr key={row.decision}><th>{row.decision}</th><td>{row.llm}</td><td>{row.ml}</td><td>{row.guardrail}</td></tr>)}
        </tbody></table>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="02" title="稿件选择与关键词生成链路" description="先召回、再过滤、后排序，任何模型都不能绕过平台可投性" />
      <div className="model-pipeline" role="img" aria-label="稿件选择和关键词建议模型链路">
        {[
          [Database, "候选池", "可投笔记 + 定稿 + 历史指标"],
          [Filter, "硬过滤", "资质 · 状态 · SPU · 合规"],
          [BrainCircuit, "语义抽取", "人群 · 场景 · 卖点 · 风险"],
          [Layers3, "特征拼接", "内容 + 投放 + 时效 + 机构"],
          [BarChart3, "排序校准", "预期价值 + 置信区间"],
          [ListChecks, "多样性重排", "避免同质稿件和词簇"],
          [UserCheck, "运营确认", "选择理由 + 风险 + 实验变量"]
        ].map(([Icon, title, detail], index, items) => {
          const StepIcon = Icon as LucideIcon;
          return <div className="model-step-wrap" key={String(title)}><article><StepIcon size={18} /><strong>{String(title)}</strong><small>{String(detail)}</small></article>{index < items.length - 1 ? <ArrowRight size={16} /> : null}</div>;
        })}
      </div>
      <div className="feature-groups">
        <article><strong>内容特征</strong><span>标题/封面 embedding、场景、卖点证据、商业强度、内容类型、稿件完整度</span></article>
        <article><strong>投放特征</strong><span>场域、关键词意图、匹配方式、年龄、地域、出价、预算、频次、创建小时</span></article>
        <article><strong>历史结果</strong><span>曝光、点击、互动、回搜、15 天站外成本、成交、ROI；自然与付费尽量分离</span></article>
        <article><strong>可靠性特征</strong><span>样本量、数据新鲜度、缺失率、机构、达人、季节、版本、归因窗是否完整</span></article>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="03" title="模型路线与最低数据要求" description="不要一开始就训练复杂模型；先把标签、反事实和时间泄漏处理正确" />
      <div className="model-roadmap">
        <article><span>M0</span><div><strong>规则 + 描述统计</strong><p>冷启动阶段。用业务阈值、相似稿件检索、分层均值和贝叶斯平滑给建议。</p><small>要求：字段完整、口径固定、样本可追溯</small></div></article>
        <article><span>M1</span><div><strong>监督排序</strong><p>用时间切分的 LightGBM / LambdaMART 排稿件和关键词；输出校准概率与区间。</p><small>要求：每个主要场景至少数百条有效样本，正负例均存在</small></div></article>
        <article><span>M2</span><div><strong>早期预测</strong><p>用 survival / multi-task 模型从 1-3 天信号预测 7/15 天结果，避免未成熟标签。</p><small>要求：稳定的小时/日报、完整归因窗和延迟标签处理</small></div></article>
        <article><span>M3</span><div><strong>增量与在线探索</strong><p>随机对照积累后做 uplift；再以有预算和风险约束的 contextual bandit 小流量探索。</p><small>要求：控制组、随机化、倾向分、回放评估和 kill switch</small></div></article>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="04" title="LLM 工程框架" description="模型输出是有版本的建议数据，不是自由文本命令" />
      <div className="llm-framework">
        <article><header><Database size={17} /><strong>检索上下文</strong></header><p>只检索本广告主可访问的稿件、历史实验、平台枚举与已批准品牌规范；上下文记录 source_id 和时间。</p></article>
        <article><header><Braces size={17} /><strong>结构化输出</strong></header><p>强制 JSON Schema，要求 evidence、uncertainties、alternatives、requires_human_review；解析失败不做静默修复。</p></article>
        <article><header><ShieldCheck size={17} /><strong>二次校验</strong></header><p>敏感词、医疗承诺、平台字段、预算和人群规模由规则服务验证；LLM 的数值不直接进入发布 payload。</p></article>
        <article><header><TestTube2 size={17} /><strong>离线评测</strong></header><p>黄金集覆盖选稿、意图聚类、错误拒答、证据引用和 JSON 合法性；每次模型或提示词升级做回归。</p></article>
        <article><header><FileCheck2 size={17} /><strong>可观测性</strong></header><p>保存模型、提示词版本、输入数据 ID、输出哈希、耗时与人工采纳结果；敏感正文按权限和期限留存。</p></article>
        <article><header><CircleDollarSign size={17} /><strong>成本控制</strong></header><p>内容抽取离线批处理并缓存；在线只重排新增候选。低价值任务使用小模型，禁止逐页面重复生成。</p></article>
      </div>
    </section>
  </div>;
}

function ArchitectureView() {
  return <div className="delivery-view">
    <section className="delivery-section first">
      <SectionHeading index="01" title="目标系统架构" description="在现有 Go 服务和聚光认证守护进程上扩展，不让前端、模型或数据库接触媒体 Token" />
      <div className="architecture-diagram" role="img" aria-label="自建投流系统架构图">
        <div className="architecture-column users-layer">
          <span className="architecture-label">使用者</span>
          <article><Users size={19} /><div><strong>运营 / 投手</strong><small>建草稿 · 调整 · 启停</small></div></article>
          <article><UserCheck size={19} /><div><strong>预算审批人</strong><small>额度 · 风险 · 审计</small></div></article>
        </div>
        <ArrowRight className="architecture-arrow" size={21} />
        <div className="architecture-column app-layer">
          <span className="architecture-label">中台应用层</span>
          <article><SlidersHorizontal size={19} /><div><strong>React 投流工作台</strong><small>动态表单 · diff · 作业状态</small></div></article>
          <article><Workflow size={19} /><div><strong>Delivery Orchestrator</strong><small>状态机 · Saga · 幂等 · 审批</small></div></article>
          <article><ShieldCheck size={19} /><div><strong>Policy Engine</strong><small>权限 · 预算 · 合规 · 熔断</small></div></article>
        </div>
        <div className="architecture-split"><ArrowRight size={21} /><ArrowRight size={21} /></div>
        <div className="architecture-column services-layer">
          <span className="architecture-label">决策与数据层</span>
          <article><BrainCircuit size={19} /><div><strong>Decision Service</strong><small>LLM 抽取 · 排序 · 实验建议</small></div></article>
          <article><Database size={19} /><div><strong>PostgreSQL + 特征视图</strong><small>草稿 · 审批 · 映射 · 指标</small></div></article>
          <article><BarChart3 size={19} /><div><strong>Report Ingestion</strong><small>实时/离线报表 · 业务转化</small></div></article>
        </div>
        <ArrowRight className="architecture-arrow" size={21} />
        <div className="architecture-column media-layer">
          <span className="architecture-label">媒体边界</span>
          <article><LockKeyhole size={19} /><div><strong>XHS Adapter / Authd</strong><small>Token · 限流 · 重试 · 脱敏</small></div></article>
          <article><ServerCog size={19} /><div><strong>聚光 Marketing API</strong><small>资产 · 创建 · 状态 · 报表</small></div></article>
        </div>
      </div>
      <div className="architecture-feedback"><RefreshCw size={16} /><span>投放和业务结果回流 Report Ingestion，经口径校验后进入特征视图；模型只读快照，不查询生产媒体 Token。</span></div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="02" title="核心数据模型" description="媒体对象、内部版本和审批记录分离，避免上游状态覆盖业务事实" />
      <div className="data-model-grid">
        {[
          ["delivery_drafts", "业务目标、广告主、当前状态、预算总额与幂等键"],
          ["delivery_draft_versions", "规范化草稿 JSON、版本、哈希、创建来源"],
          ["delivery_recommendations", "模型/提示词版本、候选、证据、不确定性、采纳结果"],
          ["delivery_validations", "规则版本、平台能力快照、错误、警告和有效期"],
          ["delivery_approvals", "审批人、角色、草稿哈希、金额、决定和过期时间"],
          ["delivery_publish_jobs", "状态机、幂等键、当前步骤、重试次数和错误类别"],
          ["delivery_media_entities", "内部对象到 campaign/unit/creativity ID 的不可变映射"],
          ["delivery_api_attempts", "脱敏请求摘要、响应、request_id、延迟与重试"],
          ["delivery_performance_snapshots", "账户到关键词分层报表、契约版本与抓取时间"],
          ["delivery_guardrail_events", "预算、成本、异常消耗、漂移与人工处理结果"]
        ].map(([name, detail]) => <article key={name}><code>{name}</code><p>{detail}</p></article>)}
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="03" title="安全、权限与审计" description="这是资金系统，权限模型应高于普通分析页面" />
      <div className="security-matrix">
        <div className="security-header"><span>角色</span><span>草稿</span><span>建议</span><span>审批</span><span>发布</span><span>启停</span><span>扩预算</span></div>
        {[
          ["分析员", "读", "读", "-", "-", "-", "-"],
          ["运营", "写", "生成", "提交", "-", "-", "-"],
          ["投手", "写", "生成", "提交", "执行", "执行", "限额内"],
          ["预算审批人", "读", "读", "批准", "-", "-", "批准"],
          ["系统任务", "-", "批处理", "-", "已批准作业", "仅熔断暂停", "禁止"]
        ].map((row) => <div key={row[0]}>{row.map((cell, index) => <span key={`${row[0]}-${index}`} className={cell === "-" || cell === "禁止" ? "denied" : ""}>{cell}</span>)}</div>)}
      </div>
      <div className="audit-requirements">
        <span><Check size={14} />RBAC + 广告主级数据范围</span><span><Check size={14} />预算动作二次认证</span><span><Check size={14} />不可篡改审计事件</span><span><Check size={14} />Token 与正文分级脱敏</span><span><Check size={14} />紧急全局暂停开关</span>
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="04" title="运行指标与告警" description="业务效果和系统可靠性分别监控，不能只看是否成功创建" />
      <div className="observability-grid">
        <article><Gauge size={18} /><strong>接口可靠性</strong><p>成功率、p95 延迟、429/5xx、Token 刷新、重试、重复对象、字段漂移。</p></article>
        <article><CircleDollarSign size={18} /><strong>资金安全</strong><p>小时消耗斜率、广告主日额度、计划预算利用率、异常增幅和暂停延迟。</p></article>
        <article><BarChart3 size={18} /><strong>模型质量</strong><p>候选采纳率、排序 NDCG、概率校准、分群误差、漂移和相对基线增益。</p></article>
        <article><TestTube2 size={18} /><strong>实验质量</strong><p>样本比例偏差、变量污染、归因成熟度、控制组覆盖和提前停止次数。</p></article>
      </div>
    </section>
  </div>;
}

const PHASES = [
  {
    phase: "P0", duration: "1-2 周", title: "权限与契约验证", tone: "discovery",
    scope: ["申请并核对投放管理、工具、报表 scope", "保存官方登录态文档字段与更新时间", "真实广告主白名单、余额、资质和测试账户", "对创建/暂停/查询做最小冒烟，不接生产预算"],
    exit: "三层创建均能以暂停态成功并读回；错误码、频控和幂等策略有实测记录"
  },
  {
    phase: "P1", duration: "2-3 周", title: "草稿与人工发布", tone: "foundation",
    scope: ["草稿版本、动态表单、校验和双人审批", "计划→单元→创意 Saga 与作业状态", "默认暂停、读回 diff、审计和预算上限", "仅授权投手人工启用与暂停"],
    exit: "预发 30 组故障用例通过；生产灰度 1 个广告主、5 个计划零重复零越权"
  },
  {
    phase: "P2", duration: "2-4 周", title: "AI 辅助决策", tone: "intelligence",
    scope: ["稿件语义抽取与相似候选", "平台关键词召回、意图聚类和否词", "年龄/地域单变量实验建议", "结构化理由、不确定性与人工采纳记录"],
    exit: "离线黄金集通过；建议采纳后无字段越界，运营搭建时间中位数下降 50%"
  },
  {
    phase: "P3", duration: "4-6 周", title: "效果闭环与建议调优", tone: "optimization",
    scope: ["实时/离线报表和业务转化统一口径", "固定规则熔断与日级调价建议", "时间切分排序模型和概率校准", "A/B 实验、控制组、模型/提示词版本追踪"],
    exit: "回放无超预算动作；线上实验达到预设样本，主指标改善且保护指标不恶化"
  },
  {
    phase: "P4", duration: "持续", title: "受控自动化", tone: "automation",
    scope: ["小额度自动暂停和恢复", "约束优化分配预算", "成熟场景 contextual bandit", "模型漂移、降级规则和季度权限复核"],
    exit: "仅对已验证广告主和低风险动作开放；全局 kill switch、回滚和人工接管演练通过"
  }
];

function RolloutView() {
  return <div className="delivery-view">
    <section className="delivery-section first">
      <SectionHeading index="01" title="分期实施路线" description="先证明权限与资金安全，再追求模型收益；每一期都有退出条件" />
      <div className="phase-list">
        {PHASES.map((phase) => <article className={`phase-card ${phase.tone}`} key={phase.phase}>
          <header><span>{phase.phase}</span><div><h3>{phase.title}</h3><small>{phase.duration}</small></div></header>
          <ul>{phase.scope.map((item) => <li key={item}>{item}</li>)}</ul>
          <footer><CheckCircle2 size={15} /><div><strong>退出条件</strong><p>{phase.exit}</p></div></footer>
        </article>)}
      </div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="02" title="建议的研发拆分" description="按责任边界并行推进，避免把媒体适配、AI 和审批揉成一个服务" />
      <div className="workstream-table-wrap"><table className="workstream-table"><thead><tr><th>工作流</th><th>主要产物</th><th>依赖</th><th>完成判据</th></tr></thead><tbody>
        {[
          ["媒体适配", "能力探测、资产、工具、创建、状态、报表客户端与契约测试", "官方权限与测试广告主", "真实响应与官方契约一致；错误全部分类"],
          ["投放编排", "草稿、校验、审批、Saga、幂等、读回和暂停", "媒体适配", "故障注入、重复提交和部分失败全部可恢复"],
          ["前端工作台", "三层动态表单、建议证据、diff、审批和作业状态", "内部 Schema", "桌面与移动端关键流无横向溢出，字段状态完整"],
          ["数据闭环", "报表采集、指标语义层、归因成熟度和特征视图", "报表 scope", "媒体与业务指标可按对象、日期和版本追溯"],
          ["AI / ML", "语义抽取、排序、评测集、模型注册与采纳日志", "数据闭环", "时间外验证优于规则基线且概率校准达标"],
          ["安全治理", "RBAC、预算策略、审计、密钥、告警和 kill switch", "所有工作流", "越权、超额和 Token 泄露测试为零容忍"]
        ].map((row) => <tr key={row[0]}>{row.map((cell, index) => index === 0 ? <th key={cell}>{cell}</th> : <td key={cell}>{cell}</td>)}</tr>)}
      </tbody></table></div>
    </section>

    <section className="delivery-section">
      <SectionHeading index="03" title="生产上线总验收" description="这是完成自建投流能力的 Definition of Done" />
      <div className="dod-grid">
        {[
          [ShieldCheck, "权限与资金", "广告主级 RBAC、双人审批、日/总预算、余额检查、全局暂停全部生效"],
          [ServerCog, "媒体一致性", "创建、读回、启停、报表跑通；字段 diff 为零，部分失败可恢复"],
          [GitBranch, "幂等与审计", "重复请求不重复花费；每次建议、修改、审批和媒体调用可追溯"],
          [BrainCircuit, "AI 可信", "输出 Schema 合法、引用可核对、不确定性可见，模型无写权限"],
          [TestTube2, "实验有效", "有控制组、单变量、成熟归因窗和保护指标，不用相关性冒充增量"],
          [Gauge, "运行可靠", "SLO、限流、熔断、告警、回滚和人工接管演练全部通过"]
        ].map(([Icon, title, detail]) => {
          const ItemIcon = Icon as LucideIcon;
          return <article key={String(title)}><ItemIcon size={19} /><div><strong>{String(title)}</strong><p>{String(detail)}</p></div></article>;
        })}
      </div>
    </section>

    <section className="delivery-section risk-register">
      <SectionHeading index="04" title="关键风险登记" description="负责人在每个阶段评审，不把风险留到上线后" />
      <div className="risk-list">
        <div><span className="risk high">高</span><strong>ad_manage 已授权，但不同广告主或端点能力可能不同</strong><p>按广告主保存能力快照；P0 不通过则保持草稿导出模式。</p></div>
        <div><span className="risk high">高</span><strong>重复创建或超预算造成直接资金损失</strong><p>幂等键、名称查重、媒体读回、预算总账和默认暂停共同防护。</p></div>
        <div><span className="risk medium">中</span><strong>平台字段与枚举频繁变化</strong><p>动态字典、契约版本、灰度适配器和每日能力探测；前端不硬编码。</p></div>
        <div><span className="risk medium">中</span><strong>历史数据存在选择偏差与自然流量混入</strong><p>先做随机实验和分层基线；报告置信区间，不把历史相关当因果。</p></div>
        <div><span className="risk medium">中</span><strong>LLM 幻觉关键词、年龄或合规结论</strong><p>平台候选集合 + JSON Schema + 硬规则；模型输出只能作为待确认建议。</p></div>
        <div><span className="risk low">低</span><strong>模型收益不稳定</strong><p>模型降级为规则不影响发布链路；版本化评测和在线控制组持续验证。</p></div>
      </div>
    </section>
  </div>;
}

function SelfServeDeliveryPlan() {
  const [view, setView] = useState<ViewKey>("decision");
  return <div className="self-serve-delivery">
    <section className="page-heading delivery-page-heading">
      <div><h1>自建投流</h1><p>聚光 Marketing API · 后端能力与验收台账</p></div>
      <div className="delivery-baseline"><span>方案基线</span><strong>2026-08-13</strong></div>
    </section>

    <section className="delivery-topline">
      <div><span className="topline-icon"><Route size={20} /></span><div><strong>后端接口已落地，上游写入待验收</strong><p>草稿、审批、算法建议、异步发布、读回和报表链路已经实现；专用广告主完成暂停态冒烟前，全局媒体写开关保持关闭。</p></div></div>
      <span className="topline-status"><i />尚未开放真实写入</span>
    </section>

    <nav className="delivery-tabs" aria-label="自建投流方案章节">
      {VIEWS.map((item) => <button key={item.key} className={view === item.key ? "active" : ""} onClick={() => setView(item.key)} aria-current={view === item.key ? "page" : undefined}>
        <item.icon size={16} /><span>{item.label}</span>
      </button>)}
    </nav>

    {view === "decision" ? <DecisionView /> : null}
    {view === "workflow" ? <WorkflowView /> : null}
    {view === "api" ? <APIView /> : null}
    {view === "intelligence" ? <IntelligenceView /> : null}
    {view === "architecture" ? <ArchitectureView /> : null}
    {view === "rollout" ? <RolloutView /> : null}
  </div>;
}

export default SelfServeDeliveryPlan;
