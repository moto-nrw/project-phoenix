"use client";

import { useState } from "react";

import { Alert } from "~/components/ui/alert";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { createLogger } from "~/lib/logger";
import {
  CareRequestApiError,
  type StaffCareRequest,
  decideCareScheduleChangeRequest,
} from "~/lib/care-request-review-api";

const logger = createLogger({ component: "CareRequestReviewItem" });

// A care request can only be APPROVED while parent messaging is on for the
// school and the submitting guardian still has access to the child. The backend
// refuses the other cases with a specific 409 code, because approving would
// change the child's weekly plan with no parent notice. Map those codes to the
// concrete recovery action (reject the request) instead of hiding them behind a
// generic failure, so the reviewer knows what to do with the still-pending row.
function decideErrorMessage(code: string | undefined): string {
  switch (code) {
    case "messaging_disabled":
      return "Nachrichten an Eltern sind für diese Schule deaktiviert. Die Anfrage kann nicht freigegeben werden, weil die Bezugsperson nicht über die Änderung informiert würde. Bitte die Anfrage stattdessen ablehnen.";
    case "guardian_access_revoked":
      return "Die anfragende Bezugsperson hat keinen Zugriff mehr auf dieses Kind. Die Anfrage kann nicht freigegeben werden. Bitte die Anfrage stattdessen ablehnen.";
    case "change_request_not_pending":
      return "Diese Anfrage wurde bereits entschieden oder von den Eltern zurückgezogen. Bitte die Seite neu laden.";
    case "pickup_change_conflict":
      return "Für diesen Tag wurde inzwischen bereits eine Änderung durch die OGS eingetragen. Bitte prüfen und die Anfrage gegebenenfalls ablehnen.";
    case "pickup_change_completed":
      return "Das Kind wurde bereits ausgecheckt. Die Abholzeit kann nicht mehr geändert werden.";
    case "pickup_change_expired":
      return "Der angefragte Tag liegt bereits in der Vergangenheit. Die Abholzeit kann nicht mehr übernommen werden. Bitte die Anfrage ablehnen.";
    default:
      return "Die Entscheidung konnte nicht gespeichert werden.";
  }
}

// One-line summary for the collapsed card: the distinct change kinds (e.g.
// "Abholzeit + Abholart"), taken from the part after "·" in each diff label.
function careSummary(diff: StaffCareRequest["diff"]): string {
  const kinds = [
    ...new Set(
      diff.map((entry) => {
        const parts = entry.label.split("·");
        return (parts.at(-1) ?? entry.label).trim();
      }),
    ),
  ];
  return kinds.join(" + ") || "Änderung";
}

function decisionNotice(row: StaffCareRequest, approve: boolean): string {
  if (row.request_kind === "pickup_change")
    return approve ? "Abholzeit übernommen" : "Abholzeit-Anfrage abgelehnt";
  return approve
    ? "Betreuungszeiten übernommen"
    : "Betreuungszeit-Anfrage abgelehnt";
}

/**
 * Eine offene Betreuungszeiten-/Abholzeit-Anfrage als entscheidbare Karte.
 * Ablehnen verlangt eine Begründung; nach der Entscheidung meldet onDecided
 * den Hinweistext und der Aufrufer entfernt die Zeile aus der Liste (#2432).
 */
export function CareRequestReviewItem({
  row,
  onDecided,
}: Readonly<{
  row: StaffCareRequest;
  onDecided: (notice: string) => void;
}>) {
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const decide = async (approve: boolean) => {
    const trimmed = reason.trim();
    if (!approve && trimmed === "") {
      // The backend requires a reason for rejections; surface that before
      // the request instead of a generic error after it.
      setReasonError(true);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await decideCareScheduleChangeRequest(
        row.id,
        approve,
        trimmed || undefined,
      );
      onDecided(decisionNotice(row, approve));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      const code = err instanceof CareRequestApiError ? err.code : undefined;
      logger.warn("care_request_review_decide_failed", {
        error: message,
        request_id: row.id,
        ...(code ? { code } : {}),
      });
      setError(decideErrorMessage(code));
      setBusy(false);
    }
  };

  // In der gemischten Liste muss jede Zeile ihre Anfrageart nennen; die
  // Detail-Arten (z. B. „Abholzeit + Abholart") bleiben dahinter sichtbar.
  const kindLabel =
    row.request_kind === "pickup_change" ? "Abholzeit" : "Betreuungszeiten";
  const details = careSummary(row.diff);
  const summary =
    details && details !== kindLabel ? `${kindLabel} · ${details}` : kindLabel;

  return (
    <RequestReviewCard
      type="care_schedule"
      childName={`${row.first_name} ${row.last_name}`}
      summary={summary}
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
      {error && (
        <div className="mb-2">
          <Alert type="error" message={error} />
        </div>
      )}
      <ReviewDiffPanel title="Änderungen">
        {row.request_reason && (
          <p className="mb-3 text-sm text-gray-700">
            <span className="font-medium text-gray-900">Grund der Eltern:</span>{" "}
            {row.request_reason}
          </p>
        )}
        {row.diff.length === 0 && (
          <span className="text-sm text-gray-500">—</span>
        )}
        {row.diff.map((entry) => (
          <div key={entry.label} className="text-sm">
            <span className="text-xs text-gray-500">{entry.label}: </span>
            <div className="flex flex-wrap items-baseline gap-2">
              <span className="text-gray-400 line-through">{entry.old}</span>
              <span className="text-gray-400" aria-hidden="true">
                →
              </span>
              <span className="font-medium text-gray-900">{entry.new}</span>
            </div>
          </div>
        ))}
      </ReviewDiffPanel>
    </RequestReviewCard>
  );
}
