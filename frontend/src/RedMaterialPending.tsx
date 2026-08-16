import { FormEvent, useEffect, useState } from "react";
import { AlertCircle, CheckCircle2, ChevronLeft, ChevronRight, ExternalLink, LoaderCircle, Pencil, Search, Tags } from "lucide-react";
import { useNavigate } from "react-router-dom";

import "./red-materials.css";

type ServiceState = "checking" | "online" | "offline";

type ManualMaterialImage = {
  asset_id: string;
  width: number;
  height: number;
};

type ManualMaterialTags = {
  note_type: string;
  cover_type: string;
  commercial_intensity: string;
  audience: string;
  user_scenario: string;
};

type ManualMaterial = {
  material_id: string;
  note_id: string;
  note_url: string;
  title: string;
  body: string;
  comments: string[];
  tags: ManualMaterialTags;
  tagged: boolean;
  images: ManualMaterialImage[];
  image_count: number;
  comment_count: number;
};

type ManualMaterialTagOptions = {
  note_type: string[];
  cover_type: string[];
  commercial_intensity: string[];
  audience: string[];
  user_scenario: string[];
};

type ManualMaterialsResult = {
  search: string;
  untagged: boolean;
  total: number;
  page: number;
  page_size: number;
  tag_options: ManualMaterialTagOptions;
  items: ManualMaterial[];
};

type TagField = {
  key: keyof ManualMaterialTags;
  label: string;
};

const EMPTY_TAGS: ManualMaterialTags = {
  note_type: "",
  cover_type: "",
  commercial_intensity: "",
  audience: "",
  user_scenario: ""
};

const EMPTY_OPTIONS: ManualMaterialTagOptions = {
  note_type: [],
  cover_type: [],
  commercial_intensity: [],
  audience: [],
  user_scenario: []
};

const TAG_FIELDS: TagField[] = [
  { key: "note_type", label: "内容类型" },
  { key: "audience", label: "对话人群" },
  { key: "user_scenario", label: "用户场景" },
  { key: "cover_type", label: "封面类型" },
  { key: "commercial_intensity", label: "商业强度" }
];

function imageURL(assetID: string): string {
  return `${import.meta.env.BASE_URL}api/manuscript-assets/${assetID}`;
}

function tagsComplete(tags: ManualMaterialTags): boolean {
  return TAG_FIELDS.every((field) => tags[field.key].trim() !== "");
}

function RedMaterialPending({ serviceState }: { serviceState: ServiceState }) {
  const navigate = useNavigate();
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<ManualMaterialsResult>({
    search: "", untagged: true, total: 0, page: 1, page_size: 10,
    tag_options: EMPTY_OPTIONS, items: []
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [selected, setSelected] = useState<ManualMaterial | null>(null);
  const [draftTags, setDraftTags] = useState<ManualMaterialTags>(EMPTY_TAGS);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setSearch(searchInput.trim());
      setPage(1);
    }, 300);
    return () => window.clearTimeout(timer);
  }, [searchInput]);

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ page: String(page), page_size: "10", untagged: "true" });
    if (search) params.set("q", search);
    setLoading(true);
    setError("");
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/manual-materials?${params}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: ManualMaterialsResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "待标注素材读取失败");
        setResult(payload.data);
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "待标注素材读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [page, search, reloadKey]);

  const pageCount = Math.max(1, Math.ceil(result.total / result.page_size));

  const openAnnotator = (item: ManualMaterial) => {
    setSelected(item);
    setDraftTags({
      note_type: item.tags?.note_type ?? "",
      cover_type: item.tags?.cover_type ?? "",
      commercial_intensity: item.tags?.commercial_intensity ?? "",
      audience: item.tags?.audience ?? "",
      user_scenario: item.tags?.user_scenario ?? ""
    });
    setSaveError("");
  };

  const saveTags = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!selected) return;
    if (!tagsComplete(draftTags)) {
      setSaveError("请完整填写全部标签");
      return;
    }
    setSaving(true);
    setSaveError("");
    try {
      const response = await fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/manual-material-tags?material_id=${selected.material_id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(draftTags)
      });
      const payload = await response.json() as { success: boolean; data?: ManualMaterial; error?: string };
      if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "标签保存失败");
      setSelected(null);
      setReloadKey((current) => current + 1);
    } catch (tagError) {
      setSaveError(tagError instanceof Error ? tagError.message : "标签保存失败");
    } finally {
      setSaving(false);
    }
  };

  return <>
    <section className="page-heading red-material-page-heading">
      <div>
        <h1>待标注素材</h1>
        <p>为手动添加的素材补齐内容类型、人群和场景标签</p>
      </div>
      <div className="heading-status">
        <span className={`status-dot ${serviceState}`} />{serviceState === "online" ? "素材服务已连接" : serviceState === "offline" ? "素材服务未连接" : "正在检查连接"}
      </div>
    </section>

    <section className="red-material-toolbar">
      <div className="red-material-toolbar-primary">
        <label className="analysis-search red-material-search"><Search size={16} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索标题、笔记 ID 或正文" aria-label="搜索待标注素材" /></label>
        <span>{loading ? <LoaderCircle size={16} className="spin" /> : null}{search ? `“${search}”` : "全部待标注"} · {result.total} 条</span>
      </div>
    </section>

    {error ? <div className="analysis-error"><AlertCircle size={16} />{error}</div> : null}

    <section className="red-material-pending-list">
      {result.items.map((item) => <article className="red-material-pending-card" key={item.material_id}>
        <div className="red-material-pending-preview">
          {item.images[0] ? <img src={imageURL(item.images[0].asset_id)} alt={`${item.title} 封面`} /> : <div>无图</div>}
        </div>
        <div className="red-material-pending-copy">
          <h2>{item.title}</h2>
          <p>{item.body}</p>
          <div className="red-material-pending-meta">
            <code>{item.note_id || "未填笔记 ID"}</code>
            <span>{item.image_count} 张图 · {item.comment_count} 条评论</span>
          </div>
        </div>
        <div className="red-material-pending-actions">
          {item.note_url ? <a href={item.note_url} target="_blank" rel="noreferrer" title="打开笔记" aria-label={`打开笔记 ${item.note_id || item.title}`}><ExternalLink size={15} /></a> : null}
          <button type="button" onClick={() => navigate(`/red-materials/new?material_id=${item.material_id}`)} title="编辑素材" aria-label={`编辑素材 ${item.title}`}><Pencil size={15} /></button>
          <button className="primary-button" type="button" onClick={() => openAnnotator(item)}><Tags size={15} />标注</button>
        </div>
      </article>)}
      {!loading && result.items.length === 0 ? <div className="red-material-recent-empty">没有待标注的素材</div> : null}
      <footer className="analysis-pagination"><span>第 {result.page}/{pageCount} 页</span><div>
        <button className="icon-button" title="上一页" aria-label="上一页" disabled={page <= 1 || loading} onClick={() => setPage((current) => Math.max(1, current - 1))}><ChevronLeft size={17} /></button>
        <button className="icon-button" title="下一页" aria-label="下一页" disabled={page >= pageCount || loading} onClick={() => setPage((current) => current + 1)}><ChevronRight size={17} /></button>
      </div></footer>
    </section>

    {selected ? <div className="note-content-overlay" onMouseDown={(event) => {
      if (event.target === event.currentTarget && !saving) setSelected(null);
    }}>
      <section className="note-content-dialog red-material-tag-dialog" role="dialog" aria-modal="true" aria-labelledby="pending-material-title">
        <header>
          <div>
            <h2 id="pending-material-title">标注素材</h2>
            <span>{selected.note_id || selected.material_id}</span>
          </div>
          <button className="icon-button" type="button" title="关闭" aria-label="关闭标注" disabled={saving} onClick={() => setSelected(null)}>×</button>
        </header>
        <form onSubmit={saveTags}>
          <div className="red-material-tag-body">
            <strong>{selected.title}</strong>
            <p>{selected.body}</p>
            {TAG_FIELDS.map((field) => <label className="red-material-field" key={field.key}>
              <span>{field.label}</span>
              <input
                list={`pending-tag-${field.key}`}
                value={draftTags[field.key]}
                onChange={(event) => setDraftTags((current) => ({ ...current, [field.key]: event.target.value }))}
                placeholder={`填写${field.label}`}
                aria-label={field.label}
              />
              <datalist id={`pending-tag-${field.key}`}>
                {result.tag_options[field.key].map((option) => <option value={option} key={option} />)}
              </datalist>
            </label>)}
            {saveError ? <div className="analysis-error" role="alert"><AlertCircle size={15} />{saveError}</div> : null}
          </div>
          <footer>
            <button className="outline-button" type="button" disabled={saving} onClick={() => setSelected(null)}>取消</button>
            <button className="primary-button" type="submit" disabled={saving || !tagsComplete(draftTags)}>
              {saving ? <LoaderCircle size={15} className="spin" /> : <CheckCircle2 size={15} />}
              {saving ? "保存中" : "保存标签"}
            </button>
          </footer>
        </form>
      </section>
    </div> : null}
  </>;
}

export default RedMaterialPending;
