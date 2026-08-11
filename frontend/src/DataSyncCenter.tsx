import { useCallback, useEffect, useState } from "react";
import { AlertCircle, CalendarDays, CheckCircle2, Database, FileText, LoaderCircle, RefreshCw, XCircle } from "lucide-react";

export type DataSyncTarget = "dandelion" | "manuscripts" | "coenzyme-q10";

type DandelionRun = {
  run_id: number;
  status: "running" | "succeeded" | "failed";
  fetched_count: number;
  upserted_count: number;
  deleted_count: number;
  error_message?: string;
  started_at: string;
  completed_at?: string;
};

type ProviderStatus = {
  provider_code: string;
  provider_name: string;
  sheet_name: string;
  status: "pending" | "running" | "succeeded" | "failed";
  error?: string;
  last_synced_at?: string;
};

type CoenzymeRun = {
  run_id: number;
  status: "running" | "succeeded" | "failed";
  fetched: number;
  inserted: number;
  updated: number;
  unchanged: number;
  earliest_date?: string;
  latest_date?: string;
  error_message?: string;
  started_at: string;
  completed_at?: string;
};

type CoenzymeStatus = {
  record_count: number;
  earliest_date?: string;
  latest_date?: string;
  last_synced_at?: string;
  recent: CoenzymeRun[];
};

const EMPTY_COENZYME_STATUS: CoenzymeStatus = { record_count: 0, recent: [] };

type SyncResult = {
  fetched: number;
  upserted?: number;
  deleted?: number;
  inserted?: number;
  updated?: number;
  unchanged?: number;
  providers?: number;
  notes?: number;
  note_errors?: number;
  tables?: number;
};

const dateTimeFormatter = new Intl.DateTimeFormat("zh-CN", {
  month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false
});

function stateLabel(status?: string): string {
  if (status === "succeeded") return "已完成";
  if (status === "failed") return "失败";
  if (status === "running") return "进行中";
  return "未同步";
}


function targetLabel(target: DataSyncTarget): string {
  if (target === "dandelion") return "蒲公英数据";
  if (target === "manuscripts") return "稿件数据";
  return "辅酶Q10日数据";
}

function resultSummary(target: DataSyncTarget, result: SyncResult): string {
  if (target === "coenzyme-q10") {
    return `读取 ${result.fetched.toLocaleString()} · 新增 ${(result.inserted ?? 0).toLocaleString()} · 更新 ${(result.updated ?? 0).toLocaleString()} · 未变化 ${(result.unchanged ?? 0).toLocaleString()}`;
  }
  return `读取 ${result.fetched.toLocaleString()} · 写入 ${(result.upserted ?? 0).toLocaleString()} · 失效 ${(result.deleted ?? 0).toLocaleString()}`;
}
function DataSyncCenter({ activeTarget }: { activeTarget: DataSyncTarget }) {
  const [dandelionRuns, setDandelionRuns] = useState<DandelionRun[]>([]);
  const [providers, setProviders] = useState<ProviderStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState<DataSyncTarget | "">("");
  const [coenzymeStatus, setCoenzymeStatus] = useState<CoenzymeStatus>(EMPTY_COENZYME_STATUS);
  const [result, setResult] = useState<{ target: DataSyncTarget; data: SyncResult }>();
  const [error, setError] = useState("");

  const loadStatus = useCallback(async (signal?: AbortSignal) => {
    const [dandelionResponse, manuscriptResponse, coenzymeResponse] = await Promise.all([
      fetch(`${import.meta.env.BASE_URL}api/lark/sync/dandelion/status`, { signal }),
      fetch(`${import.meta.env.BASE_URL}api/lark/sync/manuscripts/status`, { signal }),
      fetch(`${import.meta.env.BASE_URL}api/lark/sync/coenzyme-q10/status`, { signal })
    ]);
    const dandelionPayload = await dandelionResponse.json() as { success: boolean; data?: { recent: DandelionRun[] }; error?: string };
    const manuscriptPayload = await manuscriptResponse.json() as { success: boolean; data?: { providers: ProviderStatus[] }; error?: string };
    const coenzymePayload = await coenzymeResponse.json() as { success: boolean; data?: CoenzymeStatus; error?: string };
    if (!dandelionResponse.ok || !dandelionPayload.success) throw new Error(dandelionPayload.error || "无法读取蒲公英同步状态");
    if (!manuscriptResponse.ok || !manuscriptPayload.success) throw new Error(manuscriptPayload.error || "无法读取稿件同步状态");
    if (!coenzymeResponse.ok || !coenzymePayload.success) throw new Error(coenzymePayload.error || "无法读取辅酶Q10同步状态");
    setDandelionRuns(dandelionPayload.data?.recent ?? []);
    setProviders(manuscriptPayload.data?.providers ?? []);
    setCoenzymeStatus(coenzymePayload.data ?? EMPTY_COENZYME_STATUS);
    setError("");
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    loadStatus(controller.signal)
      .catch((loadError) => {
        if (!(loadError instanceof DOMException && loadError.name === "AbortError")) {
          setError(loadError instanceof Error ? loadError.message : "无法读取数据同步状态");
        }
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [loadStatus]);

  const trigger = async (target: DataSyncTarget) => {
    if (syncing) return;
    setSyncing(target);
    setError("");
    setResult(undefined);
    try {
      const response = await fetch(`${import.meta.env.BASE_URL}api/lark/sync/${target}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: "{}"
      });
      const payload = await response.json() as { success: boolean; data?: SyncResult; error?: string };
      if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "同步失败");
      setResult({ target, data: payload.data });
      await loadStatus();
    } catch (syncError) {
      setError(syncError instanceof Error ? syncError.message : "同步失败");
      void loadStatus().catch(() => undefined);
    } finally {
      setSyncing("");
    }
  };

  const latestDandelion = dandelionRuns[0];
  const manuscriptSucceeded = providers.filter((provider) => provider.status === "succeeded").length;
  const manuscriptLatest = providers.reduce<string | undefined>((latest, provider) =>
    provider.last_synced_at && (!latest || provider.last_synced_at > latest) ? provider.last_synced_at : latest, undefined);
  const latestCoenzyme = coenzymeStatus.recent[0];

  return <>
    <section className="page-heading data-sync-page-heading">
      <div><h1>飞书数据同步</h1><p>数据中心 · 手动同步</p></div>
      <div className="heading-status"><span className={`status-dot ${error ? "offline" : loading ? "checking" : ""}`} />
        {error ? "同步服务异常" : loading ? "正在读取状态" : "同步服务已连接"}
      </div>
    </section>

    {result ? <section className="data-sync-result"><CheckCircle2 size={18} /><div><strong>{targetLabel(result.target)}同步完成</strong><span>{resultSummary(result.target, result.data)}</span></div></section> : null}
    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <section className="data-sync-grid">
      <article className={`data-sync-target ${activeTarget === "dandelion" ? "active" : ""}`}>
        <header><span className="data-target-icon dandelion"><Database size={21} /></span><div><h2>蒲公英数据</h2><p>{latestDandelion ? `最近同步 ${dateTimeFormatter.format(new Date(latestDandelion.started_at))}` : "暂无同步记录"}</p></div></header>
        <div className="data-sync-metrics"><div><span>最近状态</span><strong className={`data-state ${latestDandelion?.status ?? "pending"}`}>{stateLabel(latestDandelion?.status)}</strong></div><div><span>读取记录</span><strong>{latestDandelion ? latestDandelion.fetched_count.toLocaleString() : "-"}</strong></div><div><span>写入记录</span><strong>{latestDandelion ? latestDandelion.upserted_count.toLocaleString() : "-"}</strong></div></div>
        <footer><span>单表同步</span><button className="sync-trigger" disabled={syncing !== ""} onClick={() => void trigger("dandelion")}>{syncing === "dandelion" ? <LoaderCircle size={16} className="spin" /> : <RefreshCw size={16} />}{syncing === "dandelion" ? "同步中" : "同步蒲公英"}</button></footer>
      </article>

      <article className={`data-sync-target ${activeTarget === "manuscripts" ? "active" : ""}`}>
        <header><span className="data-target-icon manuscripts"><FileText size={21} /></span><div><h2>稿件数据</h2><p>{manuscriptLatest ? `最近同步 ${dateTimeFormatter.format(new Date(manuscriptLatest))}` : "暂无同步记录"}</p></div></header>
        <div className="data-sync-metrics"><div><span>服务商</span><strong>{providers.length || "-"}</strong></div><div><span>已完成</span><strong>{providers.length ? manuscriptSucceeded : "-"}</strong></div><div><span>异常</span><strong>{providers.length ? providers.length - manuscriptSucceeded : "-"}</strong></div></div>
        <footer><span>全部服务商</span><button className="sync-trigger" disabled={syncing !== ""} onClick={() => void trigger("manuscripts")}>{syncing === "manuscripts" ? <LoaderCircle size={16} className="spin" /> : <RefreshCw size={16} />}{syncing === "manuscripts" ? "同步中" : "同步稿件"}</button></footer>
      </article>

      <article className={`data-sync-target ${activeTarget === "coenzyme-q10" ? "active" : ""}`}>
        <header><span className="data-target-icon coenzyme"><CalendarDays size={21} /></span><div><h2>辅酶Q10日数据</h2><p>{coenzymeStatus.last_synced_at ? `最近同步 ${dateTimeFormatter.format(new Date(coenzymeStatus.last_synced_at))}` : "暂无同步记录"}</p></div></header>
        <div className="data-sync-metrics"><div><span>最近状态</span><strong className={`data-state ${latestCoenzyme?.status ?? "pending"}`}>{stateLabel(latestCoenzyme?.status)}</strong></div><div><span>日记录</span><strong>{coenzymeStatus.record_count ? coenzymeStatus.record_count.toLocaleString() : "-"}</strong></div><div><span>最新日期</span><strong className="coenzyme-latest-date">{coenzymeStatus.latest_date ?? "-"}</strong></div></div>
        <footer><span>按日期增量更新</span><button className="sync-trigger" disabled={syncing !== ""} onClick={() => void trigger("coenzyme-q10")}>{syncing === "coenzyme-q10" ? <LoaderCircle size={16} className="spin" /> : <RefreshCw size={16} />}{syncing === "coenzyme-q10" ? "同步中" : "同步辅酶Q10"}</button></footer>
      </article>
    </section>

    <section className="provider-status-section">
      <header><div><h2>稿件服务商状态</h2><p>{providers.length} 个同步目标</p></div>{loading ? <LoaderCircle size={17} className="spin" /> : null}</header>
      <div className="provider-status-list">
        {providers.map((provider) => <div className="provider-status-row" key={provider.provider_code} title={provider.error || undefined}>
          <span className={`provider-state-icon ${provider.status}`}>{provider.status === "succeeded" ? <CheckCircle2 size={16} /> : provider.status === "failed" ? <XCircle size={16} /> : <LoaderCircle size={16} className={provider.status === "running" ? "spin" : ""} />}</span>
          <div><strong>{provider.provider_name}</strong><span>{provider.sheet_name}</span></div>
          <span>{provider.last_synced_at ? dateTimeFormatter.format(new Date(provider.last_synced_at)) : "尚未同步"}</span>
          <b className={`data-state ${provider.status}`}>{stateLabel(provider.status)}</b>
        </div>)}
        {!loading && providers.length === 0 ? <div className="sync-empty">暂无服务商状态</div> : null}
      </div>
    </section>
  </>;
}

export default DataSyncCenter;
