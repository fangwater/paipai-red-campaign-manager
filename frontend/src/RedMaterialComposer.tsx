import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { AlertCircle, CheckCircle2, ImagePlus, LoaderCircle, Pencil, Plus, Save, Trash2, X } from "lucide-react";
import { useNavigate, useSearchParams } from "react-router-dom";

import "./red-materials.css";

type ServiceState = "checking" | "online" | "offline";

type ManualMaterialImage = {
  asset_id: string;
  width: number;
  height: number;
};

type ManualMaterial = {
  material_id: string;
  note_id: string;
  note_url: string;
  title: string;
  body: string;
  comments: string[];
  tagged?: boolean;
  images: ManualMaterialImage[];
  image_count: number;
  comment_count: number;
  updated_at?: string;
};

type ManualMaterialsResult = {
  total: number;
  items: ManualMaterial[];
};

type LocalImage = {
  key: string;
  file?: File;
  assetID?: string;
  previewURL: string;
  width: number;
  height: number;
};

const MAX_TITLE_CHARACTERS = 200;
const MAX_BODY_CHARACTERS = 20000;
const MAX_COMMENT_CHARACTERS = 500;
const MAX_COMMENTS = 20;
const MAX_IMAGES = 9;
const MAX_IMAGE_BYTES = 10 * 1024 * 1024;
const ACCEPTED_IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp", "image/gif"];

function clipText(value: string, limit: number): string {
  const characters = Array.from(value);
  return characters.length > limit ? characters.slice(0, limit).join("") : value;
}

function imageURL(assetID: string): string {
  return `${import.meta.env.BASE_URL}api/manuscript-assets/${assetID}`;
}

function RedMaterialComposer({ serviceState }: { serviceState: ServiceState }) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const materialID = (searchParams.get("material_id") || "").trim().toLowerCase();
  const editing = /^[0-9a-f]{32}$/.test(materialID);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const imagesRef = useRef<LocalImage[]>([]);
  const [noteID, setNoteID] = useState("");
  const [noteURL, setNoteURL] = useState("");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [comments, setComments] = useState<string[]>([""]);
  const [images, setImages] = useState<LocalImage[]>([]);
  const [loading, setLoading] = useState(editing);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [savedID, setSavedID] = useState("");
  const [recent, setRecent] = useState<ManualMaterial[]>([]);
  const [recentLoading, setRecentLoading] = useState(true);

  useEffect(() => {
    if (!editing) return;
    const controller = new AbortController();
    setLoading(true);
    setError("");
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/manual-material?material_id=${materialID}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: ManualMaterial; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "素材读取失败");
        setNoteID(payload.data.note_id ?? "");
        setNoteURL(payload.data.note_url ?? "");
        setTitle(payload.data.title);
        setBody(payload.data.body);
        setComments(payload.data.comments.length > 0 ? payload.data.comments : [""]);
        setImages(payload.data.images.map((image) => ({
          key: image.asset_id,
          assetID: image.asset_id,
          previewURL: imageURL(image.asset_id),
          width: image.width,
          height: image.height
        })));
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "素材读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [editing, materialID]);

  useEffect(() => {
    const controller = new AbortController();
    setRecentLoading(true);
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/manual-materials?page=1&page_size=8`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: ManualMaterialsResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "已添加素材读取失败");
        setRecent(payload.data.items ?? []);
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
      })
      .finally(() => {
        if (!controller.signal.aborted) setRecentLoading(false);
      });
    return () => controller.abort();
  }, [savedID]);

  imagesRef.current = images;
  useEffect(() => () => {
    for (const image of imagesRef.current) {
      if (image.file) URL.revokeObjectURL(image.previewURL);
    }
  }, []);

  const filledComments = useMemo(
    () => comments.map((comment) => comment.trim()).filter(Boolean),
    [comments]
  );
  const titleCount = Array.from(title).length;
  const bodyCount = Array.from(body).length;
  const canSave = noteID.trim() !== "" && title.trim() !== "" && body.trim() !== "" && images.length > 0 && !loading && !saving;

  const applyNoteID = (value: string) => {
    const next = clipText(value.trim().toLowerCase(), 24);
    setNoteID(next);
    setSavedID("");
    if (!noteURL.trim() && /^[0-9a-f]{24}$/.test(next)) {
      setNoteURL(`https://www.xiaohongshu.com/explore/${next}`);
    }
  };

  const applyNoteURL = (value: string) => {
    setNoteURL(clipText(value, 500));
    setSavedID("");
    const match = value.match(/xiaohongshu\.com\/(?:explore|discovery\/item)\/([0-9a-fA-F]{24})/i);
    if (match && !noteID.trim()) setNoteID(match[1].toLowerCase());
  };

  const addImages = (files: File[]) => {
    const remaining = MAX_IMAGES - images.length;
    if (remaining <= 0) {
      setError("图片不能超过 9 张");
      return;
    }
    const next: LocalImage[] = [];
    for (const file of files.slice(0, remaining)) {
      if (!ACCEPTED_IMAGE_TYPES.includes(file.type)) {
        setError("仅支持 JPEG、PNG、WebP 或 GIF 图片");
        return;
      }
      if (file.size > MAX_IMAGE_BYTES) {
        setError("单张图片不能超过 10 MB");
        return;
      }
      next.push({
        key: `${file.name}-${file.size}-${file.lastModified}-${crypto.randomUUID()}`,
        file,
        previewURL: URL.createObjectURL(file),
        width: 0,
        height: 0
      });
    }
    setError("");
    setSavedID("");
    setImages((current) => [...current, ...next]);
  };

  const removeImage = (key: string) => {
    setImages((current) => {
      const target = current.find((image) => image.key === key);
      if (target?.file) URL.revokeObjectURL(target.previewURL);
      return current.filter((image) => image.key !== key);
    });
    setSavedID("");
  };

  const updateComment = (index: number, value: string) => {
    setComments((current) => current.map((comment, commentIndex) => commentIndex === index ? clipText(value, MAX_COMMENT_CHARACTERS) : comment));
    setSavedID("");
  };

  const addComment = () => {
    if (comments.length >= MAX_COMMENTS) return;
    setComments((current) => [...current, ""]);
    setSavedID("");
  };

  const removeComment = (index: number) => {
    setComments((current) => current.length === 1 ? [""] : current.filter((_, commentIndex) => commentIndex !== index));
    setSavedID("");
  };

  const saveMaterial = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSave) {
      if (!noteID.trim()) setError("笔记 ID 不能为空");
      else if (images.length === 0) setError("至少上传一张图片");
      else if (!title.trim()) setError("标题不能为空");
      else if (!body.trim()) setError("正文不能为空");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const form = new FormData();
      form.set("note_id", noteID.trim());
      form.set("note_url", noteURL.trim());
      form.set("title", title.trim());
      form.set("body", body.trim());
      form.set("comments", JSON.stringify(filledComments));
      form.set("existing_image_ids", JSON.stringify(images.flatMap((image) => image.assetID ? [image.assetID] : [])));
      for (const image of images) {
        if (image.file) form.append("images", image.file, image.file.name);
      }
      const endpoint = editing
        ? `${import.meta.env.BASE_URL}api/analytics/maituo/manual-material?material_id=${materialID}`
        : `${import.meta.env.BASE_URL}api/analytics/maituo/manual-materials`;
      const response = await fetch(endpoint, { method: editing ? "PUT" : "POST", body: form });
      const payload = await response.json() as { success: boolean; data?: ManualMaterial; error?: string };
      if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "素材保存失败");
      setSavedID(payload.data.material_id);
      if (!editing) {
        setNoteID("");
        setNoteURL("");
        setTitle("");
        setBody("");
        setComments([""]);
        setImages((current) => {
          for (const image of current) {
            if (image.file) URL.revokeObjectURL(image.previewURL);
          }
          return [];
        });
      }
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "素材保存失败");
    } finally {
      setSaving(false);
    }
  };

  return <>
    <section className="page-heading red-material-page-heading">
      <div>
        <h1>{editing ? "编辑素材" : "添加素材"}</h1>
        <p>配置笔记 ID、链接、图片、评论、标题和正文</p>
      </div>
      <div className="heading-status"><span className={`status-dot ${serviceState}`} />{serviceState === "online" ? "素材服务已连接" : serviceState === "offline" ? "素材服务未连接" : "正在检查连接"}</div>
    </section>

    <form className="red-material-composer" onSubmit={saveMaterial}>
      {loading ? <div className="note-content-state"><LoaderCircle size={18} className="spin" />正在读取素材</div> : <>
        <section className="red-material-composer-card">
          <header>
            <div><h2>图片</h2><p>最多 9 张，支持 JPEG / PNG / WebP / GIF</p></div>
            <span>{images.length} / {MAX_IMAGES}</span>
          </header>
          <div className="red-material-image-grid">
            {images.map((image, index) => <figure className="red-material-image-tile" key={image.key}>
              <img src={image.previewURL} alt={`素材图片 ${index + 1}`} />
              <button type="button" title="移除图片" aria-label={`移除图片 ${index + 1}`} onClick={() => removeImage(image.key)}><Trash2 size={14} /></button>
            </figure>)}
            {images.length < MAX_IMAGES ? <button className="red-material-image-add" type="button" onClick={() => fileInputRef.current?.click()}>
              <ImagePlus size={20} /><span>添加图片</span>
            </button> : null}
          </div>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            multiple
            hidden
            onChange={(event) => {
              addImages(Array.from(event.target.files ?? []));
              event.target.value = "";
            }}
          />
        </section>

        <section className="red-material-composer-card">
          <header>
            <div><h2>笔记信息</h2><p>填写小红书笔记 ID，链接可自动补全</p></div>
          </header>
          <div className="red-material-field-grid">
            <label className="red-material-field">
              <span>笔记 ID</span>
              <input
                value={noteID}
                onChange={(event) => applyNoteID(event.target.value)}
                placeholder="24 位小红书笔记 ID"
                aria-label="笔记 ID"
              />
            </label>
            <label className="red-material-field">
              <span>笔记链接</span>
              <input
                value={noteURL}
                onChange={(event) => applyNoteURL(event.target.value)}
                placeholder="https://www.xiaohongshu.com/explore/..."
                aria-label="笔记链接"
              />
            </label>
          </div>
        </section>

        <section className="red-material-composer-card">
          <header>
            <div><h2>标题与正文</h2><p>按小红书笔记结构填写</p></div>
          </header>
          <label className="red-material-field">
            <span>标题</span>
            <input
              value={title}
              onChange={(event) => { setTitle(clipText(event.target.value, MAX_TITLE_CHARACTERS)); setSavedID(""); }}
              placeholder="输入笔记标题"
              aria-label="素材标题"
            />
            <small>{titleCount.toLocaleString("zh-CN")} / {MAX_TITLE_CHARACTERS.toLocaleString("zh-CN")}</small>
          </label>
          <label className="red-material-field">
            <span>正文</span>
            <textarea
              value={body}
              onChange={(event) => { setBody(clipText(event.target.value, MAX_BODY_CHARACTERS)); setSavedID(""); }}
              placeholder="输入笔记正文"
              aria-label="素材正文"
            />
            <small>{bodyCount.toLocaleString("zh-CN")} / {MAX_BODY_CHARACTERS.toLocaleString("zh-CN")}</small>
          </label>
        </section>

        <section className="red-material-composer-card">
          <header>
            <div><h2>评论</h2><p>可选，用于预置笔记下的引导评论</p></div>
            <span>{filledComments.length} / {MAX_COMMENTS}</span>
          </header>
          <div className="red-material-comment-list">
            {comments.map((comment, index) => <div className="red-material-comment-row" key={index}>
              <textarea
                value={comment}
                onChange={(event) => updateComment(index, event.target.value)}
                placeholder={`评论 ${index + 1}`}
                aria-label={`素材评论 ${index + 1}`}
              />
              <button type="button" title="移除评论" aria-label={`移除评论 ${index + 1}`} onClick={() => removeComment(index)}><X size={15} /></button>
            </div>)}
          </div>
          <button className="red-material-add-comment" type="button" disabled={comments.length >= MAX_COMMENTS} onClick={addComment}>
            <Plus size={15} />添加评论
          </button>
        </section>
      </>}

      {error ? <div className="analysis-error" role="alert"><AlertCircle size={16} />{error}</div> : null}
      {savedID ? <div className="red-material-save-success" role="status"><CheckCircle2 size={16} />素材已保存，可继续添加或前往检索查看</div> : null}

      <footer className="red-material-composer-actions">
        <button className="outline-button" type="button" disabled={saving} onClick={() => navigate("/red-materials/pending")}>前往待标注素材</button>
        <button className="primary-button" type="submit" disabled={!canSave}>
          {saving ? <LoaderCircle size={15} className="spin" /> : <Save size={15} />}
          {saving ? "保存中" : editing ? "保存修改" : "保存素材"}
        </button>
      </footer>
    </form>

    <section className="red-material-recent">
      <header>
        <div><h2>已添加素材</h2><p>最近保存的结构化素材</p></div>
        {recentLoading ? <LoaderCircle size={16} className="spin" /> : null}
      </header>
      {recent.length === 0 && !recentLoading ? <div className="red-material-recent-empty">还没有手动添加的素材</div> : <ul>
        {recent.map((item) => <li key={item.material_id}>
          <div>
            <strong>{item.title}</strong>
            <span>{item.note_id || "未填笔记 ID"} · {item.image_count} 张图 · {item.comment_count} 条评论{item.tagged ? "" : " · 待标注"}</span>
          </div>
          <button type="button" title="编辑素材" aria-label={`编辑素材 ${item.title}`} onClick={() => navigate(`/red-materials/new?material_id=${item.material_id}`)}>
            <Pencil size={14} />编辑
          </button>
        </li>)}
      </ul>}
    </section>
  </>;
}

export default RedMaterialComposer;
