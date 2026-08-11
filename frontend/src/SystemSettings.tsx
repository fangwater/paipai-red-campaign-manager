import { useCallback, useEffect, useState } from "react";
import { CheckCircle2, Database, LoaderCircle, RefreshCw, Server, Settings, XCircle } from "lucide-react";
import { useNavigate } from "react-router-dom";

type ServiceState = "checking" | "online" | "offline";

type StateMap = {
  core: ServiceState;
  lark: ServiceState;
  spotlight: ServiceState;
};

const initialState: StateMap = { core: "checking", lark: "checking", spotlight: "checking" };
const timeFormatter = new Intl.DateTimeFormat("zh-CN", {
  month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false
});

function StateIcon({ state }: { state: ServiceState }) {
  if (state === "checking") return <LoaderCircle size={17} className="spin" />;
  if (state === "online") return <CheckCircle2 size={17} />;
  return <XCircle size={17} />;
}

function stateText(state: ServiceState): string {
  if (state === "checking") return "检查中";
  if (state === "online") return "正常";
  return "异常";
}

function SystemSettings() {
  const navigate = useNavigate();
  const [states, setStates] = useState<StateMap>(initialState);
  const [checkedAt, setCheckedAt] = useState<Date>();

  const refresh = useCallback(async (signal?: AbortSignal) => {
    setStates(initialState);
    const [core, dandelion, manuscripts, coenzyme, spotlight] = await Promise.allSettled([
      fetch(`${import.meta.env.BASE_URL}healthz`, { signal }),
      fetch(`${import.meta.env.BASE_URL}api/lark/sync/dandelion/status`, { signal }),
      fetch(`${import.meta.env.BASE_URL}api/lark/sync/manuscripts/status`, { signal }),
      fetch(`${import.meta.env.BASE_URL}api/lark/sync/cid/status`, { signal }),
      fetch(`${import.meta.env.BASE_URL}api/xhs-jg/sync/status`, { signal })
    ]);
    if (signal?.aborted) return;
    const responseOK = (result: PromiseSettledResult<Response>) => result.status === "fulfilled" && result.value.ok;
    setStates({
      core: responseOK(core) ? "online" : "offline",
      lark: responseOK(dandelion) && responseOK(manuscripts) && responseOK(coenzyme) ? "online" : "offline",
      spotlight: responseOK(spotlight) ? "online" : "offline"
    });
    setCheckedAt(new Date());
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal);
    return () => controller.abort();
  }, [refresh]);

  const services = [
    { key: "core" as const, label: "数据中台 API", detail: "日报导入与分析查询", icon: Server },
    { key: "lark" as const, label: "飞书数据同步", detail: "蒲公英、服务商稿件与cid数据", icon: Database },
    { key: "spotlight" as const, label: "聚光同步服务", detail: "计划、单元与创意", icon: RefreshCw }
  ];

  return <>
    <section className="page-heading settings-page-heading">
      <div><h1>系统设置</h1><p>运行状态 · 手动任务入口</p></div>
      <button className="outline-button settings-refresh" disabled={Object.values(states).some((state) => state === "checking")} onClick={() => void refresh()}><RefreshCw size={15} />刷新状态</button>
    </section>

    <section className="service-overview">
      <header><div><h2>服务状态</h2><p>{checkedAt ? `检查于 ${timeFormatter.format(checkedAt)}` : "正在检查"}</p></div><Settings size={18} /></header>
      <div className="service-status-list">
        {services.map((service) => {
          const Icon = service.icon;
          const state = states[service.key];
          return <div className="service-status-row" key={service.key}>
            <span className="service-kind"><Icon size={18} /></span><div><strong>{service.label}</strong><span>{service.detail}</span></div>
            <span className={`service-health ${state}`}><StateIcon state={state} />{stateText(state)}</span>
          </div>;
        })}
      </div>
    </section>

    <section className="settings-actions">
      <header><h2>手动任务</h2></header>
      <div><button onClick={() => navigate("/data-sync/cid")}><Database size={18} /><span><strong>飞书数据同步</strong><small>蒲公英、稿件与cid数据</small></span></button>
        <button onClick={() => navigate("/xhs-jg-sync/campaigns")}><RefreshCw size={18} /><span><strong>聚光数据同步</strong><small>计划、单元与创意</small></span></button></div>
    </section>
  </>;
}

export default SystemSettings;
