import {
  AlertCircle, ArrowRight, Check, CheckCircle2, CirclePlus, Database, FileText,
  Gauge, LoaderCircle, Pencil, RefreshCw, RotateCcw, Rows3, Search, ShieldCheck,
  SlidersHorizontal, Tags, Users, XCircle
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import {
  createClientKey, deliveryAPI, type CandidateNote, type DeliverySession, type Draft,
  type QuickPlanOverrides, type QuickPlanTemplate, type QuickPlanTemplates, type Validation
} from "./delivery-api";
import { errorMessage } from "./delivery-ui";
import "./quick-plan-creator.css";

type Placement = "feed" | "search";
type PageNotice = { tone: "success" | "error" | "info"; message: string } | null;
type EditablePlanSettings = {
  marketingTarget: 4 | 13;
  biddingStrategy: 2 | 3 | 7;
  dailyBudgetYuan: string;
  bidYuan: string;
  pacingMode: 0 | 1 | 2;
  timePeriodType: 0 | 1;
  phraseMatchType: 0 | 1 | 2 | 3;
};

const MARKETING_TARGETS: Record<number, string> = {
  3: "商品销量", 4: "产品种草", 8: "直播推广", 9: "客资收集", 10: "抢占关键词",
  13: "种草直达", 14: "直播预热", 15: "店铺拉新", 16: "应用唤起", 20: "应用下载", 21: "小程序推广"
};
const BIDDING_STRATEGIES: Record<number, string> = { 2: "手动出价", 3: "最大转化", 7: "稳定成本" };
const PACING_MODES: Record<number, string> = { 0: "系统默认", 1: "匀速投放", 2: "加速投放" };
const TARGET_TYPES: Record<number, string> = { 0: "默认定向", 1: "通投", 2: "智能定向", 3: "高级定向" };
const MATCH_TYPES: Record<number, string> = { 0: "精确匹配", 1: "短语匹配", 2: "智能匹配", 3: "升级匹配" };
const money = new Intl.NumberFormat("zh-CN", { style: "currency", currency: "CNY" });
const integer = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
const dateTime = new Intl.DateTimeFormat("zh-CN", {
  year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false
});

function formatDate(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : dateTime.format(date);
}

function formatFen(value = 0): string {
  return money.format(value / 100);
}

function fenInput(value = 0): string {
  return (value / 100).toFixed(2).replace(/\.00$/, "");
}

function parseFenInput(value: string): number | undefined {
  if (!value.trim()) return undefined;
  const amount = Number(value);
  if (!Number.isFinite(amount) || amount < 0) return undefined;
  return Math.round(amount * 100);
}

function settingsFromTemplate(template: QuickPlanTemplate): EditablePlanSettings {
  const marketingTarget = template.summary.marketing_target === 13 ? 13 : 4;
  const biddingStrategy = template.summary.bidding_strategy === 2 || template.summary.bidding_strategy === 3 ? template.summary.bidding_strategy : 7;
  const pacingMode = template.summary.pacing_mode === 0 || template.summary.pacing_mode === 2 ? template.summary.pacing_mode : 1;
  const timePeriodType = template.summary.time_period_type === 1 ? 1 : 0;
  const phraseMatchType = template.keyword_defaults.phrase_match_type >= 0 && template.keyword_defaults.phrase_match_type <= 3
    ? template.keyword_defaults.phrase_match_type as 0 | 1 | 2 | 3
    : 1;
  return {
    marketingTarget, biddingStrategy, pacingMode, timePeriodType, phraseMatchType,
    dailyBudgetYuan: fenInput(template.summary.day_budget_fen),
    bidYuan: fenInput(template.placement === "search" ? template.keyword_defaults.bid_fen : template.summary.event_bid_fen)
  };
}

function parseKeywords(value: string): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const item of value.split(/[\n,，]+/)) {
    const keyword = item.trim();
    const key = keyword.toLocaleLowerCase("zh-CN");
    if (!keyword || seen.has(key)) continue;
    seen.add(key);
    result.push(keyword);
  }
  return result;
}

function PageNoticeLine({ value, onClose }: { value: PageNotice; onClose: () => void }) {
  if (!value) return null;
  const Icon = value.tone === "success" ? CheckCircle2 : value.tone === "error" ? XCircle : AlertCircle;
  return <div className={`qp-notice ${value.tone}`} role={value.tone === "error" ? "alert" : "status"}>
    <Icon size={17} /><span>{value.message}</span>
    <button type="button" aria-label="关闭消息" title="关闭消息" onClick={onClose}>×</button>
  </div>;
}

function TemplateEvidence({ template }: { template: QuickPlanTemplate }) {
  const summary = template.summary;
  const confidence = Math.round(template.confidence * 100);
  const metrics = [
    { label: "营销目标", value: MARKETING_TARGETS[summary.marketing_target] || `代码 ${summary.marketing_target}` },
    { label: "出价策略", value: BIDDING_STRATEGIES[summary.bidding_strategy] || `代码 ${summary.bidding_strategy}` },
    { label: "日预算", value: formatFen(summary.day_budget_fen) },
    { label: "单元出价", value: summary.event_bid_fen > 0 ? formatFen(summary.event_bid_fen) : "按关键词" },
    { label: "消耗节奏", value: PACING_MODES[summary.pacing_mode] || `代码 ${summary.pacing_mode}` },
    { label: "投放时段", value: summary.time_period_type === 0 ? "全天" : "自定义时段" }
  ];
  return <section className="qp-template-band" aria-label={`${template.placement === "feed" ? "信息流" : "搜索"}默认模板`}>
    <header>
      <div><Gauge size={18} /><div><h2>全局统计基准</h2><p>全部广告主的有效计划、有效单元、审核通过创意</p></div></div>
      <div className="qp-evidence-count"><strong>{integer.format(template.sample_count)}</strong><span>个在投样本</span></div>
      <div className="qp-evidence-count"><strong>{confidence}%</strong><span>组合覆盖</span></div>
      <div className="qp-evidence-time"><span>最近同步</span><strong>{formatDate(template.latest_synced_at)}</strong></div>
    </header>
    <div className="qp-template-metrics">
      {metrics.map((item) => <div key={item.label}><span>{item.label}</span><strong>{item.value}</strong></div>)}
    </div>
  </section>;
}

function PlanSettingsEditor({ placement, value, onChange, onReset }: {
  placement: Placement;
  value: EditablePlanSettings;
  onChange: (value: EditablePlanSettings) => void;
  onReset: () => void;
}) {
  const dailyBudgetFen = parseFenInput(value.dailyBudgetYuan);
  const bidFen = parseFenInput(value.bidYuan);
  const update = (patch: Partial<EditablePlanSettings>) => onChange({ ...value, ...patch });
  return <section className="qp-settings-panel">
    <header>
      <div><SlidersHorizontal size={18} /><div><h2>本次计划参数</h2><p>已按全局统计值预填</p></div></div>
      <button type="button" className="qp-secondary-button" onClick={onReset}><RotateCcw size={15} />恢复统计值</button>
    </header>
    <div className="qp-settings-grid">
      <label><span>营销目标</span><select aria-label="营销目标" value={value.marketingTarget} onChange={(event) => update({ marketingTarget: Number(event.target.value) as 4 | 13 })}>
        <option value={4}>产品种草</option><option value={13}>种草直达</option>
      </select></label>
      <label><span>出价策略</span><select aria-label="出价策略" value={value.biddingStrategy} onChange={(event) => update({ biddingStrategy: Number(event.target.value) as 2 | 3 | 7 })}>
        <option value={2}>手动出价</option><option value={3}>最大转化</option><option value={7}>稳定成本</option>
      </select></label>
      <label><span>日预算（元）</span><input aria-label="日预算" type="number" inputMode="decimal" min="100" max="999998" step="10" aria-invalid={dailyBudgetFen === undefined || dailyBudgetFen < 10_000} value={value.dailyBudgetYuan} onChange={(event) => update({ dailyBudgetYuan: event.target.value })} /></label>
      <label><span>{placement === "search" ? "关键词出价（元）" : "单元出价（元）"}</span><input aria-label={placement === "search" ? "关键词出价" : "单元出价"} type="number" inputMode="decimal" min={placement === "search" ? "0.01" : "0"} max="999998" step="0.01" aria-invalid={bidFen === undefined || (placement === "search" ? bidFen <= 0 : bidFen < 0)} value={value.bidYuan} onChange={(event) => update({ bidYuan: event.target.value })} /></label>
      <label><span>消耗节奏</span><select aria-label="消耗节奏" value={value.pacingMode} onChange={(event) => update({ pacingMode: Number(event.target.value) as 0 | 1 | 2 })}>
        <option value={0}>系统默认</option><option value={1}>匀速投放</option><option value={2}>加速投放</option>
      </select></label>
      <label><span>投放时段</span><select aria-label="投放时段" value={value.timePeriodType} onChange={(event) => update({ timePeriodType: Number(event.target.value) as 0 | 1 })}>
        <option value={0}>全天</option><option value={1}>模板自定义时段</option>
      </select></label>
      {placement === "search" ? <label><span>关键词匹配</span><select aria-label="关键词匹配" value={value.phraseMatchType} onChange={(event) => update({ phraseMatchType: Number(event.target.value) as 0 | 1 | 2 | 3 })}>
        <option value={0}>精确匹配</option><option value={1}>短语匹配</option><option value={2}>智能匹配</option><option value={3}>升级匹配</option>
      </select></label> : null}
    </div>
  </section>;
}

function NotePicker({ advertiserID, loading, notes, selected, search, onSearch, onSubmit, onSelect }: {
  advertiserID: number;
  loading: boolean;
  notes: CandidateNote[];
  selected?: CandidateNote;
  search: string;
  onSearch: (value: string) => void;
  onSubmit: (event: FormEvent) => void;
  onSelect: (note: CandidateNote) => void;
}) {
  return <section className="qp-tool-panel qp-note-picker">
    <header className="qp-panel-heading">
      <div><FileText size={18} /><div><h2>笔记</h2><p>{selected ? "已选择 1 篇" : "未选择"}</p></div></div>
      <code>{advertiserID || "-"}</code>
    </header>
    <form className="qp-search" onSubmit={onSubmit}>
      <Search size={16} /><input value={search} onChange={(event) => onSearch(event.target.value)} placeholder="标题或笔记 ID" aria-label="搜索笔记" />
      <button type="submit" disabled={loading} aria-label="搜索笔记" title="搜索笔记">{loading ? <LoaderCircle size={16} className="spin" /> : <ArrowRight size={16} />}</button>
    </form>
    <div className="qp-note-list" aria-busy={loading}>
      {loading && notes.length === 0 ? <div className="qp-panel-state"><LoaderCircle size={18} className="spin" />加载笔记</div> : null}
      {!loading && notes.length === 0 ? <div className="qp-panel-state"><FileText size={20} />没有匹配笔记</div> : null}
      {notes.map((note) => {
        const active = selected?.note_id === note.note_id;
        return <button type="button" className={`qp-note-row ${active ? "selected" : ""}`} key={note.note_id} onClick={() => onSelect(note)}>
          <span className="qp-choice-mark">{active ? <Check size={14} /> : null}</span>
          <span className="qp-note-copy"><strong>{note.title || "未命名笔记"}</strong><code>{note.note_id}</code><small>{[...note.audience, ...note.scenarios].slice(0, 3).join(" · ") || "暂无内容标签"}</small></span>
          <span className="qp-note-history"><strong>{integer.format(note.historical_search_users)}</strong><small>回搜人数</small></span>
        </button>;
      })}
    </div>
  </section>;
}

function AudienceInput({ template, selectedID, onSelect }: { template: QuickPlanTemplate; selectedID: string; onSelect: (value: string) => void }) {
  const selected = template.audiences.find((item) => item.id === selectedID);
  return <section className="qp-input-section">
    <header><Users size={18} /><div><h2>人群</h2><p>{template.audiences.length} 个在投配置</p></div></header>
    <label className="qp-select-field"><span>定向配置</span><select value={selectedID} onChange={(event) => onSelect(event.target.value)}>
      {template.audiences.map((audience) => <option value={audience.id} key={audience.id}>{audience.name} · {audience.sample_count} 个样本</option>)}
    </select></label>
    {selected ? <div className="qp-audience-detail"><strong>{selected.name}</strong><span>{TARGET_TYPES[selected.target_type] || `定向代码 ${selected.target_type}`}</span><p>{selected.description}</p></div> : null}
  </section>;
}

function KeywordInput({ placement, template, value, onChange }: { placement: Placement; template: QuickPlanTemplate; value: string; onChange: (value: string) => void }) {
  const keywords = parseKeywords(value);
  const defaults = template.keyword_defaults;
  return <section className="qp-input-section qp-keyword-section">
    <header><Tags size={18} /><div><h2>{placement === "search" ? "搜索关键词" : "定向关键词"}</h2><p>{placement === "search" ? `${MATCH_TYPES[defaults.phrase_match_type] || `匹配代码 ${defaults.phrase_match_type}`} · ${formatFen(defaults.bid_fen)}` : "可选"}</p></div></header>
    <label className="qp-textarea-field"><span>已选 {keywords.length} 个</span><textarea rows={8} value={value} onChange={(event) => onChange(event.target.value)} placeholder={placement === "search" ? "每行一个搜索词" : "每行一个定向词"} /></label>
  </section>;
}

function ResultPanel({ draft, validation, advertiserID, onEdit, onOpen, onReset }: { draft: Draft; validation?: Validation; advertiserID: number; onEdit: () => void; onOpen: () => void; onReset: () => void }) {
  const valid = validation?.valid;
  const issues = validation ? [...validation.errors, ...validation.warnings].slice(0, 8) : [];
  return <section className={`qp-result ${valid ? "valid" : validation ? "invalid" : "pending"}`}>
    <header>{valid ? <CheckCircle2 size={22} /> : validation ? <AlertCircle size={22} /> : <LoaderCircle size={22} />}
      <div><h2>{valid ? "草稿已创建并通过校验" : validation ? `${validation.errors.length} 项错误阻止发布` : "草稿已创建"}</h2><p><code>{draft.id}</code> · 广告主 {advertiserID}</p></div>
      <div className="qp-result-actions"><button type="button" className="qp-secondary-button" onClick={onReset}><RefreshCw size={15} />再建一个</button><button type="button" className="qp-secondary-button" onClick={onEdit}><Pencil size={15} />继续编辑</button><button type="button" className="qp-primary-button" onClick={onOpen}>进入审批发布<ArrowRight size={15} /></button></div>
    </header>
    {issues.length ? <div className="qp-issue-list">{issues.map((issue, index) => <div key={`${issue.code}-${index}`}><span className={issue.severity}>{issue.severity === "error" ? "错误" : "警告"}</span><code>{issue.path}</code><p>{issue.message}</p></div>)}</div> : null}
  </section>;
}

export default function QuickPlanCreator() {
  const navigate = useNavigate();
  const preferredAdvertiser = Number(new URLSearchParams(window.location.search).get("advertiser_id")) || 0;
  const [session, setSession] = useState<DeliverySession>();
  const [advertiserID, setAdvertiserID] = useState(preferredAdvertiser);
  const [templates, setTemplates] = useState<QuickPlanTemplates>();
  const [planSettings, setPlanSettings] = useState<Partial<Record<Placement, EditablePlanSettings>>>({});
  const [placement, setPlacement] = useState<Placement>("feed");
  const [audienceIDs, setAudienceIDs] = useState<Record<Placement, string>>({ feed: "", search: "" });
  const [keywordText, setKeywordText] = useState<Record<Placement, string>>({ feed: "", search: "" });
  const [assetSearch, setAssetSearch] = useState("");
  const [notes, setNotes] = useState<CandidateNote[]>([]);
  const [selectedNote, setSelectedNote] = useState<CandidateNote>();
  const [loadingSession, setLoadingSession] = useState(true);
  const [loadingTemplate, setLoadingTemplate] = useState(false);
  const [templateRefresh, setTemplateRefresh] = useState(0);
  const [loadingNotes, setLoadingNotes] = useState(false);
  const [creating, setCreating] = useState(false);
  const [notice, setNotice] = useState<PageNotice>(null);
  const [draft, setDraft] = useState<Draft>();
  const [validation, setValidation] = useState<Validation>();
  const requestKey = useRef(createClientKey("quick-plan-ui"));

  useEffect(() => {
    let cancelled = false;
    setLoadingSession(true);
    deliveryAPI.session().then((next) => {
      if (cancelled) return;
      setSession(next);
      const preferred = next.advertisers.some((item) => item.advertiser_id === preferredAdvertiser) ? preferredAdvertiser : 0;
      setAdvertiserID((current) => next.advertisers.some((item) => item.advertiser_id === current) ? current : preferred || next.advertisers[0]?.advertiser_id || 0);
    }).catch((error) => {
      if (!cancelled) setNotice({ tone: "error", message: errorMessage(error) });
    }).finally(() => { if (!cancelled) setLoadingSession(false); });
    return () => { cancelled = true; };
  }, [preferredAdvertiser]);

  useEffect(() => {
    let cancelled = false;
    setLoadingTemplate(true); setTemplates(undefined); setSelectedNote(undefined); setDraft(undefined); setValidation(undefined);
    deliveryAPI.quickPlanTemplates().then((next) => {
      if (cancelled) return;
      setTemplates(next);
      setPlanSettings({ feed: settingsFromTemplate(next.feed), search: settingsFromTemplate(next.search) });
      setPlacement((current) => next[current].available ? current : next.feed.available ? "feed" : next.search.available ? "search" : current);
      setAudienceIDs({ feed: next.feed.audiences[0]?.id || "", search: next.search.audiences[0]?.id || "" });
    }).catch((error) => {
      if (!cancelled) setNotice({ tone: "error", message: errorMessage(error) });
    }).finally(() => { if (!cancelled) setLoadingTemplate(false); });
    return () => { cancelled = true; };
  }, [templateRefresh]);

  useEffect(() => {
    setSelectedNote(undefined); setDraft(undefined); setValidation(undefined);
  }, [advertiserID]);

  const loadNotes = useCallback(async () => {
    if (!advertiserID) return;
    setLoadingNotes(true);
    try {
      const result = await deliveryAPI.assets(advertiserID, assetSearch, 50);
      setNotes(result.notes); setNotice(null);
    } catch (error) {
      setNotes([]); setNotice({ tone: "error", message: errorMessage(error) });
    } finally { setLoadingNotes(false); }
  }, [advertiserID, assetSearch]);

  useEffect(() => { void loadNotes(); }, [advertiserID]); // eslint-disable-line react-hooks/exhaustive-deps

  const template = templates?.[placement];
  const settings = planSettings[placement];
  const audienceID = audienceIDs[placement];
  const selectedAudience = template?.audiences.find((item) => item.id === audienceID);
  const keywords = useMemo(() => parseKeywords(keywordText[placement]), [keywordText, placement]);
  const dayBudgetFen = settings ? parseFenInput(settings.dailyBudgetYuan) : undefined;
  const bidFen = settings ? parseFenInput(settings.bidYuan) : undefined;
  const settingsValid = Boolean(settings && dayBudgetFen !== undefined && dayBudgetFen >= 10_000 && dayBudgetFen < 99_999_900
    && bidFen !== undefined && bidFen < 99_999_900 && (placement === "feed" ? bidFen >= 0 : bidFen > 0));
  const canCreate = Boolean(template?.available && selectedNote && selectedAudience && settingsValid && (placement === "feed" || keywords.length > 0));

  const createDraft = async () => {
    if (!template || !settings || !selectedNote || !selectedAudience || dayBudgetFen === undefined || bidFen === undefined || !canCreate) return;
    setCreating(true); setNotice(null); setDraft(undefined); setValidation(undefined);
    try {
      const overrides: QuickPlanOverrides = {
        marketing_target: settings.marketingTarget,
        bidding_strategy: settings.biddingStrategy,
        day_budget_fen: dayBudgetFen,
        event_bid_fen: placement === "feed" ? bidFen : template.summary.event_bid_fen,
        pacing_mode: settings.pacingMode,
        time_period_type: settings.timePeriodType,
        keyword_bid_fen: placement === "search" ? bidFen : undefined,
        phrase_match_type: placement === "search" ? settings.phraseMatchType : undefined
      };
      const created = await deliveryAPI.createQuickPlanDraft({
        advertiser_id: advertiserID, placement, note_id: selectedNote.note_id,
        note_title: selectedNote.title, audience_id: selectedAudience.id,
        keywords, overrides, idempotency_key: requestKey.current
      });
      setDraft(created);
      try {
        const checked = await deliveryAPI.validate(created.id);
        setValidation(checked);
        setNotice({ tone: checked.valid ? "success" : "info", message: checked.valid ? "快速计划草稿已创建并通过校验" : "草稿已创建，请处理校验结果后再审批发布" });
      } catch (error) {
        setNotice({ tone: "info", message: `草稿已创建，自动校验未完成：${errorMessage(error)}` });
      }
      requestKey.current = createClientKey("quick-plan-ui");
    } catch (error) {
      setNotice({ tone: "error", message: errorMessage(error) });
    } finally { setCreating(false); }
  };

  const reset = () => {
    setSelectedNote(undefined); setDraft(undefined); setValidation(undefined); setNotice(null);
    setKeywordText((current) => ({ ...current, [placement]: "" }));
    requestKey.current = createClientKey("quick-plan-ui");
  };

  if (loadingSession) return <div className="quick-plan-page">
    <section className="page-heading qp-page-heading"><div><h1>快速新建计划</h1><p>基于全部广告主当前有效聚光投放配置</p></div></section>
    <div className="qp-bootstrap"><LoaderCircle size={20} className="spin" />加载广告主与全局模板</div>
  </div>;

  return <div className="quick-plan-page">
    <section className="page-heading qp-page-heading"><div><h1>快速新建计划</h1><p>基于全部广告主当前有效聚光投放配置</p></div></section>

    <section className="qp-context-bar">
      <label><span>广告主</span><div><Database size={16} /><select value={advertiserID || ""} onChange={(event) => setAdvertiserID(Number(event.target.value))} aria-label="选择广告主">
        <option value="" disabled>请选择广告主</option>{session?.advertisers.map((item) => <option value={item.advertiser_id} key={item.advertiser_id}>{item.advertiser_name || item.advertiser_id} · {item.advertiser_id}</option>)}
      </select></div></label>
      <nav className="qp-placement-switch" aria-label="计划场域">
        <button type="button" className={placement === "feed" ? "active" : ""} disabled={Boolean(templates && !templates.feed.available)} onClick={() => setPlacement("feed")}><Rows3 size={16} />信息流{templates ? <span>{templates.feed.sample_count}</span> : null}</button>
        <button type="button" className={placement === "search" ? "active" : ""} disabled={Boolean(templates && !templates.search.available)} onClick={() => setPlacement("search")}><Search size={16} />搜索{templates ? <span>{templates.search.sample_count}</span> : null}</button>
      </nav>
      <button type="button" className="qp-icon-button" disabled={loadingTemplate} onClick={() => setTemplateRefresh((current) => current + 1)} aria-label="刷新模板" title="刷新全局模板"><RefreshCw size={17} className={loadingTemplate ? "spin" : ""} /></button>
    </section>

    <PageNoticeLine value={notice} onClose={() => setNotice(null)} />
    {!session?.advertisers.length ? <section className="qp-unavailable"><AlertCircle size={20} /><div><strong>没有可用广告主</strong><p>请先检查聚光 OAuth 授权中的广告主范围。</p></div></section> : null}
    {loadingTemplate ? <div className="qp-bootstrap compact"><LoaderCircle size={19} className="spin" />统计在投模板</div> : null}
    {!loadingTemplate && template && !template.available ? <section className="qp-unavailable"><AlertCircle size={20} /><div><strong>当前场域没有可用的全局模板</strong><p>{template.unavailable_reason || "全部广告主中没有有效在投样本"}</p></div></section> : null}
    {!loadingTemplate && template?.available ? <>
      <TemplateEvidence template={template} />
      {settings ? <PlanSettingsEditor placement={placement} value={settings} onChange={(value) => setPlanSettings((current) => ({ ...current, [placement]: value }))} onReset={() => setPlanSettings((current) => ({ ...current, [placement]: settingsFromTemplate(template) }))} /> : null}
      <div className="qp-builder-grid">
        <NotePicker advertiserID={advertiserID} loading={loadingNotes} notes={notes} selected={selectedNote} search={assetSearch} onSearch={setAssetSearch} onSubmit={(event) => { event.preventDefault(); void loadNotes(); }} onSelect={setSelectedNote} />
        <section className="qp-tool-panel qp-configuration-panel">
          <AudienceInput template={template} selectedID={audienceID} onSelect={(value) => setAudienceIDs((current) => ({ ...current, [placement]: value }))} />
          <KeywordInput placement={placement} template={template} value={keywordText[placement]} onChange={(value) => setKeywordText((current) => ({ ...current, [placement]: value }))} />
        </section>
      </div>
      {!draft ? <section className="qp-submit-band">
        <div className="qp-selection-summary">
          <span><FileText size={15} />{selectedNote?.title || "未选择笔记"}</span>
          <span><Users size={15} />{selectedAudience?.name || "未选择人群"}</span>
          <span><Tags size={15} />{keywords.length} 个关键词</span>
        </div>
        <div className="qp-guardrail"><ShieldCheck size={16} /><span>创建后保持暂停，进入校验与双人审批</span></div>
        <button type="button" className="qp-primary-button qp-create-button" disabled={!canCreate || creating} onClick={() => void createDraft()}>{creating ? <LoaderCircle size={16} className="spin" /> : <CirclePlus size={16} />}生成并校验草稿</button>
      </section> : <ResultPanel draft={draft} validation={validation} advertiserID={advertiserID} onReset={reset} onEdit={() => navigate(`/self-serve-delivery?advertiser_id=${advertiserID}&draft=${encodeURIComponent(draft.id)}&view=editor`)} onOpen={() => navigate(`/self-serve-delivery?advertiser_id=${advertiserID}&draft=${encodeURIComponent(draft.id)}&view=review`)} />}
    </> : null}
  </div>;
}
