import { useEffect, useState } from "react";
import { AlertCircle, CheckCircle2, ChevronLeft, ChevronRight, CircleDashed, ExternalLink, FileText, Link2, LoaderCircle, Pencil, RotateCcw, Search } from "lucide-react";

import ReferenceMaterialEditor from "./ReferenceMaterialEditor";
import StoredManuscriptDialog from "./StoredManuscriptDialog";
import "./red-materials.css";

type ServiceState = "checking" | "online" | "offline";

type ReferenceMaterialSource = {
  note_id: string;
  title: string;
  url: string;
};

type ReferenceMaterialTags = {
  note_type: string[];
  cover_type: string[];
  commercial_intensity: string[];
  audience: string[];
  user_scenario: string[];
};

type ReferenceMaterialFilters = {
  provider: string;
  note_type: string;
  cover_type: string;
  commercial_intensity: string;
  audience: string;
  user_scenario: string;
};

type ReferenceMaterialFilterOptions = ReferenceMaterialTags & {
  providers: string[];
};

type ReferenceMaterialItem = {
  reference_note_id: string;
  note_url: string;
  source_note_ids: string[];
  source_manuscripts?: ReferenceMaterialSource[];
  providers: string[];
  tags: ReferenceMaterialTags;
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
  filters: ReferenceMaterialFilters;
  filter_options: ReferenceMaterialFilterOptions;
  stats: ReferenceMaterialStats;
  total: number;
  page: number;
  page_size: number;
  items: ReferenceMaterialItem[];
};

type FilterKey = keyof ReferenceMaterialFilters;
type FilterField = {
  key: FilterKey;
  optionKey: keyof ReferenceMaterialFilterOptions;
  label: string;
};

const EMPTY_FILTERS: ReferenceMaterialFilters = {
  provider: "",
  note_type: "",
  cover_type: "",
  commercial_intensity: "",
  audience: "",
  user_scenario: ""
};

const EMPTY_FILTER_OPTIONS: ReferenceMaterialFilterOptions = {
  providers: [],
  note_type: [],
  cover_type: [],
  commercial_intensity: [],
  audience: [],
  user_scenario: []
};

const TAG_FIELDS: Array<{ key: keyof ReferenceMaterialTags; label: string }> = [
  { key: "note_type", label: "内容类型" },
  { key: "audience", label: "对话人群" },
  { key: "user_scenario", label: "用户场景" },
  { key: "cover_type", label: "封面类型" },
  { key: "commercial_intensity", label: "商业强度" }
];

const FILTER_FIELDS: FilterField[] = [
  { key: "provider", optionKey: "providers", label: "来源机构" },
  ...TAG_FIELDS.map((field) => ({ ...field, optionKey: field.key }))
];

const EMPTY_RESULT: ReferenceMaterialsResult = {
  search: "",
  filters: EMPTY_FILTERS,
  filter_options: EMPTY_FILTER_OPTIONS,
  stats: { material_count: 0, source_note_count: 0, reference_count: 0, provider_count: 0 },
  total: 0,
  page: 1,
  page_size: 25,
  items: []
};

const integerFormatter = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });

function MaterialSources({ item, onOpen }: { item: ReferenceMaterialItem; onOpen: (source: ReferenceMaterialSource) => void }) {
  const sources = item.source_manuscripts?.length
    ? item.source_manuscripts
    : item.source_note_ids.map((noteID) => ({ note_id: noteID, title: "", url: "" }));
  return <div className="red-material-sources">{sources.map((source) => {
    const label = source.title.trim() || source.note_id;
    return <div className="red-material-source" key={source.note_id}>
      <button type="button" onClick={() => onOpen(source)} title={label} aria-label={`查看已存稿件 ${label}`}>
        <FileText size={13} /><span>{label}</span>
      </button>
      {source.url ? <a href={source.url} target="_blank" rel="noreferrer" title="打开飞书稿件" aria-label={`打开飞书稿件 ${label}`}>
        <ExternalLink size={13} />
      </a> : null}
    </div>;
  })}</div>;
}

function MaterialTags({ tags, filters, onSelect }: {
  tags: ReferenceMaterialTags;
  filters: ReferenceMaterialFilters;
  onSelect: (key: FilterKey, value: string) => void;
}) {
  const taggedFields = TAG_FIELDS.filter((field) => tags[field.key].length > 0);
  if (taggedFields.length === 0) return <small className="red-material-untagged">未标注</small>;
  return <div className="red-material-tag-groups">{taggedFields.map((field) => <div key={field.key}>
    <span>{field.label}</span>
    <div>{tags[field.key].map((value) => {
      const active = filters[field.key] === value;
      return <button
        type="button"
        className={active ? "active" : ""}
        key={value}
        title={active ? `取消筛选：${value}` : `按${field.label}筛选：${value}`}
        aria-label={active ? `取消${field.label}筛选 ${value}` : `按${field.label}筛选 ${value}`}
        onClick={() => onSelect(field.key, value)}
      >{value}</button>;
    })}</div>
  </div>)}</div>;
}

function RedMaterials({ serviceState }: { serviceState: ServiceState }) {
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<ReferenceMaterialFilters>(EMPTY_FILTERS);
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<ReferenceMaterialsResult>(EMPTY_RESULT);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selectedManuscript, setSelectedManuscript] = useState<ReferenceMaterialSource | null>(null);
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
    for (const field of FILTER_FIELDS) {
      const value = filters[field.key];
      if (value) params.set(field.key, value);
    }
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
  }, [
    page,
    search,
    filters.provider,
    filters.note_type,
    filters.cover_type,
    filters.commercial_intensity,
    filters.audience,
    filters.user_scenario
  ]);

  const pageCount = Math.max(1, Math.ceil(result.total / result.page_size));
  const stats = [
    { label: "素材笔记", value: result.stats.material_count },
    { label: "引用稿件", value: result.stats.source_note_count },
    { label: "引用关系", value: result.stats.reference_count },
    { label: "来源机构", value: result.stats.provider_count }
  ];
  const activeFilterCount = FILTER_FIELDS.reduce((count, field) => count + Number(Boolean(filters[field.key])), 0);

  const updateFilter = (key: FilterKey, value: string) => {
    setFilters((current) => ({ ...current, [key]: value }));
    setPage(1);
  };

  const toggleFilter = (key: FilterKey, value: string) => {
    setFilters((current) => ({
      ...current,
      [key]: current[key] === value ? "" : value
    }));
    setPage(1);
  };

  const clearFilters = () => {
    setFilters(EMPTY_FILTERS);
    setPage(1);
  };

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
      <div className="red-material-toolbar-primary">
        <label className="analysis-search red-material-search"><Search size={16} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索素材 ID、稿件标题、机构或标签" /></label>
        <span>{loading ? <LoaderCircle size={16} className="spin" /> : null}{search ? `“${search}”` : "全部素材"}{activeFilterCount > 0 ? ` · ${activeFilterCount} 个筛选` : ""}</span>
      </div>
      <div className="red-material-filters" aria-label="素材标签筛选">
        {FILTER_FIELDS.map((field) => <label className="red-material-filter" key={field.key}>
          <span>{field.label}</span>
          <select value={filters[field.key]} onChange={(event) => updateFilter(field.key, event.target.value)}>
            <option value="">全部</option>
            {result.filter_options[field.optionKey].map((value) => <option value={value} key={value}>{value}</option>)}
          </select>
        </label>)}
        <button
          className="red-material-filter-reset"
          type="button"
          title="清除全部筛选"
          aria-label="清除全部筛选"
          disabled={activeFilterCount === 0}
          onClick={clearFilters}
        ><RotateCcw size={15} /></button>
      </div>
    </section>

    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <section className="red-material-stats" aria-label="素材统计">
      {stats.map((item) => <div key={item.label}><span>{item.label}</span><strong>{integerFormatter.format(item.value)}</strong></div>)}
    </section>

    <section className="red-material-table-section">
      <header><div><h2>素材目录</h2><p>{integerFormatter.format(result.total)} 条去重参考笔记</p></div>{loading ? <LoaderCircle size={17} className="spin" /> : null}</header>
      <div className="red-material-table-wrap">
        <table className="red-material-table">
          <thead><tr><th>参考笔记</th><th>内容状态</th><th>对应稿件</th><th>稿件标签</th><th>来源机构</th><th>引用次数</th><th aria-label="操作" /></tr></thead>
          <tbody>
            {result.items.map((item) => <tr key={item.reference_note_id}>
              <td>{item.has_content
                ? <button className="red-material-identity" type="button" onClick={() => setSelectedReference(item)} title="查看参考内容" aria-label={`查看参考内容 ${item.reference_note_id}`}><span><FileText size={16} /></span><code>{item.reference_note_id}</code></button>
                : <a className="red-material-identity" href={item.note_url} target="_blank" rel="noreferrer" aria-label={`打开红薯素材 ${item.reference_note_id}`}><span><Link2 size={16} /></span><code>{item.reference_note_id}</code></a>}</td>
              <td><span className={`red-material-content-status ${item.has_content ? "filled" : "empty"}`} title={item.has_content ? (item.content_source === "manual" ? "已填充：人工录入" : "已填充：稿件库") : "未填充"}>{item.has_content ? <CheckCircle2 size={13} /> : <CircleDashed size={13} />}{item.has_content ? "已填充" : "未填充"}</span></td>
              <td><MaterialSources item={item} onOpen={setSelectedManuscript} /></td>
              <td><MaterialTags tags={item.tags} filters={filters} onSelect={toggleFilter} /></td>
              <td><div className="red-material-providers">{item.providers.length > 0 ? item.providers.map((provider) => {
                const active = filters.provider === provider;
                return <button
                  type="button"
                  className={active ? "active" : ""}
                  key={provider}
                  title={active ? `取消机构筛选：${provider}` : `按机构筛选：${provider}`}
                  aria-label={active ? `取消机构筛选 ${provider}` : `按机构筛选 ${provider}`}
                  onClick={() => toggleFilter("provider", provider)}
                >{provider}</button>;
              }) : <small>未标注</small>}</div></td>
              <td><span className="red-material-usage">{integerFormatter.format(item.usage_count)}</span></td>
              <td><div className="red-material-actions">
                <button className="red-material-open" type="button" title={item.has_content ? "编辑参考内容" : "录入参考内容"} aria-label={`${item.has_content ? "编辑" : "录入"}参考内容 ${item.reference_note_id}`} onClick={() => openReferenceEditor(item)}><Pencil size={15} /></button>
                <a className="red-material-open" href={item.note_url} target="_blank" rel="noreferrer" title="打开参考笔记" aria-label={`打开参考笔记 ${item.reference_note_id}`}><ExternalLink size={16} /></a>
              </div></td>
            </tr>)}
            {!loading && result.items.length === 0 ? <tr><td className="red-material-empty" colSpan={7}>暂无符合条件的红薯素材</td></tr> : null}
          </tbody>
        </table>
      </div>
      <footer className="analysis-pagination"><span>第 {result.page}/{pageCount} 页</span><div>
        <button className="icon-button" title="上一页" aria-label="上一页" disabled={page <= 1 || loading} onClick={() => setPage((current) => Math.max(1, current - 1))}><ChevronLeft size={17} /></button>
        <button className="icon-button" title="下一页" aria-label="下一页" disabled={page >= pageCount || loading} onClick={() => setPage((current) => current + 1)}><ChevronRight size={17} /></button>
      </div></footer>
    </section>
    {selectedManuscript ? <StoredManuscriptDialog noteID={selectedManuscript.note_id} manuscriptTitle={selectedManuscript.title} onClose={() => setSelectedManuscript(null)} /> : null}
    {selectedReference ? <StoredManuscriptDialog noteID={selectedReference.reference_note_id} variant="reference" onClose={() => setSelectedReference(null)} onEdit={() => openReferenceEditor(selectedReference)} /> : null}
    {editingReference ? <ReferenceMaterialEditor noteID={editingReference.reference_note_id} hasContent={editingReference.has_content} onClose={() => setEditingReference(null)} onSaved={() => handleReferenceSaved(editingReference)} /> : null}
  </>;
}

export default RedMaterials;
