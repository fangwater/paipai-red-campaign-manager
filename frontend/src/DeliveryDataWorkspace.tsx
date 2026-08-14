import {
  BarChart3, Boxes, CalendarRange, ClipboardCopy, Database, FileSearch, Filter, Layers3, LoaderCircle,
  RefreshCw, Search, SlidersHorizontal, Table2, Target, Wrench
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { deliveryAPI, type Assets, type CandidateNote, type GatewayResponse, type JSONObject } from "./delivery-api";
import { AlertLine, EmptyState, JsonOutput, LoadingState, Notice, SectionTitle, errorMessage, formatFen, type NoticeState } from "./delivery-ui";

type Props = { advertiserID: number };
type View = "assets" | "platform" | "reports";

type ToolDefinition = {
  key: string;
  label: string;
  group: string;
  path: string;
  discriminator?: [string, string];
  payload: JSONObject;
};

const TOOLS: ToolDefinition[] = [
  { key: "asset-notes", label: "平台笔记", group: "资产", path: "/assets/platform", discriminator: ["asset_type", "notes"], payload: { page_num: 1, page_size: 20 } },
  { key: "asset-spus", label: "SPU", group: "资产", path: "/assets/platform", discriminator: ["asset_type", "spus"], payload: { page_num: 1, page_size: 20 } },
  { key: "asset-qual", label: "资质", group: "资产", path: "/assets/platform", discriminator: ["asset_type", "qualifications"], payload: { page_num: 1, page_size: 20 } },
  { key: "asset-events", label: "事件资产", group: "资产", path: "/assets/platform", discriminator: ["asset_type", "events"], payload: { page_num: 1, page_size: 20 } },
  { key: "targets", label: "可用定向", group: "策略工具", path: "/target-options", payload: { promotion_target: 1 } },
  { key: "audience", label: "人群预估", group: "策略工具", path: "/audience-estimates", payload: { promotion_target: 1, target_type: 1, target: { gender: "all", age: "all", device: "all" } } },
  { key: "keyword-recommend", label: "关键词推荐", group: "关键词", path: "/keyword-candidates", discriminator: ["source", "recommend"], payload: { request_type: "keyword", keywords: [] } },
  { key: "word-bags", label: "关键词词包", group: "关键词", path: "/keyword-candidates", discriminator: ["source", "word_bags"], payload: { page_num: 1, page_size: 20 } },
  { key: "negative", label: "否定词", group: "关键词", path: "/negative-keywords", discriminator: ["action", "list"], payload: { page_num: 1, page_size: 20 } },
  { key: "campaigns", label: "推广计划", group: "媒体对象", path: "/campaigns/query", payload: { page_num: 1, page_size: 20 } },
  { key: "units", label: "广告单元", group: "媒体对象", path: "/units/query", payload: { page_num: 1, page_size: 20 } },
  { key: "creativities", label: "创意", group: "媒体对象", path: "/creativities/query", payload: { page_num: 1, page_size: 20 } }
];

function findRows(value: unknown, depth = 0): Record<string, unknown>[] | null {
  if (depth > 4) return null;
  if (Array.isArray(value)) {
    if (value.length === 0) return [];
    if (value.every((item) => item && typeof item === "object" && !Array.isArray(item))) return value as Record<string, unknown>[];
    return null;
  }
  if (!value || typeof value !== "object") return null;
  const object = value as Record<string, unknown>;
  for (const key of ["list", "items", "rows", "data", "records", "result"]) {
    if (!(key in object)) continue;
    const found = findRows(object[key], depth + 1);
    if (found !== null) return found;
  }
  for (const child of Object.values(object)) {
    const found = findRows(child, depth + 1);
    if (found !== null) return found;
  }
  return null;
}

function displayCell(value: unknown): string {
  if (value === null || value === undefined || value === "") return "-";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function ResultTable({ value }: { value: unknown }) {
  const rows = useMemo(() => findRows(value), [value]);
  const columns = useMemo(() => {
    if (!rows?.length) return [];
    const keys: string[] = [];
    for (const row of rows.slice(0, 20)) for (const key of Object.keys(row)) if (!keys.includes(key)) keys.push(key);
    return keys.slice(0, 12);
  }, [rows]);
  if (!rows?.length || columns.length === 0) return <JsonOutput value={value} />;
  return <div className="dc-result-table-wrap"><table className="dc-table dc-result-table"><thead><tr>{columns.map((column) => <th key={column}>{column}</th>)}</tr></thead><tbody>{rows.slice(0, 200).map((row, index) => <tr key={index}>{columns.map((column) => <td key={column} title={displayCell(row[column])}>{displayCell(row[column])}</td>)}</tr>)}</tbody></table>{rows.length > 200 ? <p className="dc-table-note">仅展示前 200 条，共 {rows.length} 条</p> : null}</div>;
}

function AssetsView({ advertiserID }: Props) {
  const [search, setSearch] = useState("");
  const [assets, setAssets] = useState<Assets>();
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<NoticeState>(null);
  const load = useCallback(async () => {
    setLoading(true); setNotice(null);
    try { setAssets(await deliveryAPI.assets(advertiserID, search, 100)); }
    catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setLoading(false); }
  }, [advertiserID, search]);
  useEffect(() => { void load(); }, [advertiserID]); // eslint-disable-line react-hooks/exhaustive-deps
  const copy = async (note: CandidateNote) => {
    try { await navigator.clipboard.writeText(note.note_id); setNotice({ tone: "success", message: `已复制 ${note.note_id}` }); }
    catch { setNotice({ tone: "error", message: "浏览器未允许复制，请手动选择笔记 ID" }); }
  };
  return <section className="dc-data-section">
    <SectionTitle icon={Database} title="本地稿件资产" meta={assets ? `${assets.count} 条候选 · 已关联历史投放特征` : "稿件目录与历史表现"} actions={<form className="dc-search-form" onSubmit={(event) => { event.preventDefault(); void load(); }}><Search size={15} /><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="标题、内容或笔记 ID" aria-label="搜索本地稿件" /><button type="submit" className="dc-action-button secondary" disabled={loading}>{loading ? <LoaderCircle className="spin" size={15} /> : <Search size={15} />}查询</button></form>} />
    <Notice state={notice} onDismiss={() => setNotice(null)} />
    {loading && !assets ? <LoadingState label="加载稿件资产" /> : assets?.notes.length ? <div className="dc-table-wrap"><table className="dc-table dc-assets-table"><thead><tr><th>稿件</th><th>受众 / 场景</th><th>历史消耗</th><th>回搜人数</th><th>回搜成本</th><th>创意数</th><th /></tr></thead><tbody>{assets.notes.map((note) => <tr key={note.note_id}><td><div className="dc-note-cell"><strong>{note.title || "未命名稿件"}</strong><code>{note.note_id}</code><span>{note.published ? "已发布" : "未发布"}</span></div></td><td><div className="dc-tag-lines"><span>{note.audience.join("、") || "-"}</span><small>{[...note.scenarios, ...note.note_types].join("、") || "-"}</small></div></td><td>{formatFen(Math.round(note.historical_spend * 100))}</td><td>{note.historical_search_users}</td><td>{note.historical_search_cost === undefined ? "-" : `¥${note.historical_search_cost.toFixed(2)}`}</td><td>{note.creativity_count}</td><td><button className="dc-icon-button" type="button" title="复制笔记 ID" aria-label={`复制 ${note.note_id}`} onClick={() => void copy(note)}><ClipboardCopy size={15} /></button></td></tr>)}</tbody></table></div> : <EmptyState icon={FileSearch} title="没有匹配稿件" />}
  </section>;
}

function PlatformView({ advertiserID }: Props) {
  const [selectedKey, setSelectedKey] = useState(TOOLS[0].key);
  const selected = TOOLS.find((item) => item.key === selectedKey) || TOOLS[0];
  const [payloadText, setPayloadText] = useState(JSON.stringify(selected.payload, null, 2));
  const [result, setResult] = useState<GatewayResponse>();
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<NoticeState>(null);
  const select = (key: string) => {
    const definition = TOOLS.find((item) => item.key === key) || TOOLS[0];
    setSelectedKey(key); setPayloadText(JSON.stringify(definition.payload, null, 2)); setResult(undefined); setNotice(null);
  };
  const run = async () => {
    setLoading(true); setNotice(null);
    try {
      const payload = JSON.parse(payloadText) as JSONObject;
      const body: JSONObject = { advertiser_id: advertiserID, payload };
      if (selected.discriminator) body[selected.discriminator[0]] = selected.discriminator[1];
      setResult(await deliveryAPI.platformTool(selected.path, body));
      setNotice({ tone: "success", message: `${selected.label}查询完成` });
    } catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setLoading(false); }
  };
  const groups = Array.from(new Set(TOOLS.map((tool) => tool.group)));
  return <div className="dc-platform-layout">
    <aside className="dc-tool-menu">{groups.map((group) => <div key={group}><strong>{group}</strong>{TOOLS.filter((tool) => tool.group === group).map((tool) => <button type="button" className={tool.key === selectedKey ? "active" : ""} key={tool.key} onClick={() => select(tool.key)}>{tool.group === "媒体对象" ? <Layers3 size={15} /> : tool.group === "策略工具" ? <Target size={15} /> : tool.group === "关键词" ? <SlidersHorizontal size={15} /> : <Boxes size={15} />}{tool.label}</button>)}</div>)}</aside>
    <section className="dc-data-section dc-platform-panel">
      <SectionTitle icon={Wrench} title={selected.label} meta={`${selected.path} · 只读白名单`} actions={<button className="dc-action-button primary" type="button" disabled={loading} onClick={() => void run()}>{loading ? <LoaderCircle size={15} className="spin" /> : <RefreshCw size={15} />}执行查询</button>} />
      <Notice state={notice} onDismiss={() => setNotice(null)} />
      <div className="dc-query-layout"><div><label className="dc-field"><span>请求 payload</span><textarea className="dc-tool-editor" value={payloadText} spellCheck={false} onChange={(event) => setPayloadText(event.target.value)} /></label></div><div className="dc-query-result"><header><strong>返回数据</strong>{result ? <span>{result.latency_ms} ms · {result.request_id || result.request_hash.slice(0, 12)}</span> : null}</header>{loading ? <LoadingState label="请求聚光接口" /> : result ? <ResultTable value={result.data} /> : <EmptyState icon={Table2} title="尚未执行查询" />}</div></div>
    </section>
  </div>;
}

function localDate(offsetDays = 0): string {
  const date = new Date();
  date.setDate(date.getDate() + offsetDays);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

function ReportsView({ advertiserID }: Props) {
  const [level, setLevel] = useState("campaign");
  const [realtime, setRealtime] = useState(false);
  const [startDate, setStartDate] = useState(localDate(-7));
  const [endDate, setEndDate] = useState(localDate());
  const [pageSize, setPageSize] = useState(100);
  const [splitColumns, setSplitColumns] = useState("");
  const [filters, setFilters] = useState("{}");
  const [result, setResult] = useState<JSONObject>();
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<NoticeState>(null);
  const run = async () => {
    setLoading(true); setNotice(null);
    try {
      const filterObject = JSON.parse(filters) as JSONObject;
      const data = await deliveryAPI.performance({
        advertiser_id: advertiserID, level, realtime, start_date: startDate, end_date: endDate,
        page: 1, page_size: pageSize,
        split_columns: splitColumns.split(",").map((item) => item.trim()).filter(Boolean), filters: filterObject
      });
      setResult(data); setNotice({ tone: "success", message: "报表查询完成并已保存快照" });
    } catch (error) { setNotice({ tone: "error", message: errorMessage(error) }); }
    finally { setLoading(false); }
  };
  return <section className="dc-data-section">
    <SectionTitle icon={BarChart3} title="聚光效果报表" meta="账户、计划、单元、创意、关键词五层数据" actions={<button className="dc-action-button primary" type="button" disabled={loading} onClick={() => void run()}>{loading ? <LoaderCircle size={15} className="spin" /> : <BarChart3 size={15} />}查询报表</button>} />
    <Notice state={notice} onDismiss={() => setNotice(null)} />
    <div className="dc-report-filters">
      <label className="dc-field"><span>数据层级</span><select value={level} onChange={(event) => setLevel(event.target.value)}><option value="account">账户</option><option value="campaign">推广计划</option><option value="unit">广告单元</option><option value="creativity">创意</option><option value="keyword">关键词</option></select></label>
      <label className="dc-field"><span>开始日期</span><input type="date" value={startDate} onChange={(event) => setStartDate(event.target.value)} /></label>
      <label className="dc-field"><span>结束日期</span><input type="date" value={endDate} onChange={(event) => setEndDate(event.target.value)} /></label>
      <label className="dc-field"><span>每页条数</span><input type="number" min={1} max={1000} value={pageSize} onChange={(event) => setPageSize(Number(event.target.value) || 100)} /></label>
      <label className="dc-check-field"><input type="checkbox" checked={realtime} onChange={(event) => setRealtime(event.target.checked)} /><span>实时数据</span></label>
      <label className="dc-field grow"><span>拆分字段</span><input value={splitColumns} disabled={realtime} placeholder="逗号分隔" onChange={(event) => setSplitColumns(event.target.value)} /></label>
    </div>
    <details className="dc-filter-details"><summary><Filter size={15} />高级过滤条件</summary><label className="dc-field"><span>filters JSON</span><textarea rows={5} value={filters} spellCheck={false} onChange={(event) => setFilters(event.target.value)} /></label></details>
    <div className="dc-report-result"><header><CalendarRange size={16} /><strong>{startDate} 至 {endDate}</strong><span>{realtime ? "实时" : "离线"} · {level}</span></header>{loading ? <LoadingState label="获取聚光报表" /> : result ? <ResultTable value={result} /> : <EmptyState icon={BarChart3} title="暂无报表结果" />}</div>
  </section>;
}

export default function DeliveryDataWorkspace({ advertiserID }: Props) {
  const [view, setView] = useState<View>("assets");
  return <div className="dc-data-workspace">
    <nav className="dc-subnav dc-data-nav" aria-label="资产与报表">
      <button className={view === "assets" ? "active" : ""} onClick={() => setView("assets")}><Database size={16} />本地资产</button>
      <button className={view === "platform" ? "active" : ""} onClick={() => setView("platform")}><Wrench size={16} />平台查询</button>
      <button className={view === "reports" ? "active" : ""} onClick={() => setView("reports")}><BarChart3 size={16} />效果报表</button>
    </nav>
    {view === "assets" ? <AssetsView advertiserID={advertiserID} /> : null}
    {view === "platform" ? <PlatformView advertiserID={advertiserID} /> : null}
    {view === "reports" ? <ReportsView advertiserID={advertiserID} /> : null}
  </div>;
}
