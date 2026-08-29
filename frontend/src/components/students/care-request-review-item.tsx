"use client";

import { useState } from "react";

import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { createLogger } from "~/lib/logger";
import { useToast } from "~/contexts/ToastContext";
import {
  CareRequestApiError,
  type StaffCareRequest,
  decideCareScheduleChangeRequest,
} from "~/lib/care-request-review-api";
import type { RequestDiffEntry } from "~/lib/messaging-status";

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
    case "care_day_managed_by_booking":
      return "Dieser Betreuungstag gehört zu einem gebuchten Angebot. Ändern Sie zuerst die Buchung des Kindes. Lehnen Sie diese Anfrage danach ab.";
    default:
      return "Die Entscheidung konnte nicht gespeichert werden.";
  }
}

/**
 * Zusammenfassung für die zugeklappte Zeile, aus den Diff-Labels
 * ("Freitag · Abholzeit", "21.08.2026 · Abholzeit"). Vorn steht, WOFÜR die
 * Änderung gilt, dahinter, WAS sie ändert; beide Hälften werden für sich
 * entdoppelt, damit eine Anfrage über mehrere Tage die Zeile nicht sprengt.
 *
 * Der Geltungstag muss vorn stehen: er ist der einzige Unterschied zwischen
 * einer Anfrage für einen einzelnen Tag (Datum) und einer dauerhaften
 * (Wochentag), und die Spalte wird bei wenig Platz hinten abgeschnitten. Ohne
 * ihn lasen sich beide Arten als „Abholzeit" und eine Ganztagskoordinatorin
 * hat eine Tages-Anfrage für eine dauerhafte Änderung gehalten (#2480).
 */
export function careSummary(
  diff: readonly RequestDiffEntry[],
  requestKind: StaffCareRequest["request_kind"],
): string {
  const days = new Set<string>();
  const kinds = new Set<string>();
  for (const entry of diff) {
    const parts = entry.label.split("·").map((part) => part.trim());
    const kind = parts.at(-1);
    if (kind) kinds.add(kind);
    const day = parts.length > 1 ? parts[0] : undefined;
    if (day) days.add(day);
  }
  if (kinds.size === 0)
    return requestKind === "pickup_change" ? "Abholzeit" : "Betreuungszeiten";
  const what = [...kinds].join(" + ");
  if (days.size === 0) return what;
  // Ab drei Tagen nur noch die Anzahl: ausgeschrieben ("Montag, Dienstag,
  // Mittwoch, Donnerstag, Freitag · …") wird die Spalte hinten abgeschnitten,
  // und abgeschnitten würde ausgerechnet die Änderungsart. „3 Wochentage"
  // hält den Unterschied zu einem Datum, um den es hier geht.
  const when = days.size > 2 ? `${days.size} Wochentage` : [...days].join(", ");
  return `${when} · ${what}`;
}

/**
 * Beschriftung der Art-Pille. Eine Anfrage für einen einzelnen Tag heißt nicht
 * „Betreuungszeiten": dieses Wort steht in der App für den dauerhaften
 * Wochenplan, und genau diese Verwechslung ist der Grund für #2480.
 */
export function careTypeLabel(
  requestKind: StaffCareRequest["request_kind"],
): string | undefined {
  return requestKind === "pickup_change" ? "Einzelner Tag" : undefined;
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
  grouped = false,
}: Readonly<{
  row: StaffCareRequest;
  onDecided: (notice: string) => void;
  grouped?: boolean;
}>) {
  const decision = useCareRequestDecision(row, onDecided);
  return (
    <RequestReviewCard
      type="care_schedule"
      typeLabel={careTypeLabel(row.request_kind)}
      childName={`${row.first_name} ${row.last_name}`}
      grouped={grouped}
      summary={careSummary(row.diff, row.request_kind)}
      submittedAt={row.created_at}
      reason={decision.reason}
      onReasonChange={decision.setReason}
      reasonPlaceholder="Begründung (Pflicht bei Ablehnung)"
      reasonError={decision.reasonError}
      busy={decision.busy}
      onApprove={() => void decision.decide(true)}
      onReject={() => void decision.decide(false)}
    >
      <CareRequestDetails row={row} />
    </RequestReviewCard>
  );
}

function useCareRequestDecision(
  row: StaffCareRequest,
  onDecided: (notice: string) => void,
) {
  const toast = useToast();
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState(false);
  const [busy, setBusy] = useState(false);
  // Fehler laufen als Toast, nicht als Kasten in der Karte: sie sind die
  // Ausnahme, und der Platz gehört dem, worüber entschieden wird.
  const showError = (message: string) =>
    toast.error(message, { duration: 8000 });
  const decide = async (approve: boolean) => {
    const trimmed = reason.trim();
    if (!decisionInputReady(row, approve, trimmed, showError, setReasonError))
      return;
    setBusy(true);
    try {
      await decideCareScheduleChangeRequest(
        row.id,
        approve,
        trimmed || undefined,
        row.impact_token,
      );
      onDecided(decisionNotice(row, approve));
    } catch (err) {
      handleDecisionError(err, row.id, showError, setBusy);
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

function CareRequestDetails({ row }: Readonly<{ row: StaffCareRequest }>) {
  return (
    <>
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
