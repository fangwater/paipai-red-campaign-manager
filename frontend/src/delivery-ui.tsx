import { CheckCircle2, CircleAlert, Info, LoaderCircle, XCircle, type LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { DeliveryAPIError } from "./delivery-api";

export type NoticeState = { tone: "success" | "error" | "info"; message: string } | null;

export function errorMessage(error: unknown): string {
  if (error instanceof DeliveryAPIError) return error.message;
  if (error instanceof Error) return error.message;
  return "请求失败，请稍后重试";
}

export function formatDateTime(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false
  }).format(date);
}

export function formatFen(value = 0): string {
  return new Intl.NumberFormat("zh-CN", { style: "currency", currency: "CNY" }).format(value / 100);
}

export function StatusPill({ value }: { value: string }) {
  const normalized = value.toLowerCase();
  const tone = ["approved", "active", "succeeded", "valid", "online"].includes(normalized)
    ? "success"
    : ["failed", "rejected", "invalid", "error", "cancelled"].includes(normalized)
      ? "danger"
      : ["pending_approval", "publishing", "queued", "warning"].includes(normalized)
        ? "warning"
        : "neutral";
  const labels: Record<string, string> = {
    draft: "草稿", validated: "已校验", pending_approval: "待审批", approved: "已批准",
    publishing: "发布中", paused: "已暂停", active: "投放中", failed: "失败", cancelled: "已取消",
    queued: "队列中", succeeded: "成功", rejected: "已拒绝", valid: "有效", invalid: "无效",
    dry_run: "演练", execute: "真实发布", operator: "运营审批", budget_owner: "预算审批",
    viewer: "只读", analyst: "分析", admin: "管理员"
  };
  return <span className={`dc-status ${tone}`}>{labels[normalized] || value}</span>;
}

export function Notice({ state, onDismiss }: { state: NoticeState; onDismiss?: () => void }) {
  if (!state) return null;
  const Icon = state.tone === "success" ? CheckCircle2 : state.tone === "error" ? XCircle : Info;
  return <div className={`dc-notice ${state.tone}`} role={state.tone === "error" ? "alert" : "status"}>
    <Icon size={16} />
    <span>{state.message}</span>
    {onDismiss ? <button type="button" onClick={onDismiss} aria-label="关闭消息">×</button> : null}
  </div>;
}

export function EmptyState({ icon: Icon = Info, title, detail, action }: { icon?: LucideIcon; title: string; detail?: string; action?: ReactNode }) {
  return <div className="dc-empty">
    <Icon size={22} />
    <strong>{title}</strong>
    {detail ? <p>{detail}</p> : null}
    {action}
  </div>;
}

export function LoadingState({ label = "正在加载" }: { label?: string }) {
  return <div className="dc-loading"><LoaderCircle size={18} className="spin" />{label}</div>;
}

export function JsonOutput({ value, empty = "暂无返回数据" }: { value: unknown; empty?: string }) {
  if (value === undefined || value === null) return <EmptyState title={empty} />;
  return <pre className="dc-json-output">{JSON.stringify(value, null, 2)}</pre>;
}

export function SectionTitle({ icon: Icon, title, meta, actions }: { icon: LucideIcon; title: string; meta?: string; actions?: ReactNode }) {
  return <header className="dc-section-title">
    <span className="dc-section-icon"><Icon size={16} /></span>
    <div><h2>{title}</h2>{meta ? <p>{meta}</p> : null}</div>
    {actions ? <div className="dc-section-actions">{actions}</div> : null}
  </header>;
}

export function FenInput({ value, onChange, min = 0, disabled = false, id }: { value?: number; onChange: (value: number) => void; min?: number; disabled?: boolean; id?: string }) {
  return <div className="dc-money-input"><span>¥</span><input id={id} type="number" min={min / 100} step="0.01" value={(value || 0) / 100} disabled={disabled} onChange={(event) => onChange(Math.round((Number(event.target.value) || 0) * 100))} /></div>;
}

export function AlertLine({ children }: { children: ReactNode }) {
  return <div className="dc-alert-line"><CircleAlert size={15} />{children}</div>;
}
