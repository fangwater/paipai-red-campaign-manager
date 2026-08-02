import { useEffect, useState } from "react";
import { AlertCircle, ChevronLeft, ChevronRight, ExternalLink, FileText, Image as ImageIcon, Link2, LoaderCircle, Search } from "lucide-react";
import { useNavigate } from "react-router-dom";

type ServiceState = "checking" | "online" | "offline";
type EntityType = "note" | "plan";
type SortOption = "roi" | "publish_time" | "payment" | "cost";
type SPUOption = "辅酶" | "磷虾油";

type GuoraiMetrics = {
  total_pay_amount: number | null;
  part_pay_amount: number | null;
  ad_cost: number | null;
  click_count: number | null;
  interaction_count: number | null;
  total_roi: number | null;
};

type GuoraiItem = {
  id: string;
  url: string;
  name: string;
  author_name: string;
  account_name: string;
  publish_time: string;
  picture_url: string;
  spu_id: string;
  spu_name: string;
  tag: string;
  plan_type: string;
  note_type: number;
  linked_note_count: number;
  is_new: boolean;
  metrics: GuoraiMetrics;
};

type GuoraiResult = {
  entity_type: EntityType;
  spu: SPUOption;
  sort: SortOption;
  snapshot: {
    fetch_id: number;
    entity_type: EntityType;
    snapshot_date: string;
    window_start: string;
    window_end: string;
    window_days: number;
    source_cutoff_date: string;
    brand_name: string;
    attribution_type: string;
    attribution_model: string;
    attribution_window_days: number;
    row_count: number;
    finished_at: string | null;
  } | null;
  summary: {
    item_count: number;
    account_count: number;
    linked_count: number;
    new_count: number;
    metric_item_count: number;
    metrics: GuoraiMetrics;
  };
  total: number;
  page: number;
  page_size: number;
  items: GuoraiItem[];
};

const SORT_OPTIONS: Array<{ value: SortOption; label: string }> = [
  { value: "roi", label: "ROI" },
  { value: "publish_time", label: "发布时间" },
  { value: "payment", label: "成交金额" },
  { value: "cost", label: "投放消耗" }
];
const integerFormatter = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });
const moneyFormatter = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

function money(value: number | null): string {
  return value === null ? "--" : "¥" + moneyFormatter.format(value);
}

function integer(value: number | null): string {
  return value === null ? "--" : integerFormatter.format(value);
}

function decimal(value: number | null): string {
  return value === null ? "--" : value.toFixed(2);
}

function safeImageURL(value: string): string | null {
  try {
    const url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    if (url.protocol === "http:") url.protocol = "https:";
    return url.toString();
  } catch {
    return null;
  }
}

function snapshotTime(value: string | null): string {
  if (!value) return "--";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN", { hour12: false });
}

function GuoraiData({ serviceState }: { serviceState: ServiceState }) {
  const navigate = useNavigate();
  const [entityType, setEntityType] = useState<EntityType>("note");
  const [spu, setSPU] = useState<SPUOption>("辅酶");
  const [sort, setSort] = useState<SortOption>("roi");
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<GuoraiResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch(searchInput.trim());
      setPage(1);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ type: entityType, sort, page: String(page), page_size: "25" });
    params.set("spu", spu);
    if (search) params.set("q", search);
    setLoading(true);
    setError("");
    fetch(import.meta.env.BASE_URL + "api/analytics/guorai/latest?" + params, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: GuoraiResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "薯量数据读取失败");
        setResult(payload.data);
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "薯量数据读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [entityType, page, search, sort, spu]);

  const pageCount = Math.max(1, Math.ceil((result?.total ?? 0) / (result?.page_size ?? 25)));
  const snapshot = result?.snapshot;
  const summary = result?.summary;
  const summaryCards = entityType === "note"
    ? [
      { label: "笔记总数", value: summary?.item_count ?? 0 },
      { label: "达人账号", value: summary?.account_count ?? 0 },
      { label: "关联 SPU", value: summary?.linked_count ?? 0 },
      { label: "窗口内新增", value: summary?.new_count ?? 0 }
    ]
    : [
      { label: "计划总数", value: summary?.item_count ?? 0 },
      { label: "投放账号", value: summary?.account_count ?? 0 },
      { label: "关联笔记", value: summary?.linked_count ?? 0 },
      { label: "窗口内新增", value: summary?.new_count ?? 0 }
    ];

  return <>
    <section className="page-heading guorai-page-heading">
      <div><h1>薯量数据</h1><p>最新同步快照 · {entityType === "note" ? "近 14 日笔记" : "近 14 日计划"}</p></div>
      <div className="heading-status"><span className={"status-dot " + serviceState} />{serviceState === "online" ? "数据服务已连接" : serviceState === "offline" ? "数据服务未连接" : "正在检查连接"}</div>
    </section>

    <section className="guorai-toolbar">
      <div className="guorai-entity-switch" aria-label="薯量数据类型">
        <button className={entityType === "note" ? "active" : ""} onClick={() => { setEntityType("note"); setPage(1); }}>笔记</button>
        <button className={entityType === "plan" ? "active" : ""} onClick={() => { setEntityType("plan"); setPage(1); }}>计划</button>
      </div>
      <label className="analysis-search guorai-search"><Search size={16} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder={entityType === "note" ? "搜索笔记、达人或 SPU" : "搜索计划、账号或类型"} /></label>
      <label className="guorai-sort guorai-spu-filter"><span>SPU</span><select aria-label="SPU" value={spu} onChange={(event) => { setSPU(event.target.value as SPUOption); setPage(1); }}>
        <option value="辅酶">辅酶</option><option value="磷虾油">磷虾油</option>
      </select></label>
      <label className="guorai-sort guorai-order-filter"><span>排序</span><select value={sort} onChange={(event) => { setSort(event.target.value as SortOption); setPage(1); }}>
        {SORT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select></label>
    </section>

    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <section className="guorai-snapshot-strip">
      <div><span>快照日期</span><strong>{snapshot?.snapshot_date || "--"}</strong></div>
      <div><span>统计窗口</span><strong>{snapshot ? snapshot.window_start + " - " + snapshot.window_end : "--"}</strong></div>
      <div><span>品牌</span><strong>{snapshot?.brand_name || "--"}</strong></div>
      <div><span>同步完成</span><strong>{snapshotTime(snapshot?.finished_at ?? null)}</strong></div>
      {loading ? <LoaderCircle size={17} className="spin" /> : null}
    </section>

    <section className="guorai-summary-grid">
      {summaryCards.map((card) => <article key={card.label}><span>{card.label}</span><strong>{integerFormatter.format(card.value)}</strong></article>)}
    </section>

    <section className="guorai-metric-strip">
      <div><span>总成交金额</span><strong>{money(summary?.metrics.total_pay_amount ?? null)}</strong></div>
      <div><span>广告消耗</span><strong>{money(summary?.metrics.ad_cost ?? null)}</strong></div>
      <div><span>总 ROI</span><strong>{decimal(summary?.metrics.total_roi ?? null)}</strong></div>
      <p>{summary?.metric_item_count ? summary.metric_item_count + " 条记录包含投放指标" : "当前快照未返回投放指标"}</p>
    </section>

    <section className="guorai-table-section">
      <header><div><h2>{spu}{entityType === "note" ? "笔记明细" : "计划明细"}</h2><p>{result?.total.toLocaleString() ?? 0} 条，按{SORT_OPTIONS.find((option) => option.value === sort)?.label}倒序</p></div>{loading ? <LoaderCircle size={17} className="spin" /> : null}</header>
      <div className="guorai-table-wrap"><table className="guorai-table"><thead><tr>
        <th>{entityType === "note" ? "笔记" : "计划"}</th><th>{entityType === "note" ? "达人" : "投放账号"}</th><th>{entityType === "note" ? "SPU" : "计划类型"}</th><th>发布时间</th>
        {entityType === "plan" ? <th>关联笔记</th> : null}<th>总成交金额</th><th>广告消耗</th><th>点击</th><th>互动</th><th>ROI</th>
      </tr></thead><tbody>
        {result?.items.map((item) => {
          const picture = safeImageURL(item.picture_url);
          const noteURL = entityType === "note" ? item.url : "";
          const cover = <span className={"guorai-cover " + (entityType === "plan" ? "plan" : "")}>{entityType === "note" ? <ImageIcon size={17} /> : <FileText size={17} />}{picture && entityType === "note" ? <img src={picture} alt="" onError={(event) => event.currentTarget.remove()} /> : null}</span>;
          return <tr key={item.id}>
            <td><div className="guorai-identity">
              {noteURL ? <a className="guorai-cover-link" href={noteURL} target="_blank" rel="noreferrer" aria-label={"打开笔记封面：" + (item.name || item.id)}>{cover}</a> : cover}
              <span>{noteURL ? <a className="guorai-title-link" href={noteURL} target="_blank" rel="noreferrer" aria-label={"打开笔记：" + (item.name || item.id)}><strong title={item.name}>{item.name || item.id}</strong><ExternalLink size={12} /></a> : <strong title={item.name}>{item.name || item.id}</strong>}<small>{item.id}{item.is_new ? <em>新增</em> : null}</small></span>
            </div></td>
            <td>{(entityType === "note" ? item.author_name : item.account_name) || "--"}</td>
            <td>{entityType === "note" ? (item.spu_name || item.spu_id || "--") : (item.plan_type || item.tag || "--")}</td>
            <td>{item.publish_time || "--"}</td>
            {entityType === "plan" ? <td>{item.linked_note_count > 0
              ? <button className="linked-count linked-count-button" title="查看关联笔记分析" aria-label={`查看${item.name || item.id}的${item.linked_note_count}条关联笔记分析`} onClick={() => {
                const params = new URLSearchParams({ plan_id: item.id, plan_name: item.name || item.id, window: "all" });
                navigate(`/note-campaign-analysis?${params}`);
              }}><Link2 size={13} />{integerFormatter.format(item.linked_note_count)}</button>
              : <span className="linked-count"><Link2 size={13} />0</span>}</td> : null}
            <td>{money(item.metrics.total_pay_amount)}</td><td>{money(item.metrics.ad_cost)}</td><td>{integer(item.metrics.click_count)}</td><td>{integer(item.metrics.interaction_count)}</td><td>{decimal(item.metrics.total_roi)}</td>
          </tr>;
        })}
        {!loading && result?.items.length === 0 ? <tr><td className="guorai-empty" colSpan={entityType === "plan" ? 10 : 9}>暂无符合条件的数据</td></tr> : null}
      </tbody></table></div>
      <footer className="analysis-pagination"><span>第 {result?.page ?? page}/{pageCount} 页</span><div>
        <button className="icon-button" title="上一页" aria-label="上一页" disabled={page <= 1 || loading} onClick={() => setPage((current) => Math.max(1, current - 1))}><ChevronLeft size={17} /></button>
        <button className="icon-button" title="下一页" aria-label="下一页" disabled={page >= pageCount || loading} onClick={() => setPage((current) => current + 1)}><ChevronRight size={17} /></button>
      </div></footer>
    </section>
  </>;
}

export default GuoraiData;
