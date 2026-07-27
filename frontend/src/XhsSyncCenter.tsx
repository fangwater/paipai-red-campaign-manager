import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertCircle, CheckCircle2, Clock3, Lightbulb, LoaderCircle, Megaphone,
  RefreshCw, Rows3, XCircle
} from "lucide-react";

export type SyncTarget = "campaigns" | "units" | "creativities";
type SyncMode = "incremental" | "full";
type RunStatus = "running" | "succeeded" | "failed";

type SyncRun = {
  run_id: number;
  mode: SyncMode;
  target: SyncTarget | "all";
  trigger_type: string;
  requested_advertiser_id?: number;
  status: RunStatus;
  advertisers_count: number;
  campaigns_count: number;
  units_count: number;
  creativities_count: number;
  deactivated_count: number;
  error_message?: string;
  started_at: string;
  finished_at?: string;
};

type SyncStatus = {
  running: boolean;
  current?: SyncRun;
  recent: SyncRun[];
};

const TARGETS: Array<{
  key: SyncTarget;
  label: string;
  singular: string;
  icon: typeof Megaphone;
  modes: SyncMode[];
}> = [
  { key: "campaigns", label: "推广计划", singular: "计划", icon: Megaphone, modes: ["incremental", "full"] },
  { key: "units", label: "广告单元", singular: "单元", icon: Rows3, modes: ["incremental", "full"] },
  { key: "creativities", label: "创意", singular: "创意", icon: Lightbulb, modes: ["full"] }
];

const EMPTY_STATUS: SyncStatus = { running: false, recent: [] };
const dateTimeFormatter = new Intl.DateTimeFormat("zh-CN", {
  month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false
});

function targetLabel(target: SyncRun["target"]): string {
  return TARGETS.find((item) => item.key === target)?.label ?? "全部对象";
}

function modeLabel(mode: SyncMode): string {
  return mode === "full" ? "全量" : "增量";
}

function runCount(run: SyncRun): number {
  if (run.target === "campaigns") return run.campaigns_count;
  if (run.target === "units") return run.units_count;
  if (run.target === "creativities") return run.creativities_count;
  return run.campaigns_count + run.units_count + run.creativities_count;
}

function runDuration(run: SyncRun): string {
  if (!run.finished_at) return "进行中";
  const seconds = Math.max(0, Math.round((new Date(run.finished_at).getTime() - new Date(run.started_at).getTime()) / 1000));
  if (seconds < 60) return `${seconds} 秒`;
  return `${Math.floor(seconds / 60)} 分 ${seconds % 60} 秒`;
}

function statusText(status: RunStatus): string {
  if (status === "succeeded") return "已完成";
  if (status === "failed") return "失败";
  return "进行中";
}

function StatusIcon({ status }: { status: RunStatus }) {
  if (status === "succeeded") return <CheckCircle2 size={15} />;
  if (status === "failed") return <XCircle size={15} />;
  return <LoaderCircle size={15} className="spin" />;
}

function XhsSyncCenter({ activeTarget }: { activeTarget: SyncTarget }) {
  const [modes, setModes] = useState<Record<SyncTarget, SyncMode>>({
    campaigns: "incremental", units: "incremental", creativities: "full"
  });
  const [status, setStatus] = useState<SyncStatus>(EMPTY_STATUS);
  const [loading, setLoading] = useState(true);
  const [triggering, setTriggering] = useState<SyncTarget | "">("");
  const [error, setError] = useState("");

  const loadStatus = useCallback(async (signal?: AbortSignal) => {
    const response = await fetch(`${import.meta.env.BASE_URL}api/xhs-jg/sync/status`, { signal });
    const payload = await response.json() as { success: boolean; data?: SyncStatus; error?: string };
    if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "无法读取聚光同步状态");
    setStatus(payload.data);
    setError("");
    return payload.data;
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    loadStatus(controller.signal)
      .catch((loadError) => {
        if (!(loadError instanceof DOMException && loadError.name === "AbortError")) {
          setError(loadError instanceof Error ? loadError.message : "无法读取聚光同步状态");
        }
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [loadStatus]);

  useEffect(() => {
    if (!status.running) return;
    const controller = new AbortController();
    const timer = window.setInterval(() => {
      void loadStatus(controller.signal).catch((loadError) => {
        if (!(loadError instanceof DOMException && loadError.name === "AbortError")) {
          setError(loadError instanceof Error ? loadError.message : "无法读取聚光同步状态");
        }
      });
    }, 1500);
    return () => { controller.abort(); window.clearInterval(timer); };
  }, [loadStatus, status.running]);

  const latestByTarget = useMemo(() => {
    const result = new Map<SyncTarget, SyncRun>();
    for (const run of status.recent) {
      if (run.target !== "all" && !result.has(run.target)) result.set(run.target, run);
    }
    return result;
  }, [status.recent]);

  const trigger = async (target: SyncTarget) => {
    if (status.running || triggering) return;
    setTriggering(target);
    setError("");
    try {
      const response = await fetch(`${import.meta.env.BASE_URL}api/xhs-jg/sync/${target}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ mode: modes[target] })
      });
      const payload = await response.json() as { success: boolean; data?: SyncRun; error?: string };
      if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "同步任务启动失败");
      setStatus((current) => ({ running: true, current: payload.data, recent: [payload.data!, ...current.recent] }));
    } catch (triggerError) {
      setError(triggerError instanceof Error ? triggerError.message : "同步任务启动失败");
      void loadStatus().catch(() => undefined);
    } finally {
      setTriggering("");
    }
  };

  return <>
    <section className="page-heading sync-page-heading">
      <div><h1>聚光数据同步</h1><p>投放管理 · 手动同步</p></div>
      <div className="heading-status"><span className={`status-dot ${error ? "offline" : loading ? "checking" : ""}`} />
        {error ? "同步服务异常" : loading ? "正在读取状态" : status.running ? "同步任务进行中" : "同步服务已连接"}
      </div>
    </section>

    {status.running && status.current ? <section className="sync-running-banner">
      <LoaderCircle size={18} className="spin" />
      <div><strong>{targetLabel(status.current.target)}正在同步</strong><span>{modeLabel(status.current.mode)} · 开始于 {dateTimeFormatter.format(new Date(status.current.started_at))}</span></div>
      <span className="run-id">#{status.current.run_id}</span>
    </section> : null}
    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <section className="sync-target-grid">
      {TARGETS.map((target) => {
        const Icon = target.icon;
        const latest = latestByTarget.get(target.key);
        const active = activeTarget === target.key;
        return <article className={`sync-target ${active ? "active" : ""}`} key={target.key}>
          <header><span className={`sync-target-icon target-${target.key}`}><Icon size={20} /></span><div><h2>{target.label}</h2><p>{latest ? `最近同步 ${dateTimeFormatter.format(new Date(latest.started_at))}` : "暂无同步记录"}</p></div></header>
          <div className="sync-target-state">
            <div><span>最近状态</span><strong className={`run-status ${latest?.status ?? "idle"}`}>{latest ? statusText(latest.status) : "未同步"}</strong></div>
            <div><span>写入数量</span><strong>{latest ? runCount(latest).toLocaleString() : "-"}</strong></div>
            <div><span>广告主</span><strong>{latest ? latest.advertisers_count.toLocaleString() : "-"}</strong></div>
          </div>
          <footer>
            {target.modes.length > 1 ? <div className="sync-mode" aria-label={`${target.label}同步模式`}>
              {target.modes.map((mode) => <button key={mode} className={modes[target.key] === mode ? "active" : ""} disabled={status.running || triggering !== ""} onClick={() => setModes((current) => ({ ...current, [target.key]: mode }))}>{modeLabel(mode)}</button>)}
            </div> : <span className="full-mode-label">全量模式</span>}
            <button className="sync-trigger" disabled={status.running || triggering !== ""} onClick={() => void trigger(target.key)}>
              {triggering === target.key ? <LoaderCircle size={16} className="spin" /> : <RefreshCw size={16} />}
              {triggering === target.key ? "正在启动" : `同步${target.singular}`}
            </button>
          </footer>
        </article>;
      })}
    </section>

    <section className="sync-history">
      <header><div><h2>最近同步记录</h2><p>{status.recent.length} 次任务</p></div>{loading ? <LoaderCircle size={17} className="spin" /> : <Clock3 size={17} />}</header>
      <div className="sync-history-wrap"><table><thead><tr><th>开始时间</th><th>目标</th><th>模式</th><th>状态</th><th>广告主</th><th>写入数量</th><th>失效数量</th><th>耗时</th></tr></thead><tbody>
        {status.recent.map((run) => <tr key={run.run_id} title={run.error_message || undefined}>
          <td>{dateTimeFormatter.format(new Date(run.started_at))}</td><td>{targetLabel(run.target)}</td><td>{modeLabel(run.mode)}</td>
          <td><span className={`history-status ${run.status}`}><StatusIcon status={run.status} />{statusText(run.status)}</span></td>
          <td>{run.advertisers_count.toLocaleString()}</td><td>{runCount(run).toLocaleString()}</td><td>{run.deactivated_count.toLocaleString()}</td><td>{runDuration(run)}</td>
        </tr>)}
      </tbody></table>{!loading && status.recent.length === 0 ? <div className="sync-empty">暂无同步记录</div> : null}</div>
    </section>
  </>;
}

export default XhsSyncCenter;
