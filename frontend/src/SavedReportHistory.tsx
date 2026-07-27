import { AlertCircle, Check, Clock3, FileSpreadsheet, LoaderCircle } from "lucide-react";

export type SavedImport = {
  run_id: number;
  file_name: string;
  file_sha256: string;
  report_date: string;
  fetched: number;
  present_sheets: string[];
  missing_sheets: string[];
  completed_at: string;
};

type CalendarEntry =
  | { kind: "report"; date: string; report: SavedImport }
  | { kind: "weekend" | "missing"; date: string };

const WEEKDAY_LABELS = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
const BUSINESS_WEEKEND_DAYS = new Set([5, 6]);

function dateFromISO(value: string): Date {
  return new Date(`${value}T00:00:00Z`);
}

function dateToISO(value: Date): string {
  return value.toISOString().slice(0, 10);
}

function weekdayLabel(value: string): string {
  return WEEKDAY_LABELS[dateFromISO(value).getUTCDay()];
}

export function buildReportCalendar(reports: SavedImport[]): CalendarEntry[] {
  if (reports.length === 0) return [];
  const reportsByDate = new Map(reports.map((report) => [report.report_date, report]));
  const dates = [...reportsByDate.keys()].sort();
  const current = dateFromISO(dates[dates.length - 1]);
  const earliest = dateFromISO(dates[0]);
  const entries: CalendarEntry[] = [];

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

function formatSavedTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false
  }).format(date);
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

  return <section className="history-section saved-section">
    <div className="history-header">
      <div><h2>已保存报表</h2><p>按报表日期从新到旧</p></div>
      <span className="saved-total">{reports.length} 份报表{weekendCount > 0 ? ` · ${weekendCount} 个周末日` : ""}{missingCount > 0 ? ` · ${missingCount} 个日期缺失` : ""}</span>
    </div>
    {loading ? <div className="history-empty"><LoaderCircle size={20} className="spin" /><span>正在读取</span></div>
      : reports.length === 0 ? <div className="history-empty"><FileSpreadsheet size={22} /><span>暂无已保存报表</span></div>
        : <div className="saved-table-wrap">
          <table className="saved-table"><thead><tr><th>报表日期</th><th>文件</th><th>表覆盖</th><th>数据行</th><th>保存时间</th><th>状态</th></tr></thead><tbody>
            {entries.map((entry) => {
              if (entry.kind === "report") {
                const present = entry.report.present_sheets ?? [];
                const missing = entry.report.missing_sheets ?? expectedSheets.filter((name) => !present.includes(name));
                return <tr key={entry.date}>
                  <td><span className="report-date-value"><strong>{entry.date}</strong><small>{weekdayLabel(entry.date)}</small></span></td>
                  <td title={entry.report.file_name}>{entry.report.file_name}</td>
                  <td title={missing.length ? `缺少：${missing.join("、")}` : "五张表齐全"}><span className={missing.length ? "coverage-count partial" : "coverage-count"}>{present.length}/5</span>{missing.length ? <small className="coverage-missing">缺 {missing.length}</small> : null}</td>
                  <td>{entry.report.fetched.toLocaleString()}</td>
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
  </section>;
}

export default SavedReportHistory;
