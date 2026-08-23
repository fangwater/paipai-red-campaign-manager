import { useEffect, useMemo, useState } from "react";
import { Check, Copy, ExternalLink, FolderOpen, LoaderCircle } from "lucide-react";

type Directory = {
  provider_code: string;
  provider_name: string;
  report_count: number;
  note_count: number;
  earliest_report_date: string;
  latest_report_date: string;
};

function directoryURL(providerCode: string): string {
  return new URL(`${import.meta.env.BASE_URL}provider-files/${providerCode}`, window.location.origin).toString();
}

function ProviderDirectoryLinks({ refreshKey }: { refreshKey: string }) {
  const [items, setItems] = useState<Directory[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [copiedCode, setCopiedCode] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    fetch(`${import.meta.env.BASE_URL}api/imports/maituo-provider-directories`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: Directory[]; error?: string };
        if (!response.ok || !payload.success) throw new Error(payload.error || "无法读取服务商目录");
        setItems(payload.data ?? []);
      })
      .catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(reason instanceof Error ? reason.message : "无法读取服务商目录");
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [refreshKey]);

  const rows = useMemo(() => items.map((item) => ({ ...item, url: directoryURL(item.provider_code) })), [items]);

  const copyURL = async (providerCode: string, value: string) => {
    await navigator.clipboard.writeText(value);
    setCopiedCode(providerCode);
    window.setTimeout(() => setCopiedCode((current) => current === providerCode ? "" : current), 1_500);
  };

  return <section className="history-section provider-directories">
    <div className="history-header">
      <div><h2>服务商日报目录</h2><p>按稿件表笔记 ID 拆分 · 独立历史下载 URL</p></div>
      <span className="saved-total">{items.length} 个服务商</span>
    </div>
    {loading ? <div className="history-empty"><LoaderCircle size={20} className="spin" /><span>正在读取</span></div>
      : error ? <div className="history-empty directory-error"><FolderOpen size={20} /><span>{error}</span></div>
        : rows.length === 0 ? <div className="history-empty"><FolderOpen size={22} /><span>暂无服务商数据</span></div>
          : <div className="directory-table-wrap"><table className="directory-table"><thead><tr><th>服务商</th><th>历史范围</th><th>文件数</th><th>笔记数</th><th>URL</th><th>操作</th></tr></thead><tbody>
            {rows.map((item) => <tr key={item.provider_code}>
              <td><strong>{item.provider_name}</strong></td>
              <td>{item.report_count === 0 ? "暂无匹配日报" : item.earliest_report_date === item.latest_report_date ? item.latest_report_date : `${item.earliest_report_date} 至 ${item.latest_report_date}`}</td>
              <td>{item.report_count}</td>
              <td>{item.note_count}</td>
              <td><input className="directory-url" readOnly value={item.url} aria-label={`${item.provider_name} 日报目录 URL`} /></td>
              <td><div className="directory-actions">
                <button className="icon-button" onClick={() => void copyURL(item.provider_code, item.url)} aria-label={`复制 ${item.provider_name} 日报目录 URL`} title="复制 URL">{copiedCode === item.provider_code ? <Check size={16} /> : <Copy size={16} />}</button>
                <a className="icon-button" href={item.url} target="_blank" rel="noreferrer" aria-label={`打开 ${item.provider_name} 日报目录`} title="打开目录"><ExternalLink size={16} /></a>
              </div></td>
            </tr>)}
          </tbody></table></div>}
  </section>;
}

export default ProviderDirectoryLinks;
