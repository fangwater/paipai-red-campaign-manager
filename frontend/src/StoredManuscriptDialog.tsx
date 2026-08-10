import { useEffect, useState } from "react";
import { AlertCircle, CheckCircle2, Image as ImageIcon, LoaderCircle, Pencil, Tags as TagsIcon, X, ZoomIn } from "lucide-react";

type ManuscriptBlock = {
  type: "paragraph" | "heading" | "bullet" | "ordered" | "quote" | "code" | "todo" | "equation" | "divider" | "image";
  text?: string;
  level?: number;
  asset_id?: string;
  width?: number;
  height?: number;
  caption?: string;
};

type NoteTags = {
  note_type: string[];
  cover_type: string[];
  commercial_intensity: string[];
  audience: string[];
  user_scenario: string[];
  progress: string[];
  complete: boolean;
  missing_fields: string[];
};

type NoteContentResult = {
  note_id: string;
  found: boolean;
  note_content: string;
  blocks?: ManuscriptBlock[];
  providers?: string[];
  tags?: NoteTags;
  source?: "manual" | "manuscript" | string;
};

type ContentVariant = "manuscript" | "reference";
type ZoomedImage = { src: string; caption: string };
type NoteTagField = "note_type" | "cover_type" | "commercial_intensity" | "audience" | "user_scenario" | "progress";

const NOTE_TAG_FIELDS: Array<{ key: NoteTagField; label: string }> = [
  { key: "note_type", label: "内容类型" },
  { key: "audience", label: "对话人群" },
  { key: "user_scenario", label: "用户场景" },
  { key: "cover_type", label: "封面类型" },
  { key: "commercial_intensity", label: "商业强度" },
  { key: "progress", label: "进度" }
];

function manuscriptAssetURL(assetID: string | undefined): string | null {
  if (!assetID || !/^[0-9a-f]{64}$/.test(assetID)) return null;
  return `${import.meta.env.BASE_URL}api/manuscript-assets/${assetID}`;
}

function ManuscriptImage({ block, onZoom }: { block: ManuscriptBlock; onZoom: (image: ZoomedImage) => void }) {
  const [failed, setFailed] = useState(false);
  const src = manuscriptAssetURL(block.asset_id);
  const caption = block.caption?.trim() ?? "";
  if (!src || failed) {
    return <div className="manuscript-image-error" role="status"><ImageIcon size={18} /><span>图片加载失败</span></div>;
  }
  return <figure className="manuscript-image">
    <button type="button" aria-label={caption ? `查看大图：${caption}` : "查看稿件大图"} onClick={() => onZoom({ src, caption })}>
      <img
        src={src}
        alt={caption || "稿件图片"}
        loading="lazy"
        decoding="async"
        width={block.width && block.width > 0 ? block.width : undefined}
        height={block.height && block.height > 0 ? block.height : undefined}
        onError={() => setFailed(true)}
      />
      <span className="manuscript-image-zoom" aria-hidden="true"><ZoomIn size={15} /></span>
    </button>
    {caption ? <figcaption>{caption}</figcaption> : null}
  </figure>;
}

function ManuscriptContent({ blocks, fallback, onZoom }: {
  blocks: ManuscriptBlock[];
  fallback: string;
  onZoom: (image: ZoomedImage) => void;
}) {
  if (blocks.length === 0) return <pre>{fallback}</pre>;
  return <div className="manuscript-blocks">{blocks.map((block, index) => {
    const key = `${index}-${block.type}-${block.asset_id ?? ""}`;
    switch (block.type) {
    case "image":
      return <ManuscriptImage key={key} block={block} onZoom={onZoom} />;
    case "heading":
      return <h3 className={`manuscript-heading level-${block.level ?? 0}`} key={key}>{block.text}</h3>;
    case "bullet":
    case "ordered":
    case "todo":
      return <div className={`manuscript-list-item ${block.type}`} key={key}><span aria-hidden="true" /><p>{block.text}</p></div>;
    case "quote":
      return <blockquote key={key}>{block.text}</blockquote>;
    case "code":
      return <pre className="manuscript-code" key={key}>{block.text}</pre>;
    case "divider":
      return <hr key={key} />;
    default:
      return <p key={key}>{block.text}</p>;
    }
  })}</div>;
}

function NoteTagsPanel({ tags }: { tags: NoteTags | undefined }) {
  const complete = tags?.complete === true;
  return <section className="note-content-tags" aria-label="稿件标签">
    <div className="note-content-tags-heading">
      <div><TagsIcon size={15} /><span>标签信息</span></div>
      <span className={`note-tags-completeness ${complete ? "complete" : "incomplete"}`} role="status">
        {complete ? <CheckCircle2 size={13} /> : <AlertCircle size={13} />}
        {complete ? "标签完整" : "标签待补充"}
      </span>
    </div>
    <dl>{NOTE_TAG_FIELDS.map((field) => {
      const values = tags?.[field.key] ?? [];
      return <div className={values.length === 0 ? "missing" : ""} key={field.key}>
        <dt>{field.label}</dt>
        <dd>{values.length > 0
          ? values.map((value) => <span className="note-tag-value" key={`${field.key}-${value}`}>{value}</span>)
          : <span className="note-tag-value missing">待补充</span>}
        </dd>
      </div>;
    })}</dl>
  </section>;
}

function StoredManuscriptDialog({
  noteID,
  manuscriptTitle = "",
  onClose,
  variant = "manuscript",
  onEdit
}: {
  noteID: string;
  manuscriptTitle?: string;
  onClose: () => void;
  variant?: ContentVariant;
  onEdit?: () => void;
}) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [content, setContent] = useState<NoteContentResult | null>(null);
  const [zoomedImage, setZoomedImage] = useState<ZoomedImage | null>(null);
  const isReference = variant === "reference";

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ note_id: noteID });
    const endpoint = isReference ? "reference-material-content" : "note-content";
    const readError = isReference ? "参考内容读取失败" : "稿件内容读取失败";
    setLoading(true);
    setError("");
    setContent(null);
    setZoomedImage(null);
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/${endpoint}?${params}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: NoteContentResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || readError);
        setContent(payload.data);
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : readError);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [isReference, noteID]);

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (zoomedImage) {
        setZoomedImage(null);
        return;
      }
      onClose();
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [onClose, zoomedImage]);

  return <>
    <div className="note-content-overlay" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="note-content-dialog stored-manuscript-dialog" role="dialog" aria-modal="true" aria-labelledby="stored-manuscript-title">
        <header>
          <div><h2 id="stored-manuscript-title">{isReference ? "参考内容" : "对应稿件"}</h2><span>
            {isReference ? (content?.note_id || noteID) : (manuscriptTitle.trim() || content?.note_id || noteID)}
          </span></div>
          <div className="note-content-header-actions">
            {isReference && onEdit ? <button className="icon-button" title="编辑参考内容" aria-label="编辑参考内容" onClick={onEdit}><Pencil size={16} /></button> : null}
            <button className="icon-button" title="关闭" aria-label={isReference ? "关闭参考内容" : "关闭对应稿件"} onClick={onClose}><X size={18} /></button>
          </div>
        </header>
        <div className="note-content-body">
          {loading ? <div className="note-content-state"><LoaderCircle size={18} className="spin" />{isReference ? "正在读取参考内容" : "正在读取稿件内容"}</div>
            : error ? <div className="note-content-state error"><AlertCircle size={18} />{error}</div>
              : content?.found ? <>
                {isReference
                  ? <div className="note-content-source"><span>内容来源</span><strong>{content.source === "manual" ? "人工录入" : "稿件库"}</strong></div>
                  : (content.providers ?? []).length > 0 ? <div className="note-content-source"><span>来源机构</span><strong>{content.providers?.join("、")}</strong></div> : null}
                {!isReference ? <NoteTagsPanel tags={content.tags} /> : null}
                <ManuscriptContent blocks={content.blocks ?? []} fallback={content.note_content} onZoom={setZoomedImage} />
              </> : <div className="note-content-state">{isReference ? "尚未录入这篇参考素材的内容" : "当前稿件库未收录这篇笔记的内容"}</div>}
        </div>
      </section>
    </div>
    {zoomedImage ? <div className="manuscript-lightbox" role="dialog" aria-modal="true" aria-label="稿件大图" onMouseDown={(event) => {
      if (event.target === event.currentTarget) setZoomedImage(null);
    }}>
      <button className="icon-button" title="关闭" aria-label="关闭稿件大图" onClick={() => setZoomedImage(null)}><X size={19} /></button>
      <img src={zoomedImage.src} alt={zoomedImage.caption || "稿件大图"} />
      {zoomedImage.caption ? <p>{zoomedImage.caption}</p> : null}
    </div> : null}
  </>;
}

export default StoredManuscriptDialog;
