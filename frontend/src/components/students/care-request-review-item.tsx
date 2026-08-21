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
    case "pickup_change_impact_changed":
      return "Der Betreuungsplan hat sich geändert. Bitte laden Sie die Seite neu und prüfen Sie die Termine noch einmal.";
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
  const decision = useCareRequestDecision(row, onDecided);
  const summary = careSummary(row.diff);
  return (
    <RequestReviewCard
      type="care_schedule"
      childName={`${row.first_name} ${row.last_name}`}
      summary={summary}
      submittedAt={row.created_at}
      reason={decision.reason}
      onReasonChange={decision.setReason}
      reasonPlaceholder="Begründung (Pflicht bei Ablehnung)"
      reasonError={decision.reasonError}
      busy={decision.busy}
      onApprove={() => void decision.decide(true)}
      onReject={() => void decision.decide(false)}
    >
      <CareRequestDetails row={row} error={decision.error} />
    </RequestReviewCard>
  );
}

function useCareRequestDecision(
  row: StaffCareRequest,
  onDecided: (notice: string) => void,
) {
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const decide = async (approve: boolean) => {
    const trimmed = reason.trim();
    if (!decisionInputReady(row, approve, trimmed, setError, setReasonError))
      return;
    setBusy(true);
    setError(null);
    try {
      await decideCareScheduleChangeRequest(
        row.id,
        approve,
        trimmed || undefined,
        row.impact_token,
      );
      onDecided(decisionNotice(row, approve));
    } catch (err) {
      handleDecisionError(err, row.id, setError, setBusy);
    }
  };
  return {
    reason,
    setReason: (value: string) => {
      setReason(value);
      setReasonError(false);
    },
    reasonError: reasonError
      ? "Für eine Ablehnung ist eine Begründung erforderlich."
      : undefined,
    busy,
    error,
    decide,
  };
}

function decisionInputReady(
  row: StaffCareRequest,
  approve: boolean,
  reason: string,
  setError: (message: string) => void,
  setReasonError: (value: boolean) => void,
): boolean {
  if (
    approve &&
    row.request_kind === "pickup_change" &&
    !row.impact_available
  ) {
    setError(
      "Die Anfrage kann nicht freigegeben werden. Bitte laden Sie die Seite neu.",
    );
    return false;
  }
  if (!approve && reason === "") {
    setReasonError(true);
    return false;
  }
  return true;
}

function handleDecisionError(
  err: unknown,
  requestID: string,
  setError: (message: string) => void,
  setBusy: (busy: boolean) => void,
) {
  const message = err instanceof Error ? err.message : String(err);
  const code = err instanceof CareRequestApiError ? err.code : undefined;
  logger.warn("care_request_review_decide_failed", {
    error: message,
    request_id: requestID,
    ...(code ? { code } : {}),
  });
  setError(decideErrorMessage(code));
  setBusy(false);
}

function CareRequestDetails({
  row,
  error,
}: Readonly<{ row: StaffCareRequest; error: string | null }>) {
  return (
    <>
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
        <RequestDiffRows diff={row.diff} />
        {row.request_kind === "pickup_change" && <PickupImpact row={row} />}
      </ReviewDiffPanel>
    </>
  );
}

function RequestDiffRows({
  diff,
}: Readonly<{ diff: StaffCareRequest["diff"] }>) {
  if (diff.length === 0)
    return <span className="text-sm text-gray-500">—</span>;
  return diff.map((entry) => (
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
  ));
}

function PickupImpact({ row }: Readonly<{ row: StaffCareRequest }>) {
  let content = (
    <p>Das Kind bleibt im Betreuungsplan für alle Termine eingeplant.</p>
  );
  if (!row.impact_available) {
    content = <p>Die Liste der betroffenen Termine ist nicht verfügbar.</p>;
  } else if (row.affected_blocks.length > 0) {
    content = (
      <>
        <p>Im Betreuungsplan wird das Kind von diesen Terminen abgemeldet:</p>
        <ul className="mt-1 list-disc space-y-0.5 pl-5">
          {row.affected_blocks.map((block) => (
            <li key={block.id}>
              {block.title}, {block.start_time}–{block.end_time} Uhr
            </li>
          ))}
        </ul>
      </>
    );
  }
  return (
    <div className="mt-3 border-t border-gray-200 pt-2">
      <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        Nach dem Freigeben
      </p>
      <div className="mt-1 text-sm text-gray-700">{content}</div>
    </div>
  );
}
