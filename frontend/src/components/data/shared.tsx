"use client";
import { useState } from "react";
import { AlertCircle, Check, Copy, Inbox } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { copy } from "@/lib/copy/app";
import { formatUsd, microsToUsd } from "@/lib/money";
export function StatCard({
  title,
  value,
  description,
}: {
  title: string;
  value: React.ReactNode;
  description: string;
}) {
  return (
    <div className="stat-card">
      <span>{title}</span>
      <strong>{value}</strong>
      <small>{description}</small>
    </div>
  );
}
export function EmptyState({
  title = copy.empty,
  description,
  action,
}: {
  title?: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="empty-state">
      <span className="empty-icon">
        <Inbox size={25} />
      </span>
      <h2>{title}</h2>
      <p>{description}</p>
      {action}
    </div>
  );
}
export function ErrorState({
  message,
  requestId,
  retry,
}: {
  message: string;
  requestId?: string;
  retry?: () => void;
}) {
  return (
    <div className="error-state" role="alert">
      <AlertCircle size={20} />
      <h2>{copy.error}</h2>
      <p>{message}</p>
      {requestId && (
        <div>
          {copy.requestId}: <code>{requestId}</code>
          <CopyButton value={requestId} />
        </div>
      )}
      {retry && (
        <Button variant="outline" onClick={retry}>
          {copy.retry}
        </Button>
      )}
    </div>
  );
}
export function StatusBadge({
  status,
}: {
  status: "healthy" | "degraded" | "unavailable";
}) {
  const label =
    status === "healthy"
      ? copy.healthyLabel
      : status === "degraded"
        ? copy.degradedLabel
        : copy.unavailableLabel;
  return (
    <span className={`status-badge ${status}`}>
      <span className={`health-dot ${status}`} />
      {label}
    </span>
  );
}
export function MoneyText({
  value,
  micros,
  unit,
  maxFraction = 4,
}: {
  value?: number;
  micros?: number;
  unit?: "CPM" | "per imp.";
  maxFraction?: number;
}) {
  return (
    <span className="money-text">
      {formatUsd(micros !== undefined ? microsToUsd(micros) : (value ?? 0), {
        maxFraction,
      })}
      {unit && <small> {unit}</small>}
    </span>
  );
}
export function CopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label={copied ? copy.copied : copy.copy}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(value);
          setCopied(true);
          setTimeout(() => setCopied(false), 1500);
        } catch {
          toast.error(copy.copyFailed);
        }
      }}
    >
      {copied ? <Check size={14} /> : <Copy size={14} />}
    </Button>
  );
}
