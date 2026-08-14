import {
  Bot, Braces, Check, CheckCircle2, ChevronDown, ChevronRight, ClipboardCheck, Copy, FilePlus2,
  FileText, Gauge, KeyRound, Layers3, Lightbulb, ListChecks, LoaderCircle, LockKeyhole, Pause,
  Play, Plus, RefreshCw, Rocket, Save, Search, ShieldCheck, Sparkles, Trash2, UsersRound, X
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent } from "react";
import {
  createClientKey, createDefaultDraftSpec, createDefaultUnit, deliveryAPI, type Actor, type Assets,
  type Capability, type Draft, type DraftSpec, type MediaEntity, type UnitSpec, type Workflow
} from "./delivery-api";
import {
  AlertLine, EmptyState, FenInput, formatDateTime, formatFen, JsonOutput, LoadingState, Notice,
  SectionTitle, StatusPill, errorMessage, type NoticeState
} from "./delivery-ui";

type Props = {
  advertiserID: number;
  actor: Actor;
  capability?: Capability;
};

type WorkspaceView = "editor" | "review" | "publish";
type EditorMode = "form" | "json";

const WRITE_ROLES = new Set(["operator", "admin"]);
const REVIEW_ROLES = new Set(["operator", "budget_owner", "admin"]);

function cloneSpec(spec: DraftSpec): DraftSpec {
  return structuredClone(spec);
}

function parseLines(value: string): string[] {
  return value.split(/\r?\n|,/).map((item) => item.trim()).filter(Boolean);
}

function keywordLines(unit: UnitSpec): string {
  return (unit.keywords || []).map((item) => `${item.keyword}|${(item.bid_fen / 100).toFixed(2)}|${item.phrase_match_type}`).join("\n");
}

function negativeLines(unit: UnitSpec): string {
  return (unit.negative_keywords || []).map((item) => `${item.keyword}|${item.phrase_match_type}`).join("\n");
}

function parseKeywordLines(value: string, fallbackBidFen: number) {
  return parseLines(value).map((line) => {
    const [keyword, bid, match] = line.split("|").map((item) => item.trim());
    return {
      keyword,
      bid_fen: bid ? Math.round((Number(bid) || 0) * 100) : fallbackBidFen,
      phrase_match_type: match ? Number(match) || 1 : 1
    };
  });
}

function parseNegativeLines(value: string) {
  return parseLines(value).map((line) => {
    const [keyword, match] = line.split("|").map((item) => item.trim());
    return { keyword, phrase_match_type: match ? Number(match) || 1 : 1 };
  });
}

function DraftList({ drafts, selectedID, loading, onSelect, onNew, canWrite }: {
  drafts: Draft[];
  selectedID: string | null;
  loading: boolean;
  onSelect: (id: string) => void;
  onNew: () => void;
  canWrite: boolean;
}) {
  return <aside className="dc-draft-list" aria-label="投放草稿列表">
    <header>
      <div><strong>投放草稿</strong><span>{drafts.length}</span></div>
      <button className="dc-icon-button" type="button" onClick={onNew} disabled={!canWrite} title="新建草稿" aria-label="新建草稿"><FilePlus2 size={17} /></button>
    </header>
    <div className="dc-draft-scroll">
      {loading ? <LoadingState label="加载草稿" /> : drafts.length === 0 ? <EmptyState icon={FileText} title="暂无草稿" action={canWrite ? <button className="dc-text-button" type="button" onClick={onNew}>新建草稿</button> : undefined} /> : drafts.map((draft) =>
        <button type="button" className={`dc-draft-row ${selectedID === draft.id ? "active" : ""}`} key={draft.id} onClick={() => onSelect(draft.id)}>
          <span className="dc-draft-row-top"><strong>{draft.spec.campaign.name || "未命名计划"}</strong><StatusPill value={draft.status} /></span>
          <span className="dc-draft-row-meta"><span>v{draft.current_version}</span><span>{formatFen(draft.spec.budget.total_limit_fen)}</span></span>
          <small>{formatDateTime(draft.updated_at)}</small>
        </button>)}
    </div>
  </aside>;
}

function GeneralFields({ spec, disabled, onChange }: { spec: DraftSpec; disabled: boolean; onChange: (mutate: (next: DraftSpec) => void) => void }) {
  return <section className="dc-editor-section">
    <SectionTitle icon={FileText} title="推广计划" meta="计划目标、投放位置与预算护栏" />
    <div className="dc-form-grid three">
      <label className="dc-field wide"><span>计划名称</span><input value={spec.campaign.name} disabled={disabled} onChange={(event) => onChange((next) => { next.campaign.name = event.target.value; })} /></label>
      <label className="dc-field"><span>业务目标</span><input value={spec.objective} disabled={disabled} onChange={(event) => onChange((next) => { next.objective = event.target.value; })} /></label>
      <label className="dc-field"><span>投放位置</span><select value={spec.placement} disabled={disabled} onChange={(event) => onChange((next) => {
        next.placement = event.target.value;
        next.campaign.placement = event.target.value === "feed" ? 1 : event.target.value === "search" ? 2 : 4;
      })}><option value="search">搜索</option><option value="feed">信息流</option><option value="all">全域</option></select></label>
      <label className="dc-field"><span>营销目标代码</span><input type="number" value={spec.campaign.marketing_target} disabled={disabled} onChange={(event) => onChange((next) => { next.campaign.marketing_target = Number(event.target.value) || 0; })} /></label>
      <label className="dc-field"><span>推广目标代码</span><input type="number" value={spec.campaign.promotion_target} disabled={disabled} onChange={(event) => onChange((next) => {
        const value = Number(event.target.value) || 0;
        next.campaign.promotion_target = value;
        next.units.forEach((unit) => { unit.promotion_target = value; });
      })} /></label>
      <label className="dc-field"><span>出价策略代码</span><input type="number" value={spec.campaign.bidding_strategy} disabled={disabled} onChange={(event) => onChange((next) => { next.campaign.bidding_strategy = Number(event.target.value) || 0; })} /></label>
      <label className="dc-field"><span>优化目标代码</span><input type="number" value={spec.campaign.optimize_target} disabled={disabled} onChange={(event) => onChange((next) => { next.campaign.optimize_target = Number(event.target.value) || 0; })} /></label>
    </div>
    <div className="dc-form-grid four dc-budget-grid">
      <label className="dc-field"><span>日预算</span><FenInput value={spec.budget.daily_limit_fen} disabled={disabled} onChange={(value) => onChange((next) => { next.budget.daily_limit_fen = value; next.campaign.day_budget_fen = value; })} /></label>
      <label className="dc-field"><span>总预算</span><FenInput value={spec.budget.total_limit_fen} disabled={disabled} onChange={(value) => onChange((next) => { next.budget.total_limit_fen = value; })} /></label>
      <label className="dc-field"><span>最高出价</span><FenInput value={spec.budget.max_bid_fen} disabled={disabled} onChange={(value) => onChange((next) => { next.budget.max_bid_fen = value; })} /></label>
      <label className="dc-field"><span>止损消耗</span><FenInput value={spec.budget.stop_loss_spend_fen} disabled={disabled} onChange={(value) => onChange((next) => { next.budget.stop_loss_spend_fen = value; })} /></label>
    </div>
  </section>;
}

function CreativityFields({ unitIndex, unit, disabled, onChange }: { unitIndex: number; unit: UnitSpec; disabled: boolean; onChange: (mutate: (unit: UnitSpec) => void) => void }) {
  return <div className="dc-subsection">
    <div className="dc-subsection-heading"><strong>创意</strong><button className="dc-icon-button small" type="button" disabled={disabled} title="添加创意" aria-label={`为单元 ${unitIndex + 1} 添加创意`} onClick={() => onChange((next) => {
      const number = next.creativities.length + 1;
      next.creativities.push({ local_key: `creative-${crypto.randomUUID()}`, name: `创意 ${unitIndex + 1}-${number}`, note_id: "" });
    })}><Plus size={15} /></button></div>
    <div className="dc-creativity-list">
      {unit.creativities.map((creative, creativeIndex) => <div className="dc-creativity-row" key={`${creative.local_key}-${creativeIndex}`}>
        <label className="dc-field"><span>创意名称</span><input value={creative.name} disabled={disabled} onChange={(event) => onChange((next) => { next.creativities[creativeIndex].name = event.target.value; })} /></label>
        <label className="dc-field grow"><span>笔记 ID</span><input value={creative.note_id} disabled={disabled} placeholder="24 位笔记 ID" onChange={(event) => onChange((next) => {
          next.creativities[creativeIndex].note_id = event.target.value.trim();
          next.note_ids = Array.from(new Set(next.creativities.map((item) => item.note_id).filter(Boolean)));
        })} /></label>
        <button className="dc-icon-button danger align-end" type="button" disabled={disabled || unit.creativities.length === 1} title="删除创意" aria-label={`删除创意 ${creativeIndex + 1}`} onClick={() => onChange((next) => {
          next.creativities.splice(creativeIndex, 1);
          next.note_ids = Array.from(new Set(next.creativities.map((item) => item.note_id).filter(Boolean)));
        })}><Trash2 size={16} /></button>
      </div>)}
    </div>
  </div>;
}

function UnitFields({ unit, index, disabled, canRemove, onUnitChange, onRemove }: {
  unit: UnitSpec;
  index: number;
  disabled: boolean;
  canRemove: boolean;
  onUnitChange: (mutate: (unit: UnitSpec) => void) => void;
  onRemove: () => void;
}) {
  const [open, setOpen] = useState(true);
  const [keywordText, setKeywordText] = useState(() => keywordLines(unit));
  const [negativeText, setNegativeText] = useState(() => negativeLines(unit));
  useEffect(() => { setKeywordText(keywordLines(unit)); }, [unit.keywords]);
  useEffect(() => { setNegativeText(negativeLines(unit)); }, [unit.negative_keywords]);
  return <article className="dc-unit-editor">
    <header className="dc-unit-header">
      <button className="dc-collapse-button" type="button" onClick={() => setOpen((value) => !value)} aria-expanded={open}>
        {open ? <ChevronDown size={17} /> : <ChevronRight size={17} />}<span><strong>{unit.name || `广告单元 ${index + 1}`}</strong><small>{unit.creativities.length} 个创意 · {(unit.keywords || []).length} 个关键词</small></span>
      </button>
      <button className="dc-icon-button danger" type="button" disabled={disabled || !canRemove} title="删除单元" aria-label={`删除广告单元 ${index + 1}`} onClick={onRemove}><Trash2 size={16} /></button>
    </header>
    {open ? <div className="dc-unit-body">
      <div className="dc-form-grid four">
        <label className="dc-field wide"><span>单元名称</span><input value={unit.name} disabled={disabled} onChange={(event) => onUnitChange((next) => { next.name = event.target.value; })} /></label>
        <label className="dc-field"><span>目标出价</span><FenInput value={unit.event_bid_fen} disabled={disabled} onChange={(value) => onUnitChange((next) => { next.event_bid_fen = value; })} /></label>
        <label className="dc-field"><span>定向类型代码</span><input type="number" value={unit.target_type} disabled={disabled} onChange={(event) => onUnitChange((next) => { next.target_type = Number(event.target.value) || 0; })} /></label>
        <label className="dc-field"><span>性别</span><select value={unit.target.gender || "all"} disabled={disabled} onChange={(event) => onUnitChange((next) => { next.target.gender = event.target.value; })}><option value="all">不限</option><option value="0">男</option><option value="1">女</option></select></label>
        <label className="dc-field"><span>年龄</span><select value={unit.target.age || "all"} disabled={disabled} onChange={(event) => onUnitChange((next) => { next.target.age = event.target.value; })}><option value="all">不限</option><option value="18-22">18-22</option><option value="23-27">23-27</option><option value="28-32">28-32</option><option value="32-100">32+</option></select></label>
        <label className="dc-field"><span>设备</span><select value={unit.target.device || "all"} disabled={disabled} onChange={(event) => onUnitChange((next) => { next.target.device = event.target.value; })}><option value="all">不限</option><option value="ios">iOS</option><option value="android">Android</option></select></label>
        <label className="dc-check-field"><input type="checkbox" checked={(unit.target.intelligent_expansion || 0) === 1} disabled={disabled} onChange={(event) => onUnitChange((next) => { next.target.intelligent_expansion = event.target.checked ? 1 : 0; })} /><span>智能拓量</span></label>
      </div>
      <div className="dc-form-grid two dc-keyword-grid">
        <label className="dc-field"><span>关键词 <small>每行：词|出价元|匹配代码</small></span><textarea rows={6} value={keywordText} disabled={disabled} placeholder="辅酶Q10|12.00|1" onChange={(event) => setKeywordText(event.target.value)} onBlur={() => onUnitChange((next) => { next.keywords = parseKeywordLines(keywordText, next.event_bid_fen || 1000); })} /></label>
        <label className="dc-field"><span>否定关键词 <small>每行：词|匹配代码</small></span><textarea rows={6} value={negativeText} disabled={disabled} placeholder="免费|1" onChange={(event) => setNegativeText(event.target.value)} onBlur={() => onUnitChange((next) => { next.negative_keywords = parseNegativeLines(negativeText); })} /></label>
      </div>
      <CreativityFields unitIndex={index} unit={unit} disabled={disabled} onChange={onUnitChange} />
    </div> : null}
  </article>;
}

function ExperimentFields({ spec, disabled, onChange }: { spec: DraftSpec; disabled: boolean; onChange: (mutate: (next: DraftSpec) => void) => void }) {
  return <section className="dc-editor-section">
    <SectionTitle icon={Gauge} title="实验与护栏" meta="单变量实验、主要指标与止损条件" />
    <div className="dc-form-grid four">
      <label className="dc-field"><span>主要指标</span><input value={spec.experiment.primary_metric} disabled={disabled} onChange={(event) => onChange((next) => { next.experiment.primary_metric = event.target.value; })} /></label>
      <label className="dc-field"><span>保护指标</span><input value={(spec.experiment.guardrails || []).join(", ")} disabled={disabled} onChange={(event) => onChange((next) => { next.experiment.guardrails = parseLines(event.target.value); })} /></label>
      <label className="dc-field"><span>实验变量</span><input value={(spec.experiment.variables || []).join(", ")} disabled={disabled} onChange={(event) => onChange((next) => { next.experiment.variables = parseLines(event.target.value); })} /></label>
      <label className="dc-field"><span>固定变量</span><input value={(spec.experiment.hold_constant || []).join(", ")} disabled={disabled} onChange={(event) => onChange((next) => { next.experiment.hold_constant = parseLines(event.target.value); })} /></label>
    </div>
  </section>;
}

function AssetPicker({ advertiserID, disabled, onPick }: { advertiserID: number; disabled: boolean; onPick: (noteID: string, title: string) => void }) {
  const [search, setSearch] = useState("");
  const [assets, setAssets] = useState<Assets>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    setLoading(true); setError("");
    try { setAssets(await deliveryAPI.assets(advertiserID, search, 20)); }
    catch (err) { setError(errorMessage(err)); }
    finally { setLoading(false); }
  }, [advertiserID, search]);
  useEffect(() => { void load(); }, [advertiserID]); // eslint-disable-line react-hooks/exhaustive-deps
  return <section className="dc-editor-section dc-asset-picker">
    <SectionTitle icon={Lightbulb} title="稿件候选" meta="选择稿件后加入第一个广告单元" actions={<form className="dc-inline-search" onSubmit={(event) => { event.preventDefault(); void load(); }}><Search size={15} /><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索稿件" aria-label="搜索稿件" /><button type="submit" aria-label="执行搜索"><RefreshCw size={14} /></button></form>} />
    {error ? <AlertLine>{error}</AlertLine> : null}
    <div className="dc-asset-strip">
      {loading ? <LoadingState label="加载稿件" /> : assets?.notes.length ? assets.notes.map((note) => <article key={note.note_id}>
        <div><strong>{note.title || "未命名稿件"}</strong><code>{note.note_id}</code></div>
        <span>{note.historical_search_users} 回搜 · {note.creativity_count} 创意</span>
        <button className="dc-icon-button" type="button" disabled={disabled} title="加入第一个广告单元" aria-label={`加入稿件 ${note.title}`} onClick={() => onPick(note.note_id, note.title)}><Plus size={16} /></button>
      </article>) : <EmptyState icon={Lightbulb} title="没有匹配稿件" />}
    </div>
  </section>;
}

function DraftEditor({ draft, spec, setSpec, mode, setMode, jsonText, setJSONText, canWrite, saving, dirty, changeReason, setChangeReason, onSave, onReset, advertiserID }: {
  draft?: Draft;
  spec: DraftSpec;
  setSpec: (spec: DraftSpec) => void;
  mode: EditorMode;
  setMode: (mode: EditorMode) => void;
  jsonText: string;
  setJSONText: (value: string) => void;
  canWrite: boolean;
  saving: boolean;
  dirty: boolean;
  changeReason: string;
  setChangeReason: (value: string) => void;
  onSave: () => void;
  onReset: () => void;
  advertiserID: number;
}) {
  const mutate = (fn: (next: DraftSpec) => void) => {
    const next = cloneSpec(spec);
    fn(next);
    next.units.forEach((unit) => {
      unit.note_ids = Array.from(new Set(unit.creativities.map((creative) => creative.note_id.trim()).filter(Boolean)));
    });
    next.notes = Array.from(new Set(next.units.flatMap((unit) => unit.note_ids || [])));
    setSpec(next);
  };
  const applyJSON = () => {
    const parsed = JSON.parse(jsonText) as DraftSpec;
    if (!parsed || typeof parsed !== "object" || !Array.isArray(parsed.units)) throw new Error("JSON 必须是有效的 DraftSpec 对象");
    parsed.advertiser_id = advertiserID;
    setSpec(parsed);
  };
  const showForm = () => {
    if (mode === "json" && canWrite) {
      try { applyJSON(); }
      catch (error) { window.alert(errorMessage(error)); return; }
    }
    setMode("form");
  };
  const addNote = (noteID: string, title: string) => mutate((next) => {
    const unit = next.units[0] || createDefaultUnit(1);
    if (!next.units.length) next.units.push(unit);
    unit.note_ids = Array.from(new Set([...(unit.note_ids || []), noteID]));
    if (!unit.creativities.some((creative) => creative.note_id === noteID)) {
      const empty = unit.creativities.find((creative) => !creative.note_id);
      if (empty) {
        empty.name = title.slice(0, 50) || empty.name;
        empty.note_id = noteID;
      } else {
        const number = unit.creativities.length + 1;
        unit.creativities.push({ local_key: `creative-${crypto.randomUUID()}`, name: title.slice(0, 50) || `创意 1-${number}`, note_id: noteID });
      }
    }
    next.notes = Array.from(new Set([...(next.notes || []), noteID]));
  });
  return <div className="dc-editor">
    <div className="dc-editor-toolbar">
      <div className="dc-segmented" role="group" aria-label="草稿编辑模式">
        <button type="button" className={mode === "form" ? "active" : ""} onClick={showForm}><ListChecks size={15} />表单</button>
        <button type="button" className={mode === "json" ? "active" : ""} onClick={() => { if (mode !== "json") setJSONText(JSON.stringify(spec, null, 2)); setMode("json"); }}><Braces size={15} />JSON</button>
      </div>
      <span className={`dc-dirty-state ${dirty ? "dirty" : ""}`}>{dirty ? "有未保存修改" : "已同步"}</span>
    </div>
    {mode === "form" ? <>
      <GeneralFields spec={spec} disabled={!canWrite} onChange={mutate} />
      <section className="dc-editor-section">
        <SectionTitle icon={Layers3} title="广告单元、定向与创意" meta={`${spec.units.length} 个广告单元`} actions={<button className="dc-action-button secondary" type="button" disabled={!canWrite || spec.units.length >= 100} onClick={() => mutate((next) => {
          const unit = createDefaultUnit(next.units.length + 1);
          unit.local_key = `unit-${crypto.randomUUID()}`;
          unit.creativities[0].local_key = `creative-${crypto.randomUUID()}`;
          next.units.push(unit);
        })}><Plus size={15} />添加单元</button>} />
        <div className="dc-unit-list">{spec.units.map((unit, index) => <UnitFields key={`${unit.local_key}-${index}`} unit={unit} index={index} disabled={!canWrite} canRemove={spec.units.length > 1} onUnitChange={(fn) => mutate((next) => fn(next.units[index]))} onRemove={() => mutate((next) => { next.units.splice(index, 1); })} />)}</div>
      </section>
      <ExperimentFields spec={spec} disabled={!canWrite} onChange={mutate} />
      <AssetPicker advertiserID={advertiserID} disabled={!canWrite} onPick={addNote} />
    </> : <section className="dc-editor-section">
      <SectionTitle icon={Braces} title="DraftSpec JSON" meta="高级媒体字段按 OpenAPI 契约提交" actions={<button type="button" className="dc-action-button secondary" disabled={!canWrite} onClick={() => {
        try { applyJSON(); } catch (error) { window.alert(errorMessage(error)); }
      }}><Check size={15} />应用 JSON</button>} />
      <textarea className="dc-json-editor" value={jsonText} disabled={!canWrite} spellCheck={false} onChange={(event) => setJSONText(event.target.value)} aria-label="DraftSpec JSON" />
    </section>}
    <footer className="dc-save-bar">
      <label className="dc-field grow"><span>变更原因</span><input value={changeReason} disabled={!canWrite} placeholder={draft ? "本次修改原因" : "新建投放方案"} onChange={(event) => setChangeReason(event.target.value)} /></label>
      <button className="dc-action-button ghost" type="button" disabled={!dirty || saving} onClick={onReset}><X size={15} />撤销</button>
      <button className="dc-action-button primary" type="button" disabled={!canWrite || !dirty || saving || (Boolean(draft) && !changeReason.trim())} onClick={onSave}>{saving ? <LoaderCircle size={15} className="spin" /> : <Save size={15} />}{draft ? "保存新版本" : "创建草稿"}</button>
    </footer>
  </div>;
}

function RecommendationPanel({ workflow, running, onRecommend, canRun }: { workflow: Workflow; running: boolean; onRecommend: () => void; canRun: boolean }) {
  const recommendation = workflow.recommendation;
  const ranked = Array.isArray(recommendation?.payload.ranked_notes) ? recommendation?.payload.ranked_notes as Array<Record<string, unknown>> : [];
  const themes = Array.isArray(recommendation?.payload.themes) ? recommendation?.payload.themes as string[] : [];
  const keywords = Array.isArray(recommendation?.payload.keyword_seeds) ? recommendation?.payload.keyword_seeds as string[] : [];
  return <section className="dc-review-section">
    <SectionTitle icon={Sparkles} title="算法建议" meta={recommendation ? `${recommendation.llm_provider}/${recommendation.llm_model} · ${recommendation.ranker_family}/${recommendation.ranker_version}` : "当前版本尚未生成"} actions={<button type="button" className="dc-action-button secondary" disabled={!canRun || running} onClick={onRecommend}>{running ? <LoaderCircle size={15} className="spin" /> : <Bot size={15} />}{recommendation ? "重新生成" : "生成建议"}</button>} />
    {!recommendation ? <EmptyState icon={Bot} title="暂无建议" /> : <div className="dc-recommendation-grid">
      <div className="dc-recommendation-block"><strong>素材排序</strong>{ranked.length ? ranked.map((item, index) => <div className="dc-rank-row" key={String(item.note_id || index)}><span>{index + 1}</span><code>{String(item.note_id || "-")}</code><b>{typeof item.score === "number" ? item.score.toFixed(3) : "-"}</b></div>) : <span className="dc-muted">无候选结果</span>}</div>
      <div className="dc-recommendation-block"><strong>主题</strong><div className="dc-chip-list">{themes.map((item) => <span key={item}>{item}</span>)}</div><strong>关键词种子</strong><div className="dc-chip-list">{keywords.map((item) => <span key={item}>{item}</span>)}</div></div>
      <div className="dc-recommendation-block"><strong>不确定性</strong>{recommendation.warnings.length ? <ul>{recommendation.warnings.map((warning) => <li key={warning}>{warning}</li>)}</ul> : <span className="dc-muted">无额外警告</span>}<details><summary>完整建议</summary><JsonOutput value={recommendation.payload} /></details></div>
    </div>}
  </section>;
}

function ValidationPanel({ workflow, running, onValidate, canRun }: { workflow: Workflow; running: boolean; onValidate: () => void; canRun: boolean }) {
  const validation = workflow.validation;
  return <section className="dc-review-section">
    <SectionTitle icon={ShieldCheck} title="确定性校验" meta={validation ? `有效至 ${formatDateTime(validation.valid_until)}` : "当前版本尚未校验"} actions={<button type="button" className="dc-action-button secondary" disabled={!canRun || running} onClick={onValidate}>{running ? <LoaderCircle size={15} className="spin" /> : <ClipboardCheck size={15} />}{validation ? "重新校验" : "执行校验"}</button>} />
    {!validation ? <EmptyState icon={ClipboardCheck} title="暂无校验结果" /> : <>
      <div className={`dc-validation-summary ${validation.valid ? "valid" : "invalid"}`}>{validation.valid ? <CheckCircle2 size={20} /> : <X size={20} />}<div><strong>{validation.valid ? "当前版本通过校验" : `${validation.errors.length} 项错误阻止发布`}</strong><span>{validation.warnings.length} 项警告 · {validation.rules_version}</span></div></div>
      <div className="dc-issue-list">{[...validation.errors, ...validation.warnings].map((issue, index) => <div className={`dc-issue-row ${issue.severity}`} key={`${issue.code}-${index}`}><StatusPill value={issue.severity} /><code>{issue.path}</code><div><strong>{issue.code}</strong><span>{issue.message}</span></div></div>)}</div>
    </>}
  </section>;
}

function ApprovalPanel({ workflow, actor, submitting, onSubmit }: { workflow: Workflow; actor: Actor; submitting: boolean; onSubmit: (input: { role: "operator" | "budget_owner"; decision: "approved" | "rejected"; comment: string; approved_budget_fen: number; expires_in_minutes: number }) => void }) {
  const initialRole = actor.role === "budget_owner" ? "budget_owner" : "operator";
  const [role, setRole] = useState<"operator" | "budget_owner">(initialRole);
  const [decision, setDecision] = useState<"approved" | "rejected">("approved");
  const [comment, setComment] = useState("");
  const [budget, setBudget] = useState(workflow.draft.spec.budget.total_limit_fen);
  const [expires, setExpires] = useState(60);
  useEffect(() => { setBudget(workflow.draft.spec.budget.total_limit_fen); }, [workflow.draft.id, workflow.draft.current_version, workflow.draft.spec.budget.total_limit_fen]);
  const canApprove = REVIEW_ROLES.has(actor.role) && actor.id !== workflow.draft.updated_by;
  return <section className="dc-review-section">
    <SectionTitle icon={UsersRound} title="双人审批" meta="运营与预算负责人分别批准当前版本" />
    <div className="dc-approval-list">{workflow.approvals.length ? workflow.approvals.map((approval) => <article key={approval.id}>
      <div><StatusPill value={approval.role} /><StatusPill value={approval.decision} /></div>
      <strong>{approval.actor}</strong><span>{formatFen(approval.approved_budget_fen)} · 至 {formatDateTime(approval.expires_at)}</span>
      {approval.comment ? <p>{approval.comment}</p> : null}
    </article>) : <EmptyState icon={UsersRound} title="尚无审批" />}</div>
    <form className="dc-approval-form" onSubmit={(event) => { event.preventDefault(); onSubmit({ role, decision, comment, approved_budget_fen: budget, expires_in_minutes: expires }); }}>
      <label className="dc-field"><span>审批职责</span><select value={role} disabled={!canApprove || actor.role !== "admin"} onChange={(event) => setRole(event.target.value as "operator" | "budget_owner")}><option value="operator">运营审批</option><option value="budget_owner">预算审批</option></select></label>
      <label className="dc-field"><span>决定</span><select value={decision} disabled={!canApprove} onChange={(event) => setDecision(event.target.value as "approved" | "rejected")}><option value="approved">批准</option><option value="rejected">拒绝</option></select></label>
      <label className="dc-field"><span>批准金额</span><FenInput value={budget} disabled={!canApprove} onChange={setBudget} /></label>
      <label className="dc-field"><span>有效分钟</span><input type="number" min={5} max={1440} value={expires} disabled={!canApprove} onChange={(event) => setExpires(Number(event.target.value) || 60)} /></label>
      <label className="dc-field grow"><span>审批意见</span><input value={comment} disabled={!canApprove} onChange={(event) => setComment(event.target.value)} /></label>
      <button className="dc-action-button primary align-end" type="submit" disabled={!canApprove || submitting}>{submitting ? <LoaderCircle size={15} className="spin" /> : <Check size={15} />}提交审批</button>
    </form>
    {!canApprove ? <AlertLine>{actor.id === workflow.draft.updated_by ? "当前身份是该版本提交人，需切换独立审批身份" : "当前身份没有审批权限"}</AlertLine> : null}
  </section>;
}

function JobList({ workflow }: { workflow: Workflow }) {
  return <section className="dc-publish-section">
    <SectionTitle icon={Rocket} title="发布作业" meta={`${workflow.jobs.length} 条当前版本记录`} />
    {workflow.jobs.length ? <div className="dc-job-list">{workflow.jobs.map((job) => <details key={job.id} className="dc-job-row">
      <summary><span><StatusPill value={job.mode} /><StatusPill value={job.status} /></span><strong>{job.current_step || "准备"}</strong><code>{job.id}</code><time>{formatDateTime(job.updated_at)}</time></summary>
      {job.error_message ? <AlertLine>{job.error_code ? `${job.error_code}: ` : ""}{job.error_message}</AlertLine> : null}
      <div className="dc-job-detail"><div><strong>请求预览</strong><JsonOutput value={job.request_preview} /></div><div><strong>执行结果</strong><JsonOutput value={job.result} /></div></div>
    </details>)}</div> : <EmptyState icon={Rocket} title="暂无发布作业" />}
  </section>;
}

function EntityList({ workflow, capability, actor, updating, onStatus }: { workflow: Workflow; capability?: Capability; actor: Actor; updating: string; onStatus: (entity: MediaEntity, status: "paused" | "active") => void }) {
  const canUpdate = WRITE_ROLES.has(actor.role) && Boolean(capability?.media_writes_enabled);
  return <section className="dc-publish-section">
    <SectionTitle icon={Layers3} title="媒体实体" meta={`${workflow.entities.length} 个读回映射`} />
    {workflow.entities.length ? <div className="dc-table-wrap"><table className="dc-table"><thead><tr><th>层级</th><th>本地键</th><th>媒体 ID</th><th>目标状态</th><th>读回状态</th><th>操作</th></tr></thead><tbody>{workflow.entities.map((entity) => <tr key={entity.id}><td><StatusPill value={entity.entity_type} /></td><td>{entity.local_key}</td><td><code>{entity.media_id}</code></td><td><StatusPill value={entity.desired_status} /></td><td><StatusPill value={entity.observed_status} /></td><td><div className="dc-table-actions"><button className="dc-icon-button" title="暂停" aria-label={`暂停 ${entity.local_key}`} disabled={!canUpdate || updating === entity.id} onClick={() => onStatus(entity, "paused")}><Pause size={15} /></button><button className="dc-icon-button" title="启用" aria-label={`启用 ${entity.local_key}`} disabled={!canUpdate || updating === entity.id} onClick={() => onStatus(entity, "active")}><Play size={15} /></button></div></td></tr>)}</tbody></table></div> : <EmptyState icon={Layers3} title="暂无媒体实体" />}
    {!capability?.media_writes_enabled ? <AlertLine>全局媒体写开关关闭，实体启停不可用</AlertLine> : null}
  </section>;
}

export default function DeliveryDraftWorkspace({ advertiserID, actor, capability }: Props) {
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [workflow, setWorkflow] = useState<Workflow>();
  const [spec, setSpec] = useState(() => createDefaultDraftSpec(advertiserID));
  const [baseline, setBaseline] = useState(() => JSON.stringify(createDefaultDraftSpec(advertiserID)));
  const [view, setView] = useState<WorkspaceView>("editor");
  const [mode, setMode] = useState<EditorMode>("form");
  const [jsonText, setJSONText] = useState("");
  const [changeReason, setChangeReason] = useState("");
  const [loadingDrafts, setLoadingDrafts] = useState(true);
  const [loadingWorkflow, setLoadingWorkflow] = useState(false);
  const [action, setAction] = useState("");
  const [notice, setNotice] = useState<NoticeState>(null);
  const [entityUpdating, setEntityUpdating] = useState("");
  const createKey = useRef(createClientKey("draft-ui"));
  const publishKeys = useRef({ scope: "", dry_run: createClientKey("publish-dry-run-ui"), execute: createClientKey("publish-execute-ui") });
  const canWrite = WRITE_ROLES.has(actor.role);

  const loadDrafts = useCallback(async (preferredID?: string | null) => {
    setLoadingDrafts(true);
    try {
      const result = await deliveryAPI.drafts(advertiserID);
      setDrafts(result.items);
      const target = preferredID === null ? null : preferredID || result.items[0]?.id || null;
      setSelectedID(target);
      return target;
    } catch (error) {
      setNotice({ tone: "error", message: errorMessage(error) });
      return null;
    } finally { setLoadingDrafts(false); }
  }, [advertiserID]);

  const loadWorkflow = useCallback(async (draftID: string, preserveEditor = false) => {
    setLoadingWorkflow(true);
    try {
      const result = await deliveryAPI.workflow(draftID);
      setWorkflow(result);
      if (!preserveEditor) {
        setSpec(cloneSpec(result.draft.spec));
        setBaseline(JSON.stringify(result.draft.spec));
        setJSONText(JSON.stringify(result.draft.spec, null, 2));
      }
      return result;
    } catch (error) {
      setNotice({ tone: "error", message: errorMessage(error) });
    } finally { setLoadingWorkflow(false); }
  }, []);

  useEffect(() => {
    setWorkflow(undefined); setView("editor"); setMode("form"); setChangeReason("");
    const empty = createDefaultDraftSpec(advertiserID);
    setSpec(empty); setBaseline(JSON.stringify(empty));
    void loadDrafts();
  }, [advertiserID, loadDrafts]);

  useEffect(() => {
    if (selectedID) void loadWorkflow(selectedID);
  }, [selectedID, loadWorkflow]);

  useEffect(() => {
    if (!workflow?.jobs.some((job) => job.status === "queued" || job.status === "publishing")) return;
    const timer = window.setInterval(() => { if (selectedID) void loadWorkflow(selectedID, true); }, 3000);
    return () => window.clearInterval(timer);
  }, [workflow?.jobs, selectedID, loadWorkflow]);

  useEffect(() => {
    const scope = workflow ? `${workflow.draft.id}:${workflow.draft.current_version}` : "new";
    if (publishKeys.current.scope !== scope) {
      publishKeys.current = {
        scope,
        dry_run: createClientKey("publish-dry-run-ui"),
        execute: createClientKey("publish-execute-ui")
      };
    }
  }, [workflow?.draft.id, workflow?.draft.current_version]);

  const changed = useMemo(() => {
    if (mode !== "json") return JSON.stringify(spec) !== baseline;
    try { return JSON.stringify(JSON.parse(jsonText)) !== baseline; }
    catch { return true; }
  }, [spec, baseline, mode, jsonText]);
  const dirty = !workflow || changed;
  const selectDraft = (id: string) => {
    if (changed && !window.confirm("当前修改尚未保存，确定切换草稿吗？")) return;
    setWorkflow(undefined); setSelectedID(id); setView("editor"); setChangeReason("");
  };
  const newDraft = () => {
    if (changed && !window.confirm("当前修改尚未保存，确定新建草稿吗？")) return;
    const next = createDefaultDraftSpec(advertiserID);
    setSelectedID(null); setWorkflow(undefined); setSpec(next); setBaseline(JSON.stringify(next)); setJSONText(JSON.stringify(next, null, 2));
    setChangeReason("新建投放方案"); setView("editor"); setMode("form"); createKey.current = createClientKey("draft-ui");
  };
  const reset = () => {
    const next = workflow?.draft.spec || createDefaultDraftSpec(advertiserID);
    setSpec(cloneSpec(next)); setJSONText(JSON.stringify(next, null, 2));
  };
  const save = async () => {
    setAction("save"); setNotice(null);
    try {
      let finalSpec = spec;
      if (mode === "json") {
        finalSpec = JSON.parse(jsonText) as DraftSpec;
        finalSpec.advertiser_id = advertiserID;
        setSpec(finalSpec);
      }
      const saved = workflow
        ? await deliveryAPI.updateDraft(workflow.draft.id, finalSpec, workflow.draft.current_version, changeReason.trim())
        : await deliveryAPI.createDraft(finalSpec, createKey.current, changeReason.trim() || "新建投放方案");
      setNotice({ tone: "success", message: workflow ? `已保存版本 v${saved.current_version}` : "草稿已创建" });
      setChangeReason(""); setSelectedID(saved.id);
      await Promise.all([loadDrafts(saved.id), loadWorkflow(saved.id)]);
    } catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setAction(""); }
  };
  const runWorkflowAction = async (kind: "recommend" | "validate") => {
    if (!workflow) return;
    setAction(kind); setNotice(null);
    try {
      if (kind === "recommend") await deliveryAPI.recommend(workflow.draft.id);
      else await deliveryAPI.validate(workflow.draft.id);
      await loadWorkflow(workflow.draft.id);
      setNotice({ tone: "success", message: kind === "recommend" ? "算法建议已生成" : "校验已完成" });
    } catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setAction(""); }
  };
  const approve = async (input: Parameters<typeof deliveryAPI.approve>[1]) => {
    if (!workflow) return;
    setAction("approve"); setNotice(null);
    try { await deliveryAPI.approve(workflow.draft.id, input); await loadWorkflow(workflow.draft.id); setNotice({ tone: "success", message: "审批已记录" }); }
    catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setAction(""); }
  };
  const publish = async (modeValue: "dry_run" | "execute") => {
    if (!workflow) return;
    if (modeValue === "execute" && !window.confirm("确认提交真实发布？新计划将以暂停态创建。")) return;
    setAction(modeValue); setNotice(null);
    try {
      const job = await deliveryAPI.publish(workflow.draft.id, modeValue, publishKeys.current[modeValue]);
      await loadWorkflow(workflow.draft.id);
      setNotice({ tone: "success", message: modeValue === "dry_run" ? "发布演练已生成" : `发布作业 ${job.id} 已提交` });
    } catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setAction(""); }
  };
  const updateStatus = async (entity: MediaEntity, status: "paused" | "active") => {
    setEntityUpdating(entity.id); setNotice(null);
    try { await deliveryAPI.updateEntityStatus(entity, status); if (workflow) await loadWorkflow(workflow.draft.id); setNotice({ tone: "success", message: `${entity.local_key} 已${status === "paused" ? "暂停" : "启用"}` }); }
    catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setEntityUpdating(""); }
  };

  return <div className="dc-workspace-layout">
    <DraftList drafts={drafts} selectedID={selectedID} loading={loadingDrafts} onSelect={selectDraft} onNew={newDraft} canWrite={canWrite} />
    <div className="dc-workspace-main">
      <Notice state={notice} onDismiss={() => setNotice(null)} />
      <header className="dc-workspace-header">
        <div><h2>{workflow?.draft.spec.campaign.name || "新建投放草稿"}</h2><p>{workflow ? <><code>{workflow.draft.id}</code> · 版本 {workflow.draft.current_version} · <StatusPill value={workflow.draft.status} /></> : "尚未写入媒体"}</p></div>
        <button className="dc-icon-button" type="button" disabled={!selectedID || loadingWorkflow} onClick={() => { if (selectedID && (!changed || window.confirm("刷新会丢弃未保存修改，确定继续吗？"))) void loadWorkflow(selectedID); }} title="刷新工作流" aria-label="刷新工作流"><RefreshCw size={17} className={loadingWorkflow ? "spin" : ""} /></button>
      </header>
      {workflow ? <nav className="dc-subnav" aria-label="草稿工作流">
        <button className={view === "editor" ? "active" : ""} onClick={() => setView("editor")}><FileText size={16} />配置</button>
        <button className={view === "review" ? "active" : ""} onClick={() => setView("review")}><ClipboardCheck size={16} />校验与审批</button>
        <button className={view === "publish" ? "active" : ""} onClick={() => setView("publish")}><Rocket size={16} />发布与实体</button>
      </nav> : null}
      {loadingWorkflow && workflow === undefined && selectedID ? <LoadingState label="加载工作流" /> : null}
      {view === "editor" && (!selectedID || workflow) ? <DraftEditor draft={workflow?.draft} spec={spec} setSpec={setSpec} mode={mode} setMode={setMode} jsonText={jsonText} setJSONText={setJSONText} canWrite={canWrite} saving={action === "save"} dirty={dirty} changeReason={changeReason} setChangeReason={setChangeReason} onSave={() => void save()} onReset={reset} advertiserID={advertiserID} /> : null}
      {view === "review" && workflow ? <div className="dc-review-stack">
        <RecommendationPanel workflow={workflow} running={action === "recommend"} onRecommend={() => void runWorkflowAction("recommend")} canRun={canWrite} />
        <ValidationPanel workflow={workflow} running={action === "validate"} onValidate={() => void runWorkflowAction("validate")} canRun={canWrite} />
        <ApprovalPanel workflow={workflow} actor={actor} submitting={action === "approve"} onSubmit={(input) => void approve(input)} />
      </div> : null}
      {view === "publish" && workflow ? <div className="dc-publish-stack">
        <section className="dc-publish-controls">
          <div><Rocket size={20} /><div><strong>发布控制</strong><span>先执行演练；真实发布要求有效校验与双人审批</span></div></div>
          <div><button className="dc-action-button secondary" disabled={!canWrite || action !== ""} onClick={() => void publish("dry_run")}><Gauge size={15} />发布演练</button><button className="dc-action-button danger" disabled={!canWrite || !capability?.media_writes_enabled || action !== ""} title={!capability?.media_writes_enabled ? "媒体写开关关闭" : "真实发布"} onClick={() => void publish("execute")}>{capability?.media_writes_enabled ? <Rocket size={15} /> : <LockKeyhole size={15} />}真实发布</button></div>
        </section>
        {!capability?.media_writes_enabled ? <AlertLine>真实媒体写入保持关闭；发布演练、审批和读回仍可使用</AlertLine> : null}
        <JobList workflow={workflow} />
        <EntityList workflow={workflow} capability={capability} actor={actor} updating={entityUpdating} onStatus={(entity, status) => void updateStatus(entity, status)} />
      </div> : null}
    </div>
  </div>;
}
