import { useCallback, useEffect, useRef, useState, type DragEvent } from "react";
import {
  AlertCircle, Check, CheckCircle2, Clock3, FileSpreadsheet, LoaderCircle, RotateCcw, UploadCloud
} from "lucide-react";
import { buildHistoryCalendar, formatSavedTime, weekdayLabel } from "./SavedReportHistory";

type ImportResult = {
  run_id: number;
  file_name: string;
  sheet_name: string;
  header_row: number;
  report_date: string;
  fetched: number;
  inserted: number;
  updated: number;
  unchanged: number;
  deleted: number;
  completed_at: string;
};

type SavedImport = {
  run_id: number;
  file_name: string;
  file_sha256: string;
  report_date: string;
  sheet_name: string;
  fetched: number;
  inserted: number;
  updated: number;
  unchanged: number;
  completed_at: string;
};

const MAX_FILE_SIZE = 50 * 1024 * 1024;

function fileSize(bytes: number): string {
  return bytes < 1024 * 1024
    ? (bytes / 1024).toFixed(1) + " KB"
    : (bytes / 1024 / 1024).toFixed(1) + " MB";
}

function DandelionUploadHistory({ reports, loading, error }: {
  reports: SavedImport[];
  loading: boolean;
  error: string;
}) {
  const entries = buildHistoryCalendar(reports);
  const nonWorkingCount = entries.filter((entry) => entry.kind === "weekend").length;
  const missingCount = entries.filter((entry) => entry.kind === "missing").length;

  return <section className="history-section saved-section dandelion-history">
    <div className="history-header">
      <div><h2>历史上传</h2><p>按数据更新日期从新到旧</p></div>
      <span className="saved-total">{reports.length} 份文件{nonWorkingCount > 0 ? ` · ${nonWorkingCount} 个非工作日` : ""}{missingCount > 0 ? ` · ${missingCount} 个日期缺失` : ""}</span>
    </div>
    {loading ? <div className="history-empty"><LoaderCircle size={20} className="spin" /><span>正在读取</span></div>
      : error ? <div className="history-empty directory-error"><AlertCircle size={20} /><span>{error}</span></div>
        : reports.length === 0 ? <div className="history-empty"><FileSpreadsheet size={22} /><span>暂无历史上传</span></div>
          : <div className="saved-table-wrap">
            <table className="saved-table dandelion-history-table">
              <thead><tr><th>数据日期</th><th>文件</th><th>工作表</th><th>数据行</th><th>数据变更</th><th>上传时间</th><th>状态</th></tr></thead>
              <tbody>{entries.map((entry) => {
                if (entry.kind === "report") {
                  return <tr key={entry.date}>
                    <td><span className="report-date-value"><strong>{entry.date}</strong><small>{weekdayLabel(entry.date)}</small></span></td>
                    <td title={entry.report.file_name}>{entry.report.file_name}</td>
                    <td title={entry.report.sheet_name}>{entry.report.sheet_name}</td>
                    <td>{entry.report.fetched.toLocaleString()}</td>
                    <td><span className="dandelion-change-count"><small>新增 {entry.report.inserted.toLocaleString()}</small><small>更新 {entry.report.updated.toLocaleString()}</small></span></td>
                    <td>{formatSavedTime(entry.report.completed_at)}</td>
                    <td><span className="saved-badge"><Check size={13} />已上传</span></td>
                  </tr>;
                }
                const nonWorking = entry.kind === "weekend";
                return <tr className={`calendar-row ${entry.kind}`} key={entry.date}>
                  <td><span className="report-date-value"><strong>{entry.date}</strong><small>{weekdayLabel(entry.date)}</small></span></td>
                  <td><span className="calendar-note">{nonWorking ? "非工作日，无需上传" : "未收到蒲公英文件"}</span></td>
                  <td>-</td><td>-</td><td>-</td><td>-</td>
                  <td><span className={`saved-badge ${entry.kind}`}>{nonWorking ? <Clock3 size={13} /> : <AlertCircle size={13} />}{nonWorking ? "无需上传" : "缺少文件"}</span></td>
                </tr>;
              })}</tbody>
            </table>
          </div>}
  </section>;
}

function DandelionUpdate() {
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState("");
  const [syncing, setSyncing] = useState(false);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [savedImports, setSavedImports] = useState<SavedImport[]>([]);
  const [historyLoading, setHistoryLoading] = useState(true);
  const [historyError, setHistoryError] = useState("");

  const loadSavedImports = useCallback(async () => {
    setHistoryLoading(true);
    setHistoryError("");
    try {
      const response = await fetch(import.meta.env.BASE_URL + "api/imports/dandelion-excel");
      const payload = await response.json() as { success: boolean; data?: SavedImport[]; error?: string };
      if (!response.ok || !payload.success) throw new Error(payload.error || "无法读取历史上传");
      const items = Array.isArray(payload.data) ? payload.data : [];
      setSavedImports([...items].sort((left, right) => right.report_date.localeCompare(left.report_date)));
    } catch (historyLoadError) {
      setHistoryError(historyLoadError instanceof Error ? historyLoadError.message : "无法读取历史上传");
    } finally {
      setHistoryLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadSavedImports();
  }, [loadSavedImports]);

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
      void loadSavedImports();
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
      <header><CheckCircle2 size={19} /><div><h2>更新完成</h2><p>{[result.report_date, result.sheet_name, result.file_name].filter(Boolean).join(" · ")}</p></div></header>
      <div>
        <span><small>读取</small><strong>{result.fetched.toLocaleString()}</strong></span>
        <span><small>新增</small><strong>{result.inserted.toLocaleString()}</strong></span>
        <span><small>更新</small><strong>{result.updated.toLocaleString()}</strong></span>
        <span><small>未变化</small><strong>{result.unchanged.toLocaleString()}</strong></span>
      </div>
    </section> : null}

    <DandelionUploadHistory reports={savedImports} loading={historyLoading} error={historyError} />
  </>;
}

export default DandelionUpdate;
