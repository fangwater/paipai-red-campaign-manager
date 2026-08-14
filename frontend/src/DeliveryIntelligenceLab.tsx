import {
  Activity, BarChart3, Bot, BrainCircuit, CheckCircle2, FlaskConical, Gauge, Layers3, LoaderCircle,
  LockKeyhole, Plus, RefreshCw, Scale, ShieldCheck, Trash2
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { deliveryAPI, type ActorRole, type JSONObject } from "./delivery-api";
import { AlertLine, EmptyState, FenInput, JsonOutput, LoadingState, Notice, SectionTitle, StatusPill, errorMessage, formatFen, type NoticeState } from "./delivery-ui";

type Props = { role: ActorRole };
type Lab = "bayesian" | "optimizer" | "bandit";
type BudgetArm = { key: string; min_fen: number; max_fen: number; increment_fen: number; expected_value: number; uncertainty: number; risk_penalty: number; minimum_sample_size: number; observed_samples: number };
type BanditArm = { key: string; pulls: number; reward_sum: number; context_score: number };

const EXECUTE_ROLES = new Set<ActorRole>(["analyst", "operator", "admin"]);

const BOUNDARY_LABELS: Record<string, { label: string; icon: typeof Bot }> = {
  llm: { label: "LLM", icon: Bot },
  lightgbm_lambdamart: { label: "LightGBM / LambdaMART", icon: BarChart3 },
  bayesian: { label: "贝叶斯", icon: Activity },
  constraint_optimizer: { label: "约束优化", icon: Scale },
  bandit: { label: "Bandit", icon: FlaskConical },
  rules: { label: "确定性规则", icon: ShieldCheck },
  human: { label: "人工决策", icon: CheckCircle2 }
};

function boundaryText(value: unknown): string {
  const translations: Record<string, string> = {
    "semantic extraction, candidate keywords, and evidence summaries only": "只做语义提取、候选关键词和证据摘要",
    "ranking over approved numeric features only": "只对已批准的数值特征排序",
    "uncertainty intervals and shrinkage for sparse segments": "估计不确定区间并收缩稀疏分群",
    "allocation suggestions inside operator-approved caps": "仅在人工批准的上限内给出分配建议",
    "shadow suggestions only; never activates or changes media state": "只输出影子建议，不启用或修改媒体状态",
    "permissions, platform enums, budget caps, approvals, and safety checks": "负责权限、平台枚举、预算上限、审批和安全检查",
    "final targeting, budget, publish, activation, and stop-loss decisions": "决定最终定向、预算、发布、启用和止损"
  };
  return translations[String(value)] || String(value || "-");
}

function BayesianLab({ enabled }: { enabled: boolean }) {
  const [alpha, setAlpha] = useState(1);
  const [beta, setBeta] = useState(1);
  const [successes, setSuccesses] = useState(8);
  const [trials, setTrials] = useState(100);
  const [result, setResult] = useState<JSONObject>();
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<NoticeState>(null);
  const run = async () => {
    setLoading(true); setNotice(null);
    try { setResult(await deliveryAPI.bayesian({ prior_alpha: alpha, prior_beta: beta, successes, trials })); }
    catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setLoading(false); }
  };
  const mean = typeof result?.mean === "number" ? result.mean : undefined;
  return <section className="dc-lab-panel">
    <SectionTitle icon={Activity} title="Beta-Binomial 后验" meta="稀疏转化率的均值与 95% 区间" actions={<button className="dc-action-button primary" disabled={!enabled || loading} onClick={() => void run()}>{loading ? <LoaderCircle className="spin" size={15} /> : <Activity size={15} />}计算后验</button>} />
    <Notice state={notice} onDismiss={() => setNotice(null)} />
    <div className="dc-form-grid four">
      <label className="dc-field"><span>先验 α</span><input type="number" min="0.01" step="0.1" value={alpha} onChange={(event) => setAlpha(Number(event.target.value))} /></label>
      <label className="dc-field"><span>先验 β</span><input type="number" min="0.01" step="0.1" value={beta} onChange={(event) => setBeta(Number(event.target.value))} /></label>
      <label className="dc-field"><span>成功数</span><input type="number" min="0" value={successes} onChange={(event) => setSuccesses(Number(event.target.value))} /></label>
      <label className="dc-field"><span>试验数</span><input type="number" min="0" value={trials} onChange={(event) => setTrials(Number(event.target.value))} /></label>
    </div>
    {result && mean !== undefined ? <div className="dc-bayesian-result"><div><span>后验均值</span><strong>{(mean * 100).toFixed(2)}%</strong></div><div><span>95% 下界</span><strong>{((Number(result.credible_low_95) || 0) * 100).toFixed(2)}%</strong></div><div><span>95% 上界</span><strong>{((Number(result.credible_high_95) || 0) * 100).toFixed(2)}%</strong></div><div><span>后验参数</span><strong>α {String(result.posterior_alpha)} / β {String(result.posterior_beta)}</strong></div></div> : <EmptyState icon={Activity} title="输入样本后计算" />}
  </section>;
}

function OptimizerLab({ enabled }: { enabled: boolean }) {
  const [totalFen, setTotalFen] = useState(30000);
  const [exploration, setExploration] = useState(0.1);
  const [arms, setArms] = useState<BudgetArm[]>([
    { key: "search-a", min_fen: 5000, max_fen: 20000, increment_fen: 100, expected_value: 0.8, uncertainty: 0.2, risk_penalty: 0.05, minimum_sample_size: 20, observed_samples: 35 },
    { key: "search-b", min_fen: 5000, max_fen: 20000, increment_fen: 100, expected_value: 0.65, uncertainty: 0.35, risk_penalty: 0.08, minimum_sample_size: 20, observed_samples: 12 }
  ]);
  const [result, setResult] = useState<JSONObject>();
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<NoticeState>(null);
  const mutate = (index: number, field: keyof BudgetArm, value: string | number) => setArms((current) => current.map((arm, armIndex) => armIndex === index ? { ...arm, [field]: value } : arm));
  const run = async () => {
    setLoading(true); setNotice(null);
    try { setResult(await deliveryAPI.optimizeBudget({ total_fen: totalFen, exploration_rate: exploration, arms })); }
    catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setLoading(false); }
  };
  const allocations = Array.isArray(result?.allocations) ? result.allocations as Array<Record<string, unknown>> : [];
  return <section className="dc-lab-panel">
    <SectionTitle icon={Scale} title="约束预算优化" meta="硬上限、最小预算、样本惩罚与风险惩罚" actions={<button className="dc-action-button primary" disabled={!enabled || loading} onClick={() => void run()}>{loading ? <LoaderCircle className="spin" size={15} /> : <Scale size={15} />}生成分配建议</button>} />
    <Notice state={notice} onDismiss={() => setNotice(null)} />
    <div className="dc-lab-controls"><label className="dc-field"><span>总预算</span><FenInput value={totalFen} onChange={setTotalFen} /></label><label className="dc-field"><span>探索率</span><input type="number" min="0" max="1" step="0.05" value={exploration} onChange={(event) => setExploration(Number(event.target.value))} /></label><button className="dc-action-button secondary align-end" onClick={() => setArms((current) => [...current, { key: `arm-${current.length + 1}`, min_fen: 0, max_fen: 10000, increment_fen: 100, expected_value: 0.5, uncertainty: 0.2, risk_penalty: 0, minimum_sample_size: 20, observed_samples: 0 }])}><Plus size={15} />添加方案</button></div>
    <div className="dc-table-wrap"><table className="dc-table dc-input-table"><thead><tr><th>方案</th><th>最小预算</th><th>最大预算</th><th>预期价值</th><th>不确定性</th><th>风险惩罚</th><th>样本 / 门槛</th><th /></tr></thead><tbody>{arms.map((arm, index) => <tr key={index}><td><input value={arm.key} onChange={(event) => mutate(index, "key", event.target.value)} /></td><td><FenInput value={arm.min_fen} onChange={(value) => mutate(index, "min_fen", value)} /></td><td><FenInput value={arm.max_fen} onChange={(value) => mutate(index, "max_fen", value)} /></td><td><input type="number" step="0.01" value={arm.expected_value} onChange={(event) => mutate(index, "expected_value", Number(event.target.value))} /></td><td><input type="number" step="0.01" value={arm.uncertainty} onChange={(event) => mutate(index, "uncertainty", Number(event.target.value))} /></td><td><input type="number" step="0.01" value={arm.risk_penalty} onChange={(event) => mutate(index, "risk_penalty", Number(event.target.value))} /></td><td><div className="dc-pair-input"><input type="number" value={arm.observed_samples} onChange={(event) => mutate(index, "observed_samples", Number(event.target.value))} /><span>/</span><input type="number" value={arm.minimum_sample_size} onChange={(event) => mutate(index, "minimum_sample_size", Number(event.target.value))} /></div></td><td><button className="dc-icon-button danger" aria-label={`删除方案 ${index + 1}`} disabled={arms.length === 1} onClick={() => setArms((current) => current.filter((_, armIndex) => armIndex !== index))}><Trash2 size={15} /></button></td></tr>)}</tbody></table></div>
    {allocations.length ? <div className="dc-allocation-result"><header><strong>建议分配</strong><StatusPill value={result?.executable ? "active" : "shadow"} /></header>{allocations.map((item) => <div key={String(item.key)}><span>{String(item.key)}</span><div><i style={{ width: `${Math.min(100, (Number(item.amount_fen) / totalFen) * 100)}%` }} /></div><strong>{formatFen(Number(item.amount_fen))}</strong><small>score {Number(item.score).toFixed(3)}</small></div>)}</div> : <EmptyState icon={Scale} title="暂无预算建议" />}
    <AlertLine>该接口的 executable 固定为 false，建议不会修改草稿或媒体预算</AlertLine>
  </section>;
}

function BanditLab({ enabled }: { enabled: boolean }) {
  const [exploration, setExploration] = useState(1);
  const [minimumPulls, setMinimumPulls] = useState(20);
  const [arms, setArms] = useState<BanditArm[]>([
    { key: "creative-a", pulls: 35, reward_sum: 6, context_score: 0.03 },
    { key: "creative-b", pulls: 12, reward_sum: 3, context_score: 0.01 }
  ]);
  const [result, setResult] = useState<JSONObject>();
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<NoticeState>(null);
  const mutate = (index: number, field: keyof BanditArm, value: string | number) => setArms((current) => current.map((arm, armIndex) => armIndex === index ? { ...arm, [field]: value } : arm));
  const run = async () => {
    setLoading(true); setNotice(null);
    try { setResult(await deliveryAPI.banditShadow({ arms, exploration_weight: exploration, minimum_pulls_per_arm: minimumPulls })); }
    catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setLoading(false); }
  };
  const scores = result?.scores && typeof result.scores === "object" ? result.scores as Record<string, number> : {};
  return <section className="dc-lab-panel">
    <SectionTitle icon={FlaskConical} title="Contextual UCB 影子评估" meta="观察候选选择，不影响投放状态" actions={<button className="dc-action-button primary" disabled={!enabled || loading} onClick={() => void run()}>{loading ? <LoaderCircle className="spin" size={15} /> : <FlaskConical size={15} />}计算影子建议</button>} />
    <Notice state={notice} onDismiss={() => setNotice(null)} />
    <div className="dc-lab-controls"><label className="dc-field"><span>探索权重</span><input type="number" min="0" step="0.1" value={exploration} onChange={(event) => setExploration(Number(event.target.value))} /></label><label className="dc-field"><span>每臂最小样本</span><input type="number" min="0" value={minimumPulls} onChange={(event) => setMinimumPulls(Number(event.target.value))} /></label><button className="dc-action-button secondary align-end" onClick={() => setArms((current) => [...current, { key: `creative-${current.length + 1}`, pulls: 0, reward_sum: 0, context_score: 0 }])}><Plus size={15} />添加候选</button></div>
    <div className="dc-table-wrap"><table className="dc-table dc-input-table"><thead><tr><th>候选</th><th>曝光次数</th><th>奖励累计</th><th>上下文得分</th><th>影子得分</th><th /></tr></thead><tbody>{arms.map((arm, index) => <tr key={index} className={result?.selected_key === arm.key ? "selected" : ""}><td><input value={arm.key} onChange={(event) => mutate(index, "key", event.target.value)} /></td><td><input type="number" min="0" value={arm.pulls} onChange={(event) => mutate(index, "pulls", Number(event.target.value))} /></td><td><input type="number" step="0.01" value={arm.reward_sum} onChange={(event) => mutate(index, "reward_sum", Number(event.target.value))} /></td><td><input type="number" step="0.01" value={arm.context_score} onChange={(event) => mutate(index, "context_score", Number(event.target.value))} /></td><td>{scores[arm.key] === undefined ? "-" : scores[arm.key] > 1e9 ? "待探索" : scores[arm.key].toFixed(4)}</td><td><button className="dc-icon-button danger" aria-label={`删除候选 ${index + 1}`} disabled={arms.length <= 2} onClick={() => setArms((current) => current.filter((_, armIndex) => armIndex !== index))}><Trash2 size={15} /></button></td></tr>)}</tbody></table></div>
    {result ? <div className="dc-bandit-result"><span>影子选择</span><strong>{String(result.selected_key)}</strong><small>{String(result.method)}</small></div> : <EmptyState icon={FlaskConical} title="暂无影子建议" />}
    <AlertLine>Bandit 固定 shadow_only，不启用创意、不改出价、不改预算</AlertLine>
  </section>;
}

export default function DeliveryIntelligenceLab({ role }: Props) {
  const [capabilities, setCapabilities] = useState<JSONObject>();
  const [loadingCapabilities, setLoadingCapabilities] = useState(true);
  const [notice, setNotice] = useState<NoticeState>(null);
  const [lab, setLab] = useState<Lab>("bayesian");
  const enabled = EXECUTE_ROLES.has(role);
  const load = useCallback(async () => {
    setLoadingCapabilities(true); setNotice(null);
    try { setCapabilities(await deliveryAPI.intelligenceCapabilities()); }
    catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setLoadingCapabilities(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  const boundaries = useMemo(() => capabilities?.responsibility_boundary && typeof capabilities.responsibility_boundary === "object" ? capabilities.responsibility_boundary as Record<string, unknown> : {}, [capabilities]);
  return <div className="dc-intelligence">
    <section className="dc-boundaries">
      <SectionTitle icon={BrainCircuit} title="算法职责边界" meta="模型只生成建议，确定性规则与人工审批控制执行" actions={<button className="dc-icon-button" onClick={() => void load()} title="刷新算法能力" aria-label="刷新算法能力"><RefreshCw size={16} className={loadingCapabilities ? "spin" : ""} /></button>} />
      <Notice state={notice} onDismiss={() => setNotice(null)} />
      {loadingCapabilities && !capabilities ? <LoadingState label="加载算法能力" /> : <div className="dc-boundary-grid">{Object.entries(BOUNDARY_LABELS).map(([key, definition]) => {
        const Icon = definition.icon;
        const metadata = key === "lightgbm_lambdamart" ? capabilities?.ranker : capabilities?.[key === "constraint_optimizer" ? "optimizer" : key];
        return <article key={key}><Icon size={18} /><div><strong>{definition.label}</strong><p>{boundaryText(boundaries[key])}</p></div>{metadata && typeof metadata === "object" ? <code>{String((metadata as JSONObject).method || (metadata as JSONObject).model || (metadata as JSONObject).version || "已配置")}</code> : null}</article>;
      })}</div>}
    </section>
    {!enabled ? <AlertLine>当前角色为 {role}，算法计算接口要求 analyst、operator 或 admin</AlertLine> : null}
    <nav className="dc-subnav" aria-label="算法实验室">
      <button className={lab === "bayesian" ? "active" : ""} onClick={() => setLab("bayesian")}><Activity size={16} />贝叶斯</button>
      <button className={lab === "optimizer" ? "active" : ""} onClick={() => setLab("optimizer")}><Scale size={16} />约束优化</button>
      <button className={lab === "bandit" ? "active" : ""} onClick={() => setLab("bandit")}><FlaskConical size={16} />Bandit 影子</button>
    </nav>
    {lab === "bayesian" ? <BayesianLab enabled={enabled} /> : null}
    {lab === "optimizer" ? <OptimizerLab enabled={enabled} /> : null}
    {lab === "bandit" ? <BanditLab enabled={enabled} /> : null}
  </div>;
}
