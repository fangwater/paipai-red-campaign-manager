import { useEffect, useMemo, useState } from "react";
import { Check, Copy, ExternalLink, FolderOpen, LoaderCircle } from "lucide-react";

type Directory = {
  subaccount: string;
  account_id: string;
  report_count: number;
  earliest_report_date: string;
  latest_report_date: string;
};

function directoryURL(accountID: string): string {
  return new URL(`${import.meta.env.BASE_URL}subaccount-files/${accountID}`, window.location.origin).toString();
}

function SubaccountDirectoryLinks({ refreshKey }: { refreshKey: string }) {
  const [items, setItems] = useState<Directory[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [copiedID, setCopiedID] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    fetch(`${import.meta.env.BASE_URL}api/imports/maituo-subaccount-directories`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: Directory[]; error?: string };
        if (!response.ok || !payload.success) throw new Error(payload.error || "无法读取子账户目录");
        setItems(payload.data ?? []);
      })
      .catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setError(reason instanceof Error ? reason.message : "无法读取子账户目录");
      })
      .finally(() => setLoading(false));
    return () => controller.abort();
  }, [refreshKey]);

  const rows = useMemo(() => items.map((item) => ({ ...item, url: directoryURL(item.account_id) })), [items]);

  const copyURL = async (accountID: string, value: string) => {
    await navigator.clipboard.writeText(value);
    setCopiedID(accountID);
    window.setTimeout(() => setCopiedID((current) => current === accountID ? "" : current), 1_500);
  };

  return <section className="history-section subaccount-directories">
    <div className="history-header">
      <div><h2>子账户文件目录</h2><p>独立历史下载 URL</p></div>
      <span className="saved-total">{items.length} 个子账户</span>
    </div>
    {loading ? <div className="history-empty"><LoaderCircle size={20} className="spin" /><span>正在读取</span></div>
      : error ? <div className="history-empty directory-error"><FolderOpen size={20} /><span>{error}</span></div>
        : rows.length === 0 ? <div className="history-empty"><FolderOpen size={22} /><span>暂无子账户数据</span></div>
          : <div className="directory-table-wrap"><table className="directory-table"><thead><tr><th>子账户</th><th>历史范围</th><th>文件数</th><th>URL</th><th>操作</th></tr></thead><tbody>
            {rows.map((item) => <tr key={item.account_id}>
              <td><strong>{item.subaccount}</strong></td>
              <td>{item.earliest_report_date === item.latest_report_date ? item.latest_report_date : `${item.earliest_report_date} 至 ${item.latest_report_date}`}</td>
              <td>{item.report_count}</td>
              <td><input className="directory-url" readOnly value={item.url} aria-label={`${item.subaccount} 文件目录 URL`} /></td>
              <td><div className="directory-actions">
                <button className="icon-button" onClick={() => void copyURL(item.account_id, item.url)} aria-label={`复制 ${item.subaccount} 文件目录 URL`} title="复制 URL">{copiedID === item.account_id ? <Check size={16} /> : <Copy size={16} />}</button>
                <a className="icon-button" href={item.url} target="_blank" rel="noreferrer" aria-label={`打开 ${item.subaccount} 文件目录`} title="打开目录"><ExternalLink size={16} /></a>
              </div></td>
            </tr>)}
          </tbody></table></div>}
  </section>;
}

export default SubaccountDirectoryLinks;
