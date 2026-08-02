import { useEffect, useMemo, useState } from "react";
import { AlertCircle, ExternalLink, LoaderCircle, Stethoscope, X } from "lucide-react";
import "./account-plan-diagnosis.css";

type ServiceState = "checking" | "online" | "offline";
type PlanTab = "over" | "enlarge" | "stop";

type DiagnosisPoint = {
  report_date: string;
  cost: number | null;
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
  cost: number | null;
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

type DiagnosisResult = {
  report_date: string;
  spu: string;
  account_kpi: number;
  plan_kpis: Record<string, number>;
  dandelion_synced_at: string;
  dandelion_matched: number;
  dandelion_missing: number;
  accounts: AccountDiagnosis[];
};

const EMPTY_RESULT: DiagnosisResult = {
  report_date: "", spu: "辅酶", account_kpi: 70, plan_kpis: { 搜索: 30, 信息流: 70 },
  dandelion_synced_at: "", dandelion_matched: 0, dandelion_missing: 0, accounts: []
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
  const values = points.map((point) => point.cost).filter((value): value is number => value !== null);
  if (values.length === 0) return <span className="diagnosis-no-trend">无数据</span>;
  const minimum = Math.min(...values);
  const maximum = Math.max(...values);
  const coordinates = points.map((point, index) => point.cost === null ? null : {
    x: 3 + index * (86 / Math.max(points.length - 1, 1)),
    y: maximum === minimum ? 14 : 3 + ((maximum - point.cost) / (maximum - minimum)) * 22
  });
  let path = "";
  let open = false;
  coordinates.forEach((coordinate) => {
    if (!coordinate) {
      open = false;
      return;
    }
    path += `${open ? "L" : "M"}${coordinate.x},${coordinate.y} `;
    open = true;
  });
  return <svg className="diagnosis-sparkline" viewBox="0 0 92 28" role="img" aria-label="7日成本趋势">
    <path d={path} />
    {coordinates.map((coordinate, index) => coordinate
      ? <circle key={points[index].report_date} cx={coordinate.x} cy={coordinate.y} r="2"><title>{points[index].report_date}：¥{money.format(points[index].cost ?? 0)}</title></circle>
      : null)}
  </svg>;
}

function planFilter(plans: PlanDiagnosis[], tab: PlanTab): PlanDiagnosis[] {
  if (tab === "over") return plans.filter((plan) => plan.over_kpi);
  return plans.filter((plan) => plan.action === tab);
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
        <div><h2>{account.account}</h2><p>{account.placement} · {account.cost_metric} · 计划 KPI {money.format(account.placement === "信息流" ? 70 : 30)}</p></div>
        <button className="icon-button" type="button" title="关闭" aria-label="关闭" onClick={onClose}><X size={19} /></button>
      </header>
      <div className="diagnosis-drawer-tabs" aria-label="计划诊断分类">
        {(Object.keys(labels) as PlanTab[]).map((value) => <button type="button" className={tab === value ? "active" : ""} key={value} onClick={() => setTab(value)}>{labels[value]} <span>{counts[value]}</span></button>)}
      </div>
      <div className="diagnosis-drawer-body">
        <div className="diagnosis-drawer-summary">本页展示全部{labels[tab]}计划，不设置消耗门槛；连续 3 个有效报表日成本超标时建议停止。</div>
        {plans.length === 0 ? <div className="diagnosis-drawer-empty">该分类暂无计划</div> : <div className="diagnosis-plan-table-wrap"><table className="diagnosis-plan-table">
          <thead><tr><th>计划名</th><th>蒲公英数据</th><th>消耗</th><th>诊断成本</th><th>KPI</th><th>超标</th><th>动作</th><th>连续天数</th></tr></thead>
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
              <td className="num" title={plan.cost_metric}>{plan.cost === null ? "-" : `¥${money.format(plan.cost)}`}</td>
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

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/account-plan-diagnosis?spu=${encodeURIComponent("辅酶")}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: DiagnosisResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "子账户诊断读取失败");
        setResult(payload.data);
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
  const totalPlans = result.accounts.reduce((total, account) => total + account.plans.length, 0);
  const dandelionTotal = result.dandelion_matched + result.dandelion_missing;
  const dandelionDate = result.dandelion_synced_at ? shortDate(result.dandelion_synced_at.slice(0, 10)) : "-";

  return <>
    <section className="page-heading diagnosis-page-heading">
      <div><h1>子账户与计划诊断</h1><p>辅酶Q10 · 子账户 KPI 70 · 计划 KPI：搜索 30、信息流 70</p></div>
      <div className="heading-status"><span className={`status-dot ${serviceState}`} />{result.report_date ? `数据截至 ${shortDate(result.report_date)}` : "等待日报数据"}</div>
    </section>
    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}
    <section className="diagnosis-table-section">
      <header><div><Stethoscope size={18} /><span><strong>子账户诊断</strong><small>{result.accounts.length} 个子账户场域 · {totalPlans} 条计划明细</small></span></div><p>蒲公英 {result.dandelion_matched}/{dandelionTotal} · 更新 {dandelionDate}</p></header>
      {loading ? <div className="diagnosis-loading"><LoaderCircle size={19} className="spin" />正在生成诊断</div>
        : result.accounts.length === 0 ? <div className="diagnosis-loading">当前 SPU 暂无可诊断数据</div>
          : <div className="diagnosis-table-wrap"><table className="diagnosis-account-table">
            <thead><tr><th>子账户</th><th>场域</th><th>消耗</th><th>诊断成本</th><th>较昨日</th><th>KPI</th><th>状态</th><th>超标计划</th><th>放大</th><th>停止</th><th>7日成本</th></tr></thead>
            <tbody>{result.accounts.map((account) => <tr key={accountKey(account)}>
              <td><button type="button" className="diagnosis-account-button" onClick={() => setSelectedKey(accountKey(account))}>{account.account}</button></td>
              <td><span className={`placement-swatch placement-${account.placement}`}>{account.placement}</span></td>
              <td className="num">¥{money.format(account.spend)}</td>
              <td className="num" title={account.cost_metric}>{account.cost === null ? "-" : `¥${money.format(account.cost)}`}</td>
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
