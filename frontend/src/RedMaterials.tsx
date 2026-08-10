import { useEffect, useState } from "react";
import { AlertCircle, CheckCircle2, ChevronLeft, ChevronRight, CircleDashed, ExternalLink, FileText, Link2, LoaderCircle, Pencil, Search } from "lucide-react";

import ReferenceMaterialEditor from "./ReferenceMaterialEditor";
import StoredManuscriptDialog from "./StoredManuscriptDialog";
import "./red-materials.css";

type ServiceState = "checking" | "online" | "offline";

type ReferenceMaterialItem = {
  reference_note_id: string;
  note_url: string;
  source_note_ids: string[];
  providers: string[];
  usage_count: number;
  has_content: boolean;
  content_source: "manual" | "manuscript" | "";
};

type ReferenceMaterialStats = {
  material_count: number;
  source_note_count: number;
  reference_count: number;
  provider_count: number;
};

type ReferenceMaterialsResult = {
  search: string;
  stats: ReferenceMaterialStats;
  total: number;
  page: number;
  page_size: number;
  items: ReferenceMaterialItem[];
};

const EMPTY_RESULT: ReferenceMaterialsResult = {
  search: "",
  stats: { material_count: 0, source_note_count: 0, reference_count: 0, provider_count: 0 },
  total: 0,
  page: 1,
  page_size: 25,
  items: []
};

const integerFormatter = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });

function RedMaterials({ serviceState }: { serviceState: ServiceState }) {
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<ReferenceMaterialsResult>(EMPTY_RESULT);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selectedManuscriptID, setSelectedManuscriptID] = useState("");
  const [selectedReference, setSelectedReference] = useState<ReferenceMaterialItem | null>(null);
  const [editingReference, setEditingReference] = useState<ReferenceMaterialItem | null>(null);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch(searchInput.trim());
      setPage(1);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ page: String(page), page_size: "25" });
    if (search) params.set("q", search);
    setLoading(true);
    setError("");
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/reference-materials?${params}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: ReferenceMaterialsResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "红薯素材读取失败");
        setResult(payload.data);
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "红薯素材读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [page, search]);

  const pageCount = Math.max(1, Math.ceil(result.total / result.page_size));
  const stats = [
    { label: "素材笔记", value: result.stats.material_count },
    { label: "引用稿件", value: result.stats.source_note_count },
    { label: "引用关系", value: result.stats.reference_count },
    { label: "来源机构", value: result.stats.provider_count }
  ];

  const openReferenceEditor = (item: ReferenceMaterialItem) => {
    setSelectedReference(null);
    setEditingReference(item);
  };

  const handleReferenceSaved = (item: ReferenceMaterialItem) => {
    const updatedItem = { ...item, has_content: true, content_source: "manual" as const };
    setResult((current) => ({ ...current, items: current.items.map((candidate) => candidate.reference_note_id === item.reference_note_id ? updatedItem : candidate) }));
    setEditingReference(null);
    setSelectedReference(updatedItem);
  };

  return <>
    <section className="page-heading red-material-page-heading">
      <div><h1>红薯素材</h1><p>稿件参考笔记库</p></div>
      <div className="heading-status"><span className={`status-dot ${serviceState}`} />{serviceState === "online" ? "素材服务已连接" : serviceState === "offline" ? "素材服务未连接" : "正在检查连接"}</div>
    </section>

    <section className="red-material-toolbar">
      <label className="analysis-search red-material-search"><Search size={16} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索素材 ID、稿件 ID 或机构" /></label>
      <span>{loading ? <LoaderCircle size={16} className="spin" /> : null}{search ? `“${search}”的结果` : "全部素材"}</span>
    </section>

    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <section className="red-material-stats" aria-label="素材统计">
      {stats.map((item) => <div key={item.label}><span>{item.label}</span><strong>{integerFormatter.format(item.value)}</strong></div>)}
    </section>

    <section className="red-material-table-section">
      <header><div><h2>素材目录</h2><p>{integerFormatter.format(result.total)} 条去重参考笔记</p></div>{loading ? <LoaderCircle size={17} className="spin" /> : null}</header>
      <div className="red-material-table-wrap">
        <table className="red-material-table">
          <thead><tr><th>参考笔记</th><th>内容状态</th><th>对应稿件</th><th>来源机构</th><th>引用次数</th><th aria-label="操作" /></tr></thead>
          <tbody>
            {result.items.map((item) => <tr key={item.reference_note_id}>
              <td>{item.has_content
                ? <button className="red-material-identity" type="button" onClick={() => setSelectedReference(item)} title="查看参考内容" aria-label={`查看参考内容 ${item.reference_note_id}`}><span><FileText size={16} /></span><code>{item.reference_note_id}</code></button>
                : <a className="red-material-identity" href={item.note_url} target="_blank" rel="noreferrer" aria-label={`打开红薯素材 ${item.reference_note_id}`}><span><Link2 size={16} /></span><code>{item.reference_note_id}</code></a>}</td>
              <td><span className={`red-material-content-status ${item.has_content ? "filled" : "empty"}`} title={item.has_content ? (item.content_source === "manual" ? "已填充：人工录入" : "已填充：稿件库") : "未填充"}>{item.has_content ? <CheckCircle2 size={13} /> : <CircleDashed size={13} />}{item.has_content ? "已填充" : "未填充"}</span></td>
              <td><div className="red-material-sources">{item.source_note_ids.map((noteID) => <button key={noteID} type="button" onClick={() => setSelectedManuscriptID(noteID)} title={`查看已存稿件 ${noteID}`} aria-label={`查看已存稿件 ${noteID}`}><FileText size={12} /><code>{noteID}</code></button>)}</div></td>
              <td><div className="red-material-providers">{item.providers.length > 0 ? item.providers.map((provider) => <span key={provider}>{provider}</span>) : <small>未标注</small>}</div></td>
              <td><span className="red-material-usage">{integerFormatter.format(item.usage_count)}</span></td>
              <td><div className="red-material-actions">
                <button className="red-material-open" type="button" title={item.has_content ? "编辑参考内容" : "录入参考内容"} aria-label={`${item.has_content ? "编辑" : "录入"}参考内容 ${item.reference_note_id}`} onClick={() => openReferenceEditor(item)}><Pencil size={15} /></button>
                <a className="red-material-open" href={item.note_url} target="_blank" rel="noreferrer" title="打开参考笔记" aria-label={`打开参考笔记 ${item.reference_note_id}`}><ExternalLink size={16} /></a>
              </div></td>
            </tr>)}
            {!loading && result.items.length === 0 ? <tr><td className="red-material-empty" colSpan={6}>暂无符合条件的红薯素材</td></tr> : null}
          </tbody>
        </table>
      </div>
      <footer className="analysis-pagination"><span>第 {result.page}/{pageCount} 页</span><div>
        <button className="icon-button" title="上一页" aria-label="上一页" disabled={page <= 1 || loading} onClick={() => setPage((current) => Math.max(1, current - 1))}><ChevronLeft size={17} /></button>
        <button className="icon-button" title="下一页" aria-label="下一页" disabled={page >= pageCount || loading} onClick={() => setPage((current) => current + 1)}><ChevronRight size={17} /></button>
      </div></footer>
    </section>
    {selectedManuscriptID ? <StoredManuscriptDialog noteID={selectedManuscriptID} onClose={() => setSelectedManuscriptID("")} /> : null}
    {selectedReference ? <StoredManuscriptDialog noteID={selectedReference.reference_note_id} variant="reference" onClose={() => setSelectedReference(null)} onEdit={() => openReferenceEditor(selectedReference)} /> : null}
    {editingReference ? <ReferenceMaterialEditor noteID={editingReference.reference_note_id} hasContent={editingReference.has_content} onClose={() => setEditingReference(null)} onSaved={() => handleReferenceSaved(editingReference)} /> : null}
  </>;
}

export default RedMaterials;
