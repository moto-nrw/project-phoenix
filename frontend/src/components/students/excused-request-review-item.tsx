"use client";

import { useLayoutEffect, useRef, useState } from "react";

import { ConsentCheckbox } from "~/components/ui/consent-checkbox";

import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import {
  type StaffExcusedRequest,
  decideExcusedAbsenceRequest,
} from "~/lib/excused-request-review-api";

const logger = createLogger({ component: "ExcusedRequestReviewItem" });

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

/** Satzanfang der Erfolgsmeldung nach der Entscheidung. */
function decisionSubject(row: StaffExcusedRequest): string {
  return row.absence_status === "sick"
    ? "Die Krankmeldung"
    : "Die entschuldigte Abmeldung";
}

/**
 * Was „Ersetzen“ an den Tagen mit Teilentschuldigung bewirkt. Wie beim
 * Entfernen gelten geänderte Blöcke und Abholzeit danach wieder wie geplant.
 */
function partialReplaceDescription(
  row: StaffExcusedRequest,
  days: readonly string[],
  fromByDate: Readonly<Record<string, string>>,
): string {
  const named = days
    .map((date) => `${formatDate(date)} (ab ${fromByDate[date]} Uhr)`)
    .join(", ");
  const subject =
    row.absence_status === "sick" ? "die Krankmeldung" : "die Entschuldigung";
  return `Am ${named} gilt dann ${subject} den ganzen Tag. Ohne Bestätigung ist Freigeben nicht möglich.`;
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
const CURRENT_STATUS_LABELS: Record<string, string> = {
  present: "Da",
  sick: "Krank",
  excused: "Entschuldigt",
  class_trip: "Ausflug",
};

/**
 * Was heute für einen Tag eingetragen ist, in Worten. Unbekannte Werte
 * bleiben als Rohwert stehen, statt als „Da" zu erscheinen.
 */
function currentStatusLabel(status: string | undefined): string {
  if (!status) return "Da";
  return CURRENT_STATUS_LABELS[status] ?? status;
}

export function ExcusedRequestReviewItem({
  row,
  onDecided,
  grouped = false,
  expectedVersion,
  currentStatusByDate,
  decisionDisabledReason,
  approveReasonRequired = false,
  partialAbsenceFromByDate,
}: Readonly<{
  row: StaffExcusedRequest;
  onDecided: (notice: string) => void;
  grouped?: boolean;
  /** Fassung, die entschieden werden soll — verhindert Überschreiben (#2267). */
  expectedVersion?: string;
  /** Aktueller Stand je Tag, damit „Aktuell → Gewünscht" möglich ist (#2267). */
  currentStatusByDate?: Readonly<Record<string, string>>;
  /** Warum hier gerade nicht einzeln entschieden werden kann (#2267). */
  decisionDisabledReason?: string;
  approveReasonRequired?: boolean;
  /** Tage mit Teilentschuldigung der OGS, je Tag die Uhrzeit (#5698). */
  partialAbsenceFromByDate?: Readonly<Record<string, string>>;
}>) {
  const { show: showError } = useApiErrorDisplay();
  const partialDays = Object.keys(partialAbsenceFromByDate ?? {})
    .filter((date) => row.dates.includes(date))
    .sort();
  // Freigeben ersetzt eine Teilentschuldigung nur, wenn das Team es hier
  // ausdrücklich bestätigt (#5698).
  const [replaceConfirmed, setReplaceConfirmed] = useState(false);
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState(false);
  const [busy, setBusy] = useState(false);
  // „Wiederholen“ entscheidet mit der aktuellen Begründung.
  const decideRef = useRef<(approve: boolean) => Promise<void>>(
    async () => undefined,
  );

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
      const replacing = approve && partialDays.length > 0 && replaceConfirmed;
      // Nur mitschicken, was die Entscheidung trägt: die Fassung, wenn die
      // Liste eine kennt, und das bestätigte Ersetzen (#5698).
      if (replacing) {
        await decideExcusedAbsenceRequest(
          row.id,
          approve,
          trimmed || undefined,
          expectedVersion,
          true,
        );
      } else if (expectedVersion) {
        await decideExcusedAbsenceRequest(
          row.id,
          approve,
          trimmed || undefined,
          expectedVersion,
        );
      } else {
        await decideExcusedAbsenceRequest(
          row.id,
          approve,
          trimmed || undefined,
        );
      }
      onDecided(
        approve
          ? `${decisionSubject(row)} ist bestätigt.`
          : `${decisionSubject(row)} ist abgelehnt.`,
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      const code = err instanceof ApiError ? err.code : undefined;
      logger.warn("excused_request_review_decide_failed", {
        error: message,
        request_id: row.id,
        ...(code ? { code } : {}),
      });
      await showError(err, {
        object: "die Anfrage",
        retry: () => void decideRef.current(approve),
      });
      setBusy(false);
    }
  };
  useLayoutEffect(() => {
    decideRef.current = decide;
  });

  return (
    <RequestReviewCard
      type="excused"
      childName={`${row.first_name} ${row.last_name}`}
      grouped={grouped}
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
      approveDisabled={partialDays.length > 0 && !replaceConfirmed}
      approveReasonRequired={approveReasonRequired}
      decisionDisabledReason={decisionDisabledReason}
      onApprove={() => void decide(true)}
      onReject={() => void decide(false)}
    >
      <ReviewDiffPanel title="Änderungen">
        {/* Ein Tag je Zeile mit dem aktuellen Stand daneben: sonst sieht man
            nur den Wunsch und nicht, was er ersetzt (#2267). */}
        {currentStatusByDate
          ? [...row.dates].sort().map((date) => (
              <div key={date} className="text-sm">
                <span className="text-xs text-gray-500">
                  {formatDate(date)}:{" "}
                </span>
                <span className="text-gray-500">
                  Aktuell{" "}
                  {partialAbsenceFromByDate?.[date]
                    ? `ab ${partialAbsenceFromByDate[date]} Uhr entschuldigt`
                    : currentStatusLabel(currentStatusByDate[date])}
                </span>
                <span className="text-gray-400" aria-hidden="true">
                  {" → "}
                </span>
                <span className="font-medium text-gray-900">
                  Gewünscht{" "}
                  {row.absence_status === "sick" ? "Krank" : "Entschuldigt"}
                </span>
              </div>
            ))
          : null}
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
            <span className="text-gray-400">–</span>
          )}
        </div>
      </ReviewDiffPanel>
      {partialDays.length > 0 && !decisionDisabledReason ? (
        <div className="mt-3">
          <ConsentCheckbox
            name={`replace-partial-${row.id}`}
            checked={replaceConfirmed}
            onChange={setReplaceConfirmed}
            label={`Teilentschuldigung durch ${row.absence_status === "sick" ? "Krankmeldung" : "Entschuldigung"} ersetzen`}
            description={partialReplaceDescription(
              row,
              partialDays,
              partialAbsenceFromByDate ?? {},
            )}
          />
        </div>
      ) : null}
    </RequestReviewCard>
  );
}
