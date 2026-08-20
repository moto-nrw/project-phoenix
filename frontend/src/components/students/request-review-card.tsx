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

// Eine Direkt-Korrektur ist keine entschiedene Anfrage, sondern eine Änderung
// der Verwaltung selbst (#2436) — eigene Kennzeichnung, eigenes Zeitwort.
const CORRECTION_META = { label: "Direkt-Korrektur", tone: "blue" as const };

type RequestReviewCardHistoryBase = {
  readonly decidedAt: string;
  readonly decidedByName?: string;
  readonly reason?: string;
};

/**
 * Eine entschiedene Anfrage (mit Status), eine Direkt-Korrektur der Verwaltung
 * (ohne) oder eine Anfrage, über die woanders entschieden wird. Als Union,
 * damit einer Anfrage-Karte der Status nicht fehlen kann.
 */
type RequestReviewCardHistory =
  | (RequestReviewCardHistoryBase & {
      readonly kind?: "decision";
      readonly status: string;
    })
  | (RequestReviewCardHistoryBase & { readonly kind: "correction" })
  // Eine Anfrage, die diese Karte nur anzeigt: entschieden wird sie auf einer
  // eigenen Seite (#2435). Sie bringt ihre eigene Status-Beschriftung mit,
  // weil ihr Statuswortschatz nicht der der vier Warteschlangen ist, und sie
  // hat noch keine Entscheidung, solange sie offen ist.
  | {
      readonly kind: "readonly";
      readonly label: string;
      readonly tone: StatusBadgeTone;
      readonly decidedAt?: string;
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
  badge,
  submittedAt,
  submittedByName,
  children,
  history,
  action,
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
  /**
   * Hinweis, der schon in der zugeklappten Zeile stehen muss, etwa die
   * Warnung vor einer Komplett-Abmeldung (#2434).
   */
  badge?: ReactNode;
  /** Einreichungszeitpunkt (ISO); rendert „Eingereicht am …" (#2432). */
  submittedAt?: string;
  /** Wer eingereicht hat; ergänzt die Zeile um „von …" (#2435). */
  submittedByName?: string;
  children?: ReactNode;
  /**
   * Aktion am Fuß der Lese-Karte, etwa der Weg zur Seite, auf der entschieden
   * wird. Nur zusammen mit `history` — die aufklappbare Entscheiden-Karte
   * trägt ihre eigenen Schaltflächen.
   */
  action?: ReactNode;
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
    const meta = readOnlyCardMeta(history);
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
          {submittedAt
            ? `Eingereicht am ${formatDate(submittedAt)}${
                submittedByName ? ` von ${submittedByName}` : ""
              }`
            : ""}
          {submittedAt && history.decidedAt ? " · " : ""}
          {history.decidedAt
            ? `${history.kind === "correction" ? "Geändert am " : "Entschieden am "}${formatDate(history.decidedAt)}${
                history.decidedByName ? ` von ${history.decidedByName}` : ""
              }`
            : ""}
        </p>
        {history.reason && (
          <p className="mt-1 text-sm text-gray-600 italic">
            „{history.reason}“
          </p>
        )}
        {children && <div className="mt-2">{children}</div>}
        {action && <div className="mt-3">{action}</div>}
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
        {badge}
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

/** Beschriftung und Farbe der Lese-Karte, je nach Art der Zeile. */
function readOnlyCardMeta(history: RequestReviewCardHistory): {
  label: string;
  tone: StatusBadgeTone;
} {
  if (history.kind === "correction") return CORRECTION_META;
  if (history.kind === "readonly") {
    return { label: history.label, tone: history.tone };
  }
  return (
    HISTORY_STATUS_META[history.status] ?? {
      label: history.status,
      tone: "gray",
    }
  );
}

/**
 * The calm "current → requested" panel shown inside an expanded review card,
 * matching the parent-side RequestDiffPanel surface (gray-50, muted uppercase
 * heading). Rows are passed as children so each queue keeps its own layout.
 */
export function ReviewDiffPanel({
  title,
  children,
}: Readonly<{ title: string; children: ReactNode }>) {
  return (
    <div className="mt-3 space-y-2 rounded-lg bg-gray-50 p-3">
      <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {title}
      </p>
      {children}
    </div>
  );
}
