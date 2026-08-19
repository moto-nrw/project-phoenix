"use client";

import { type ReactNode, useState } from "react";
import { ChevronDown } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";

const HISTORY_STATUS_META: Record<
  string,
  { label: string; tone: StatusBadgeTone }
> = {
  approved: { label: "Freigegeben", tone: "green" },
  rejected: { label: "Abgelehnt", tone: "red" },
  withdrawn: { label: "Zurückgezogen", tone: "gray" },
  auto_applied: { label: "Automatisch übernommen", tone: "gray" },
};

type RequestReviewCardHistory = {
  readonly status: string;
  readonly decidedAt: string;
  readonly decidedByName?: string;
  readonly reason?: string;
};

/**
 * One pending parent change request in the staff Änderungsanfragen queue, in the
 * calm card look of the parent portal. Collapsed by default so a long queue
 * stays scannable: the row shows the child + a one-line summary of what changed;
 * clicking it expands the full "current → requested" diff (passed as children),
 * an optional reason, and Freigeben / Ablehnen. Shared by the master-data and
 * care-schedule review lists. Color is intentionally sparse — monochrome
 * buttons, no filled green/red.
 */
export function RequestReviewCard({
  childName,
  summary,
  submittedAt,
  children,
  history,
  reason,
  onReasonChange,
  reasonPlaceholder,
  reasonError,
  busy,
  onApprove,
  onReject,
}: Readonly<{
  childName: string;
  summary?: string;
  /** Einreichungszeitpunkt (ISO); rendert „Eingereicht am …" (#2432). */
  submittedAt?: string;
  children?: ReactNode;
  history?: RequestReviewCardHistory;
  reason?: string;
  onReasonChange?: (value: string) => void;
  reasonPlaceholder?: string;
  reasonError?: string;
  busy?: boolean;
  onApprove?: () => void;
  onReject?: () => void;
}>) {
  const [open, setOpen] = useState(false);
  if (history) {
    const meta = HISTORY_STATUS_META[history.status] ?? {
      label: history.status,
      tone: "gray" as const,
    };
    return (
      <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
        <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span className="font-medium text-gray-900">{childName}</span>
          {summary && (
            <span className="text-sm text-gray-600">· {summary}</span>
          )}
          <StatusBadge label={meta.label} tone={meta.tone} />
        </div>
        <p className="mt-1 text-xs text-gray-500">
          {submittedAt ? `Eingereicht am ${formatDate(submittedAt)} · ` : ""}
          Entschieden am {formatDate(history.decidedAt)}
          {history.decidedByName ? ` von ${history.decidedByName}` : ""}
        </p>
        {history.reason && (
          <p className="mt-1 text-sm text-gray-600 italic">
            „{history.reason}“
          </p>
        )}
        {children && <div className="mt-2">{children}</div>}
      </div>
    );
  }

  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-gray-50/60 sm:px-5"
      >
        <span className="min-w-0 flex-1 truncate text-sm font-semibold text-gray-900">
          {childName}
        </span>
        <span className="hidden shrink-0 truncate text-sm text-gray-500 sm:block">
          {summary}
        </span>
        <ChevronDown
          className={`h-4 w-4 shrink-0 text-gray-400 transition-transform ${open ? "rotate-180" : ""}`}
          aria-hidden="true"
        />
      </button>
      {open && (
        <div className="border-t border-gray-100 px-4 pb-4 sm:px-5">
          {submittedAt && (
            <p className="mt-3 text-xs text-gray-500">
              Eingereicht am {formatDate(submittedAt)}
            </p>
          )}
          {children}
          <div className="mt-4 space-y-2">
            <Input
              aria-label="Begründung"
              value={reason ?? ""}
              placeholder={reasonPlaceholder}
              disabled={busy}
              onChange={(e) => onReasonChange?.(e.target.value)}
            />
            {reasonError && (
              <p className="text-moto-red-strong text-xs">{reasonError}</p>
            )}
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={busy}
                onClick={onReject}
              >
                Ablehnen
              </Button>
              <Button
                type="button"
                variant="primary"
                size="md"
                disabled={busy}
                onClick={onApprove}
              >
                Freigeben
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * The calm "current → requested" panel shown inside an expanded review card,
 * matching the parent-side RequestDiffPanel surface (gray-50, muted uppercase
 * heading). Rows are passed as children so each queue keeps its own layout.
 */
export function ReviewDiffPanel({
  children,
}: Readonly<{ children: ReactNode }>) {
  return (
    <div className="mt-3 space-y-2 rounded-lg bg-gray-50 p-3">
      <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        Änderungen
      </p>
      {children}
    </div>
  );
}
