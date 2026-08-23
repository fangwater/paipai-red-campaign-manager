import { useEffect, useState } from "react";
import { Download, FileSpreadsheet, FolderOpen, LoaderCircle } from "lucide-react";
import { useLocation } from "react-router-dom";

type Report = { report_date: string; file_name: string; note_count: number };

function ProviderFiles() {
  const location = useLocation();
  const providerCode = location.pathname.split("/").filter(Boolean).at(-1) ?? "";
  const [providerName, setProviderName] = useState("");
  const [reports, setReports] = useState<Report[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    fetch(`${import.meta.env.BASE_URL}api/downloads/maituo-provider/${providerCode}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: { provider_name: string; reports: Report[] }; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "文件目录不存在");
        setProviderName(payload.data.provider_name);
        setReports(payload.data.reports ?? []);
      })
      .catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(reason instanceof Error ? reason.message : "文件目录不存在");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [providerCode]);

  return <main className="public-file-page">
    <header className="public-file-header"><div className="brand-mark">P</div><div><strong>PaiPai RED</strong><span>Maituo 服务商日报</span></div></header>
    <section className="public-file-content">
      <div className="public-file-title"><div><span>服务商日报目录</span><h1>{providerName || "历史报表"}</h1></div><strong>{reports.length} 个文件</strong></div>
      {loading ? <div className="public-file-state"><LoaderCircle size={22} className="spin" /><span>正在读取目录</span></div>
        : error ? <div className="public-file-state error"><FolderOpen size={24} /><span>{error}</span></div>
          : reports.length === 0 ? <div className="public-file-state"><FolderOpen size={24} /><span>暂无匹配日报</span></div>
            : <div className="public-file-table-wrap"><table className="public-file-table"><thead><tr><th>日期</th><th>文件</th><th>笔记数</th><th /></tr></thead><tbody>
              {reports.map((report) => <tr key={report.report_date}>
                <td><strong>{report.report_date}</strong></td>
                <td><span className="public-file-name"><FileSpreadsheet size={17} />{report.file_name}</span></td>
                <td>{report.note_count}</td>
                <td><a className="public-download-button" href={`${import.meta.env.BASE_URL}api/downloads/maituo-provider/${providerCode}/${report.report_date}.xlsx`} aria-label={`下载 ${report.report_date} 报表`}><Download size={16} /><span>下载</span></a></td>
              </tr>)}
            </tbody></table></div>}
    </section>
  </main>;
}

export default ProviderFiles;
