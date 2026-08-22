import { useEffect, useRef, useState } from "react";
import { AlertCircle, Check, Clock3, ExternalLink, FileSpreadsheet, LoaderCircle } from "lucide-react";

export type SavedImport = {
  run_id: number;
  file_name: string;
  file_sha256: string;
  report_date: string;
  fetched: number;
  merged_rows?: number;
  present_sheets: string[];
  missing_sheets: string[];
  completed_at: string;
};

type MergedDailyNote = {
  note_id: string;
  note_url: string;
  category: string;
  placement: string;
  keyword_category_note: string | null;
  spend: number;
  search_users: number;
  search_cost: number | null;
  estimated_postback_cost: number | null;
  search_rate_pct: number | null;
  cpc: number | null;
  ctr_pct: number | null;
};

type MergedDailyReport = {
  report_date: string;
  total: number;
  items: MergedDailyNote[];
};

export type CalendarEntry<T> =
  | { kind: "report"; date: string; report: T }
  | { kind: "weekend" | "missing"; date: string };

const WEEKDAY_LABELS = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
const BUSINESS_WEEKEND_DAYS = new Set([5, 6]);
const moneyFormatter = new Intl.NumberFormat("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const countFormatter = new Intl.NumberFormat("zh-CN", { maximumFractionDigits: 0 });

function dateFromISO(value: string): Date {
  return new Date(`${value}T00:00:00Z`);
}

function dateToISO(value: Date): string {
  return value.toISOString().slice(0, 10);
}

export function weekdayLabel(value: string): string {
  return WEEKDAY_LABELS[dateFromISO(value).getUTCDay()];
}

export function buildHistoryCalendar<T extends { report_date: string }>(reports: T[]): CalendarEntry<T>[] {
  if (reports.length === 0) return [];
  const reportsByDate = new Map(reports.map((report) => [report.report_date, report]));
  const dates = [...reportsByDate.keys()].sort();
  const current = dateFromISO(dates[dates.length - 1]);
  const earliest = dateFromISO(dates[0]);
  const entries: CalendarEntry<T>[] = [];

  while (current >= earliest) {
    const date = dateToISO(current);
    const report = reportsByDate.get(date);
    if (report) {
      entries.push({ kind: "report", date, report });
    } else {
      entries.push({ kind: BUSINESS_WEEKEND_DAYS.has(current.getUTCDay()) ? "weekend" : "missing", date });
    }
    current.setUTCDate(current.getUTCDate() - 1);
  }
  return entries;
}

export function buildReportCalendar(reports: SavedImport[]): CalendarEntry<SavedImport>[] {
  return buildHistoryCalendar(reports);
}

export function formatSavedTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false
  }).format(date);
}

function formatMoney(value: number | null): string {
  return value === null ? "-" : `¥${moneyFormatter.format(value)}`;
}

function formatPercent(value: number | null): string {
  return value === null ? "-" : `${moneyFormatter.format(value)}%`;
}

type Props = {
  reports: SavedImport[];
  loading: boolean;
  expectedSheets: string[];
};

function SavedReportHistory({ reports, loading, expectedSheets }: Props) {
  const entries = buildReportCalendar(reports);
  const weekendCount = entries.filter((entry) => entry.kind === "weekend").length;
  const missingCount = entries.filter((entry) => entry.kind === "missing").length;
  const [selectedDate, setSelectedDate] = useState("");
  const [detail, setDetail] = useState<MergedDailyReport | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState("");
  const detailRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!selectedDate) return;
    const controller = new AbortController();
    const params = new URLSearchParams({ report_date: selectedDate });
    setDetail(null);
    setDetailError("");
    setDetailLoading(true);

    void fetch(`${import.meta.env.BASE_URL}api/imports/maituo-customer-daily?${params}`, { signal: controller.signal })
      .then(async (response) => {
        const payload = await response.json() as { success: boolean; data?: MergedDailyReport; error?: string };
        if (!response.ok || !payload.success || !payload.data) throw new Error(payload.error || "无法读取历史明细");
        setDetail({ ...payload.data, items: payload.data.items ?? [] });
      })
      .catch((reason: unknown) => {
        if (reason instanceof DOMException && reason.name === "AbortError") return;
        setDetailError(reason instanceof Error ? reason.message : "无法读取历史明细");
      })
      .finally(() => {
        if (!controller.signal.aborted) setDetailLoading(false);
      });

    return () => controller.abort();
  }, [selectedDate]);

  useEffect(() => {
    if (!selectedDate || detailLoading || (!detail && !detailError)) return;
    const animationFrame = window.requestAnimationFrame(() => {
      const detailElement = detailRef.current;
      if (!detailElement) return;
      detailElement.scrollIntoView({ block: "start" });
      detailElement.focus({ preventScroll: true });
    });
    return () => window.cancelAnimationFrame(animationFrame);
  }, [detail, detailError, detailLoading, selectedDate]);

  const selectReport = (reportDate: string) => {
    setSelectedDate(reportDate);
    window.requestAnimationFrame(() => {
      const detailElement = detailRef.current;
      if (!detailElement) return;
      detailElement.scrollIntoView({ block: "start" });
      detailElement.focus({ preventScroll: true });
    });
  };

  return <section className="history-section saved-section">
    <div className="history-header">
      <div><h2>已保存报表</h2><p>按报表日期从新到旧</p></div>
      <span className="saved-total">{reports.length} 份报表{weekendCount > 0 ? ` · ${weekendCount} 个周末日` : ""}{missingCount > 0 ? ` · ${missingCount} 个日期缺失` : ""}</span>
    </div>
    {loading ? <div className="history-empty"><LoaderCircle size={20} className="spin" /><span>正在读取</span></div>
      : reports.length === 0 ? <div className="history-empty"><FileSpreadsheet size={22} /><span>暂无已保存报表</span></div>
        : <div className="saved-table-wrap">
          <table className="saved-table"><thead><tr><th>报表日期</th><th>文件</th><th>表覆盖</th><th>合并行数</th><th>保存时间</th><th>状态</th></tr></thead><tbody>
            {entries.map((entry) => {
              if (entry.kind === "report") {
                const present = entry.report.present_sheets ?? [];
                const missing = entry.report.missing_sheets ?? expectedSheets.filter((name) => !present.includes(name));
                const selected = entry.date === selectedDate;
                return <tr
                  className={`saved-report-row${selected ? " selected" : ""}`}
                  key={entry.date}
                  tabIndex={0}
                  aria-label={`查看 ${entry.date} 合并明细`}
                  aria-selected={selected}
                  onClick={() => selectReport(entry.date)}
                  onKeyDown={(event) => {
                    if (event.key !== "Enter" && event.key !== " ") return;
                    event.preventDefault();
                    selectReport(entry.date);
                  }}
                >
                  <td><span className="report-date-value"><strong>{entry.date}</strong><small>{weekdayLabel(entry.date)}</small></span></td>
                  <td title={entry.report.file_name}>{entry.report.file_name}</td>
                  <td title={missing.length ? `缺少：${missing.join("、")}` : "五张表齐全"}><span className={missing.length ? "coverage-count partial" : "coverage-count"}>{present.length}/5</span>{missing.length ? <small className="coverage-missing">缺 {missing.length}</small> : null}</td>
                  <td>{(entry.report.merged_rows ?? entry.report.fetched).toLocaleString()}</td>
                  <td>{formatSavedTime(entry.report.completed_at)}</td>
                  <td><span className="saved-badge"><Check size={13} />已保存</span></td>
                </tr>;
              }
              const weekend = entry.kind === "weekend";
              return <tr className={`calendar-row ${entry.kind}`} key={entry.date}>
                <td><span className="report-date-value"><strong>{entry.date}</strong><small>{weekdayLabel(entry.date)}</small></span></td>
                <td><span className="calendar-note">{weekend ? "周末，不生成客户日报" : "未收到日报文件"}</span></td>
                <td>-</td><td>-</td><td>-</td>
                <td><span className={`saved-badge ${entry.kind}`}>{weekend ? <Clock3 size={13} /> : <AlertCircle size={13} />}{weekend ? "无需日报" : "缺少报表"}</span></td>
              </tr>;
            })}
          </tbody></table>
        </div>}
    {selectedDate ? <div
      id="saved-report-detail"
      className="history-detail"
      ref={detailRef}
      tabIndex={-1}
      aria-labelledby="saved-report-detail-title"
      aria-live="polite"
    >
      <div className="history-detail-header">
        <div><h3 id="saved-report-detail-title">{selectedDate} 合并明细</h3><p>笔记 ID + 场域</p></div>
        {detail && !detailLoading ? <span>{detail.total.toLocaleString()} 条</span> : null}
      </div>
      {detailLoading ? <div className="history-detail-state"><LoaderCircle size={20} className="spin" /><span>正在读取合并明细</span></div>
        : detailError ? <div className="history-detail-state error" role="alert"><AlertCircle size={20} /><span>{detailError}</span></div>
          : detail && detail.items.length === 0 ? <div className="history-detail-state"><FileSpreadsheet size={22} /><span>该日期暂无笔记明细</span></div>
            : detail ? <div className="history-detail-table-wrap">
              <table className="history-detail-table" aria-label={`${detail.report_date} 合并笔记明细`}>
                <thead><tr><th>笔记 ID</th><th>分类</th><th>场域</th><th>词类备注</th><th>消耗</th><th>回搜人数</th><th>回搜成本</th><th>预计回流后成本</th><th>回搜率</th><th>CPC</th><th>CTR</th></tr></thead>
                <tbody>{detail.items.map((item) => <tr key={`${item.note_id}\u0000${item.placement}`}>
                  <td>{item.note_url ? <a href={item.note_url} target="_blank" rel="noreferrer" title={item.note_id}>{item.note_id}<ExternalLink size={12} /></a> : <strong title={item.note_id}>{item.note_id}</strong>}</td>
                  <td>{item.category || "-"}</td>
                  <td><span className={`placement-swatch placement-${item.placement}`}>{item.placement || "-"}</span></td>
                  <td>{item.keyword_category_note || "-"}</td>
                  <td>{formatMoney(item.spend)}</td>
                  <td>{countFormatter.format(item.search_users)}</td>
                  <td>{formatMoney(item.search_cost)}</td>
                  <td>{formatMoney(item.estimated_postback_cost)}</td>
                  <td>{formatPercent(item.search_rate_pct)}</td>
                  <td>{formatMoney(item.cpc)}</td>
                  <td>{formatPercent(item.ctr_pct)}</td>
                </tr>)}</tbody>
              </table>
            </div> : null}
    </div> : null}
  </section>;
}

export default SavedReportHistory;
