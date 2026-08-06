import { useRef, useState, type DragEvent } from "react";
import {
  AlertCircle, CheckCircle2, FileSpreadsheet, LoaderCircle, RotateCcw, UploadCloud
} from "lucide-react";

type ImportResult = {
  run_id: number;
  file_name: string;
  sheet_name: string;
  header_row: number;
  fetched: number;
  inserted: number;
  updated: number;
  unchanged: number;
  deleted: number;
  completed_at: string;
};

const MAX_FILE_SIZE = 50 * 1024 * 1024;

function fileSize(bytes: number): string {
  return bytes < 1024 * 1024
    ? (bytes / 1024).toFixed(1) + " KB"
    : (bytes / 1024 / 1024).toFixed(1) + " MB";
}

function DandelionUpdate() {
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState("");
  const [syncing, setSyncing] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);

  const chooseFile = (candidate?: File) => {
    if (!candidate || syncing) return;
    setError("");
    setResult(null);
    if (candidate.name.split(".").pop()?.toLowerCase() !== "xlsx") {
      setFile(null);
      setError("仅支持 .xlsx 文件");
      return;
    }
    if (candidate.size > MAX_FILE_SIZE) {
      setFile(null);
      setError("文件不能超过 50 MB");
      return;
    }
    setFile(candidate);
  };

  const reset = () => {
    setFile(null);
    setError("");
    setResult(null);
    setDragging(false);
    if (inputRef.current) inputRef.current.value = "";
  };

  const updateData = async () => {
    if (!file || syncing) return;
    setSyncing(true);
    setError("");
    setResult(null);
    try {
      const formData = new FormData();
      formData.append("file", file);
      const response = await fetch(import.meta.env.BASE_URL + "api/imports/dandelion-excel", {
        method: "POST",
        body: formData
      });
      const payload = await response.json() as { success: boolean; data?: ImportResult; error?: string };
      if (!response.ok || !payload.success || !payload.data) {
        throw new Error(payload.error || "蒲公英数据更新失败");
      }
      setResult(payload.data);
    } catch (updateError) {
      setError(updateError instanceof Error ? updateError.message : "蒲公英数据更新失败");
    } finally {
      setSyncing(false);
    }
  };

  const onDrop = (event: DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setDragging(false);
    chooseFile(event.dataTransfer.files[0]);
  };

  return <>
    <section className="page-heading dandelion-update-heading">
      <div><h1>蒲公英数据更新</h1><p>数据中心 · Excel 增量更新</p></div>
    </section>

    <section className="dandelion-update-panel">
      <header>
        <div><FileSpreadsheet size={19} /><div><h2>数据文件</h2><p>XLSX · 最大 50 MB</p></div></div>
        {file ? <button className="outline-button dandelion-reset" disabled={syncing} onClick={reset}>
          <RotateCcw size={14} />重新选择
        </button> : null}
      </header>

      {!file ? <>
        <div className={"dandelion-update-dropzone " + (dragging ? "dragging" : "")}
          onDragEnter={(event) => { event.preventDefault(); setDragging(true); }}
          onDragOver={(event) => event.preventDefault()}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}>
          <UploadCloud size={28} />
          <div><strong>拖入蒲公英 Excel</strong><span>或从电脑选择文件</span></div>
          <button className="primary-button" onClick={() => inputRef.current?.click()}>
            <UploadCloud size={15} />选择文件
          </button>
        </div>
        <input ref={inputRef} type="file" accept=".xlsx" hidden
          onChange={(event) => { chooseFile(event.target.files?.[0]); event.target.value = ""; }} />
      </> : <>
        <div className="dandelion-update-file">
          <span className="dandelion-update-file-icon"><FileSpreadsheet size={22} /></span>
          <div><strong>{file.name}</strong><span>{fileSize(file.size)}</span></div>
          <CheckCircle2 size={19} />
        </div>
        <div className="dandelion-update-action">
          <span>增量更新蒲公英数据</span>
          <button className="primary-button" disabled={syncing || result !== null} onClick={() => void updateData()}>
            {syncing ? <LoaderCircle size={15} className="spin" /> : <UploadCloud size={15} />}
            {syncing ? "正在更新" : result ? "更新完成" : "更新数据"}
          </button>
        </div>
      </>}

      {error ? <div className="dandelion-update-error"><AlertCircle size={16} /><span>{error}</span></div> : null}
    </section>

    {result ? <section className="dandelion-update-result">
      <header><CheckCircle2 size={19} /><div><h2>更新完成</h2><p>{result.sheet_name} · {result.file_name}</p></div></header>
      <div>
        <span><small>读取</small><strong>{result.fetched.toLocaleString()}</strong></span>
        <span><small>新增</small><strong>{result.inserted.toLocaleString()}</strong></span>
        <span><small>更新</small><strong>{result.updated.toLocaleString()}</strong></span>
        <span><small>未变化</small><strong>{result.unchanged.toLocaleString()}</strong></span>
      </div>
    </section> : null}
  </>;
}

export default DandelionUpdate;
