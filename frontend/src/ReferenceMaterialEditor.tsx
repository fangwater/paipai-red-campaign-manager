import { FormEvent, useEffect, useState } from "react";
import { AlertCircle, LoaderCircle, Save, X } from "lucide-react";

type ReferenceMaterialContentResult = {
  note_id: string;
  found: boolean;
  note_content: string;
  source?: "manual" | "manuscript" | string;
};

type ReferenceMaterialEditorProps = {
  noteID: string;
  hasContent: boolean;
  onClose: () => void;
  onSaved: (content: ReferenceMaterialContentResult) => void;
};

const MAX_CONTENT_CHARACTERS = 20000;

function ReferenceMaterialEditor({ noteID, hasContent, onClose, onSaved }: ReferenceMaterialEditorProps) {
  const [noteContent, setNoteContent] = useState("");
  const [found, setFound] = useState(hasContent);
  const [source, setSource] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    const params = new URLSearchParams({ note_id: noteID });
    setLoading(true);
    setError("");
    fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/reference-material-content?${params}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: ReferenceMaterialContentResult; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "参考内容读取失败");
        setFound(payload.data.found);
        setSource(payload.data.source ?? "");
        setNoteContent(payload.data.note_content ?? "");
      })
      .catch((fetchError) => {
        if (fetchError instanceof DOMException && fetchError.name === "AbortError") return;
        setError(fetchError instanceof Error ? fetchError.message : "参考内容读取失败");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [noteID]);

  useEffect(() => {
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !saving) onClose();
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [onClose, saving]);

  const characterCount = Array.from(noteContent).length;

  const updateNoteContent = (value: string) => {
    const characters = Array.from(value);
    setNoteContent(characters.length > MAX_CONTENT_CHARACTERS
      ? characters.slice(0, MAX_CONTENT_CHARACTERS).join("")
      : value);
    setError("");
  };

  const saveContent = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedContent = noteContent.trim();
    if (!trimmedContent) {
      setError("素材内容不能为空");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const response = await fetch(`${import.meta.env.BASE_URL}api/analytics/maituo/reference-material-content`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ reference_note_id: noteID, note_content: trimmedContent })
      });
      const payload = await response.json() as { success: boolean; data?: ReferenceMaterialContentResult; error?: string };
      if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "参考内容保存失败");
      onSaved(payload.data);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "参考内容保存失败");
      setSaving(false);
    }
  };

  return <div className="note-content-overlay" onMouseDown={(event) => {
    if (event.target === event.currentTarget && !saving) onClose();
  }}>
    <section className="note-content-dialog reference-material-editor-dialog" role="dialog" aria-modal="true" aria-labelledby="reference-material-editor-title">
      <header>
        <div><h2 id="reference-material-editor-title">{found ? "编辑参考内容" : "录入参考内容"}</h2><span>{noteID}</span></div>
        <button className="icon-button" type="button" title="关闭" aria-label="关闭参考内容编辑" disabled={saving} onClick={onClose}><X size={18} /></button>
      </header>
      <form onSubmit={saveContent}>
        <div className="reference-material-editor-body">
          {loading ? <div className="note-content-state"><LoaderCircle size={18} className="spin" />正在读取参考内容</div> : <>
            <label htmlFor="reference-material-content">素材内容</label>
            <textarea
              id="reference-material-content"
              autoFocus
              value={noteContent}
              onChange={(event) => updateNoteContent(event.target.value)}
              aria-describedby={error ? "reference-material-editor-error" : undefined}
            />
            <div className="reference-material-editor-meta">
              <span>{found ? (source === "manual" ? "人工录入" : "来自稿件库") : "尚未填充"}</span>
              <span>{characterCount.toLocaleString("zh-CN")} / 20,000</span>
            </div>
            {error ? <div id="reference-material-editor-error" className="reference-material-editor-error" role="alert"><AlertCircle size={15} />{error}</div> : null}
          </>}
        </div>
        <footer>
          <button className="reference-material-editor-cancel" type="button" disabled={saving} onClick={onClose}><X size={15} />取消</button>
          <button className="reference-material-editor-save" type="submit" disabled={loading || saving || noteContent.trim() === ""}>
            {saving ? <LoaderCircle size={15} className="spin" /> : <Save size={15} />}{saving ? "保存中" : "保存"}
          </button>
        </footer>
      </form>
    </section>
  </div>;
}

export default ReferenceMaterialEditor;
