"use client";

import { useState } from "react";

import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { useToast } from "~/contexts/ToastContext";
import {
  ExcusedRequestApiError,
  type StaffExcusedRequest,
  decideExcusedAbsenceRequest,
} from "~/lib/excused-request-review-api";

const logger = createLogger({ component: "ExcusedRequestReviewItem" });

// Expected 409s: the parent withdrew the request / another staffer already
// decided it (reload), or the submitting guardian lost access to the child
// before approval (reject instead). Name the concrete recovery action rather
// than hiding it behind a generic failure.
function decideErrorMessage(code: string | undefined): string {
  switch (code) {
    case "guardian_access_revoked":
      return "Die anfragende Bezugsperson hat keinen Zugriff mehr auf dieses Kind. Die Abmeldung kann nicht freigegeben werden. Bitte die Anfrage stattdessen ablehnen.";
    case "change_request_not_pending":
      return "Diese Anfrage wurde bereits entschieden oder von den Eltern zurückgezogen. Bitte die Seite neu laden.";
    case "excused_request_status_conflict":
      return "Für einen dieser Tage wurde inzwischen ein neuerer Status gesetzt (z. B. krank oder Ausflug). Die Freigabe würde ihn überschreiben. Bitte die Anfrage ablehnen oder den neueren Status zuerst entfernen.";
    default:
      return "Die Entscheidung konnte nicht gespeichert werden.";
  }
}

// True when `next` (YYYY-MM-DD) is exactly the calendar day after `prev`.
function isNextDay(prev: string, next: string): boolean {
  const d = parseISODate(prev);
  d.setDate(d.getDate() + 1);
  return toISODate(d) === next;
}

// One-line summary for the collapsed card. The request endpoint accepts an
// arbitrary date list, so only collapse to a "first – last" range when the days
// are actually contiguous; otherwise list them so a Mon+Wed request is never
// shown as "Mon – Wed" (which would wrongly imply Tuesday is included too).
// Exportiert fuer die Historie, die dieselbe Kurzfassung zeigt.
export function datesSummary(dates: StaffExcusedRequest["dates"]): string {
  if (dates.length === 0) return "Keine Tage";
  const sorted = [...dates].sort((a, b) => a.localeCompare(b));
  if (sorted.length === 1) return formatDate(sorted[0]!);
  const contiguous = sorted.every(
    (d, i) => i === 0 || isNextDay(sorted[i - 1]!, d),
  );
  if (contiguous) {
    return `${formatDate(sorted[0]!)} – ${formatDate(sorted.at(-1)!)}`;
  }
  return sorted.map((d) => formatDate(d)).join(", ");
}

function absenceLabel(row: StaffExcusedRequest): string {
  return row.absence_status === "sick"
    ? "Krankmeldung"
    : "Entschuldigte Abmeldung";
}

/**
 * Eine offene Abwesenheits-Anfrage als entscheidbare Karte. Ablehnen
 * verlangt eine Begründung; nach der Entscheidung meldet onDecided den
 * Hinweistext und der Aufrufer entfernt die Zeile aus der Liste (#2432).
 */
export function ExcusedRequestReviewItem({
  row,
  onDecided,
}: Readonly<{
  row: StaffExcusedRequest;
  onDecided: (notice: string) => void;
}>) {
  const toast = useToast();
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState(false);
  const [busy, setBusy] = useState(false);

  const decide = async (approve: boolean) => {
    const trimmed = reason.trim();
    if (!approve && trimmed === "") {
      // The backend requires a reason for rejections; surface that before the
      // request instead of a generic error after it.
      setReasonError(true);
      return;
    }
    setBusy(true);
    try {
      await decideExcusedAbsenceRequest(row.id, approve, trimmed || undefined);
      onDecided(
        approve
          ? `${absenceLabel(row)} bestätigt`
          : `${absenceLabel(row)} abgelehnt`,
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      const code = err instanceof ExcusedRequestApiError ? err.code : undefined;
      logger.warn("excused_request_review_decide_failed", {
        error: message,
        request_id: row.id,
        ...(code ? { code } : {}),
      });
      toast.error(decideErrorMessage(code), { duration: 8000 });
      setBusy(false);
    }
  };

  return (
    <RequestReviewCard
      type="excused"
      childName={`${row.first_name} ${row.last_name}`}
      typeLabel={absenceLabel(row)}
      summary={datesSummary(row.dates)}
      submittedAt={row.created_at}
      reason={reason}
      onReasonChange={(value) => {
        setReason(value);
        setReasonError(false);
      }}
      reasonPlaceholder="Begründung (Pflicht bei Ablehnung)"
      reasonError={
        reasonError
          ? "Für eine Ablehnung ist eine Begründung erforderlich."
          : undefined
      }
      busy={busy}
      onApprove={() => void decide(true)}
      onReject={() => void decide(false)}
    >
      <ReviewDiffPanel title="Änderungen">
        <div className="text-sm">
          <span className="text-xs text-gray-500">Tage: </span>
          <span className="font-medium text-gray-900">
            {datesSummary(row.dates)}
          </span>
        </div>
        <div className="text-sm">
          <span className="text-xs text-gray-500">Notiz der Eltern: </span>
          {row.note.trim() ? (
            <span className="font-medium text-gray-900">{row.note}</span>
          ) : (
            <span className="text-gray-400">—</span>
          )}
        </div>
      </ReviewDiffPanel>
    </RequestReviewCard>
  );
}
