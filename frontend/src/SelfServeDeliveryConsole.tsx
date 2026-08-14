import {
	BarChart3, BookOpenCheck, BrainCircuit, ChevronDown, CircleAlert, Database,
	LockKeyhole, RefreshCw, Route, Workflow
} from "lucide-react";
import { lazy, Suspense, useCallback, useEffect, useState } from "react";
import DeliveryDataWorkspace from "./DeliveryDataWorkspace";
import DeliveryDraftWorkspace from "./DeliveryDraftWorkspace";
import DeliveryIntelligenceLab from "./DeliveryIntelligenceLab";
import { deliveryAPI, type Capability, type DeliverySession } from "./delivery-api";
import { LoadingState, Notice, StatusPill, errorMessage, type NoticeState } from "./delivery-ui";
import "./self-serve-delivery-console.css";

const SelfServeDeliveryPlan = lazy(() => import("./SelfServeDeliveryPlan"));

type ConsoleView = "workspace" | "data" | "intelligence" | "blueprint";

const VIEWS = [
  { key: "workspace" as const, label: "投放工作台", icon: Workflow },
  { key: "data" as const, label: "资产与报表", icon: BarChart3 },
  { key: "intelligence" as const, label: "算法实验室", icon: BrainCircuit },
  { key: "blueprint" as const, label: "设计与边界", icon: BookOpenCheck }
];

function SessionBar({ session, advertiserID, capability, refreshing, onAdvertiser, onRefresh }: {
  session: DeliverySession;
  advertiserID: number;
  capability?: Capability;
  refreshing: boolean;
  onAdvertiser: (advertiserID: number) => void;
  onRefresh: () => void;
}) {
  return <section className="dc-session-bar">
    <div className="dc-identity"><span className="dc-avatar">{session.actor.id.slice(0, 2).toUpperCase()}</span><div><strong>{session.actor.id}</strong><span><StatusPill value={session.actor.role} />控制台直通</span></div></div>
    <label className="dc-advertiser-select"><span>广告主</span><div><Database size={15} /><select value={advertiserID || ""} onChange={(event) => onAdvertiser(Number(event.target.value))} aria-label="选择广告主"><option value="" disabled>请选择</option>{session.advertisers.map((advertiser) => <option value={advertiser.advertiser_id} key={advertiser.advertiser_id}>{advertiser.advertiser_name || advertiser.advertiser_id} · {advertiser.advertiser_id}</option>)}</select><ChevronDown size={15} /></div></label>
    <div className="dc-capability-state">
      <span className={`dc-state-dot ${capability?.authorized && capability.advertiser_allowed ? "online" : "offline"}`} />
      <div><strong>{capability?.authorized && capability.advertiser_allowed ? "聚光授权有效" : capability ? "聚光授权不可用" : "检查聚光授权"}</strong><span>{capability?.missing_scopes.length ? `缺少 ${capability.missing_scopes.join(", ")}` : capability?.contract_version || "-"}</span></div>
    </div>
    <div className={`dc-write-gate ${capability?.media_writes_enabled ? "open" : "closed"}`}>{capability?.media_writes_enabled ? <Route size={15} /> : <LockKeyhole size={15} />}<span>{capability?.media_writes_enabled ? "媒体写入已开启" : "媒体写入关闭"}</span></div>
    <div className="dc-session-actions"><button className="dc-icon-button" type="button" title="刷新广告主与能力" aria-label="刷新广告主与能力" disabled={refreshing} onClick={onRefresh}><RefreshCw size={17} className={refreshing ? "spin" : ""} /></button></div>
  </section>;
}

export default function SelfServeDeliveryConsole() {
  const [view, setView] = useState<ConsoleView>("workspace");
  const [session, setSession] = useState<DeliverySession | null>();
  const [advertiserID, setAdvertiserID] = useState(0);
  const [capability, setCapability] = useState<Capability>();
  const [loadingSession, setLoadingSession] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [notice, setNotice] = useState<NoticeState>(null);

  const applySession = useCallback((next: DeliverySession) => {
    setSession(next);
    setAdvertiserID((current) => next.advertisers.some((item) => item.advertiser_id === current) ? current : next.advertisers[0]?.advertiser_id || 0);
  }, []);

  const loadSession = useCallback(async (quiet = false) => {
    if (!quiet) setLoadingSession(true);
    else setRefreshing(true);
    try {
      const next = await deliveryAPI.session();
      applySession(next); setNotice(null);
    } catch (error) {
      setSession(null); setAdvertiserID(0); setCapability(undefined);
      setNotice({ tone: "error", message: errorMessage(error) });
    } finally { setLoadingSession(false); setRefreshing(false); }
  }, [applySession]);

  const loadCapability = useCallback(async (selectedAdvertiser: number) => {
    if (!selectedAdvertiser) { setCapability(undefined); return; }
    setRefreshing(true);
    try { setCapability(await deliveryAPI.capabilities(selectedAdvertiser)); setNotice(null); }
    catch (error) { setCapability(undefined); setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setRefreshing(false); }
  }, []);

  useEffect(() => { void loadSession(); }, [loadSession]);
  useEffect(() => { if (session && advertiserID) void loadCapability(advertiserID); }, [session, advertiserID, loadCapability]);

  const refresh = async () => {
    await loadSession(true);
    if (advertiserID) await loadCapability(advertiserID);
  };

  return <div className="self-serve-delivery-console">
    <section className="page-heading dc-page-heading">
      <div><h1>自建投流</h1><p>聚光计划、单元、创意、定向、关键词与报表工作台</p></div>
      <a className="dc-openapi-link" href={`${import.meta.env.BASE_URL}api/delivery/openapi.json`} target="_blank" rel="noreferrer">OpenAPI</a>
    </section>

    <nav className="dc-main-tabs" aria-label="自建投流功能">
      {VIEWS.map((item) => <button type="button" key={item.key} className={view === item.key ? "active" : ""} disabled={!session && item.key !== "blueprint"} onClick={() => setView(item.key)}><item.icon size={16} />{item.label}</button>)}
    </nav>

    {loadingSession ? <section className="dc-bootstrap"><LoadingState label="加载投放控制台" /></section> : null}
    {!loadingSession && session ? <SessionBar session={session} advertiserID={advertiserID} capability={capability} refreshing={refreshing} onAdvertiser={setAdvertiserID} onRefresh={() => void refresh()} /> : null}
    <Notice state={notice} onDismiss={() => setNotice(null)} />

    {!loadingSession && !session && view !== "blueprint" ? <section className="dc-no-advertiser"><CircleAlert size={21} /><div><strong>投放服务暂不可用</strong><p>控制台已配置为直接进入，请刷新页面或检查后端服务状态。</p></div></section> : null}
    {!loadingSession && session && session.advertisers.length === 0 && view !== "blueprint" ? <section className="dc-no-advertiser"><CircleAlert size={21} /><div><strong>没有可用广告主</strong><p>请检查聚光 OAuth 授权中是否包含广告主。</p></div></section> : null}
    {session && advertiserID && view === "workspace" ? <DeliveryDraftWorkspace key={advertiserID} advertiserID={advertiserID} actor={session.actor} capability={capability} /> : null}
    {session && advertiserID && view === "data" ? <DeliveryDataWorkspace key={advertiserID} advertiserID={advertiserID} /> : null}
    {session && view === "intelligence" ? <DeliveryIntelligenceLab role={session.actor.role} /> : null}
    {view === "blueprint" ? <div className="dc-blueprint"><Suspense fallback={<LoadingState label="加载设计与边界" />}><SelfServeDeliveryPlan /></Suspense></div> : null}
  </div>;
}
