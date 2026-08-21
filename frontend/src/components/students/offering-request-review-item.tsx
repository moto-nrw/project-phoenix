"use client";

import { useState } from "react";

import { Alert } from "~/components/ui/alert";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { Checkbox } from "~/components/ui/checkbox";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  OfferingRequestApiError,
  type OfferingRequestDiffLine,
  type OfferingRequestPreviewSelection,
  type StaffOfferingRequest,
  decideOfferingChangeRequest,
  previewOfferingChangeRequest,
} from "~/lib/offering-request-review-api";

const logger = createLogger({ component: "OfferingRequestReviewItem" });

// An approval genuinely applies the switch, so it can fail for reasons the
// office has to act on rather than retry. Name each one and say what to do; the
// row deliberately stays pending in all of these cases.
function decideErrorMessage(code: string | undefined): string {
  switch (code) {
    case "offering_change_capacity_full":
      return "Für ein gewünschtes Angebot ist kein Platz mehr frei. Die Anfrage bleibt offen: Bitte mit der Familie eine Alternative klären oder die Anfrage mit Begründung ablehnen.";
    case "change_request_not_pending":
      return "Diese Anfrage wurde bereits entschieden oder von den Eltern zurückgezogen. Bitte die Seite neu laden.";
    case "offering_changes_no_enrollment":
      return "Für dieses Kind liegt keine gültige Anmeldung mehr vor, auf die die Änderung angewendet werden könnte. Bitte die Anfrage ablehnen.";
    default:
      return "Die Entscheidung konnte nicht gespeichert werden.";
  }
}

// joinNames renders trigger names as „A“ und „B“ for the explanation line.
function joinNames(names: readonly string[]): string {
  return names.map((name) => `„${name}“`).join(" und ");
}

// automaticHint says why a line appeared. Kept to one short sentence: the
// review card must be readable at a glance.
function automaticHint(entry: OfferingRequestDiffLine): string {
  const names = entry.trigger_names ?? [];
  const attributedDays = entry.rule_days ?? entry.automatic_days;
  const partial = attributedDays !== undefined && attributedDays !== entry.new;
  if (names.length === 0) {
    return partial
      ? `Die Tage ${entry.automatic_days} kommen automatisch dazu.`
      : "Kommt automatisch dazu.";
  }
  return partial
    ? `Die Tage ${attributedDays} kommen automatisch dazu, weil ${joinNames(names)} gewählt ist.`
    : `Kommt automatisch dazu, weil ${joinNames(names)} gewählt ist.`;
}

// cascadeSourceName names the unticked line a greyed dependent hangs on.
function cascadeSourceName(
  entry: OfferingRequestDiffLine,
  diff: readonly OfferingRequestDiffLine[],
  removed: ReadonlySet<string>,
): string {
  for (const id of entry.trigger_ids ?? []) {
    if (!removed.has(id)) continue;
    const trigger = diff.find((line) => line.offering_id === id);
    if (trigger) return trigger.label;
  }
  return "";
}

/**
 * Eine offene Angebots-Anfrage als entscheidbare Karte, inklusive der
 * Mitbuchungs-Abwahl mit Vorschau-Kaskade (#2370). Ablehnen verlangt eine
 * Begründung; nach der Entscheidung meldet onDecided den Hinweistext und der
 * Aufrufer entfernt die Zeile aus der Liste (#2432).
 */
export function OfferingRequestReviewItem({
  row,
  onDecided,
}: Readonly<{
  row: StaffOfferingRequest;
  onDecided: (notice: string) => void;
}>) {
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Rule-added offerings the reviewer unticked for this request (#2370).
  const [excluded, setExcluded] = useState<readonly string[]>([]);
  const [preview, setPreview] = useState<
    readonly OfferingRequestPreviewSelection[] | undefined
  >(undefined);

  const decide = async (approve: boolean) => {
    const trimmed = reason.trim();
    if (!approve && trimmed === "") {
      setReasonError(true);
      return;
    }
    setBusy(true);
    setError(null);
    const excludedIds = approve ? excluded : [];
    try {
      if (excludedIds.length > 0) {
        await decideOfferingChangeRequest(
          row.id,
          approve,
          trimmed || undefined,
          excludedIds,
        );
      } else {
        await decideOfferingChangeRequest(
          row.id,
          approve,
          trimmed || undefined,
        );
      }
      onDecided(
        approve
          ? `Änderung übernommen, gültig ab ${formatDate(row.effective_from)}`
          : "Angebots-Anfrage abgelehnt",
      );
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      const code =
        err instanceof OfferingRequestApiError ? err.code : undefined;
      logger.warn("offering_request_review_decide_failed", {
        error: message,
        request_id: row.id,
        ...(code ? { code } : {}),
      });
      setError(decideErrorMessage(code));
      setBusy(false);
    }
  };

  const toggleExcluded = async (offeringId: string) => {
    const next = excluded.includes(offeringId)
      ? excluded.filter((id) => id !== offeringId)
      : [...excluded, offeringId];
    setBusy(true);
    setError(null);
    try {
      const nextPreview =
        next.length > 0
          ? await previewOfferingChangeRequest(row.id, next)
          : undefined;
      setExcluded(next);
      setPreview(nextPreview?.selections);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("offering_request_review_preview_failed", {
        error: message,
        request_id: row.id,
      });
      setError(
        "Die Vorschau konnte nicht aktualisiert werden. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setBusy(false);
    }
  };

  const unchanged = row.unchanged ?? [];
  const previewByOffering = new Map(
    preview?.map((selection) => [selection.offering_id, selection]),
  );
  const removed = new Set(
    row.diff
      .filter((entry) => {
        const selection = previewByOffering.get(entry.offering_id);
        return selection?.removed && selection.new !== entry.new;
      })
      .map((entry) => entry.offering_id),
  );

  const fullWithdrawal = row.full_withdrawal === true;

  return (
    <RequestReviewCard
      type="offering"
      childName={row.student_name}
      summary={`ab ${formatDate(row.effective_from)}`}
      badge={
        fullWithdrawal ? (
          <StatusBadge tone="red" label="Komplett-Abmeldung" />
        ) : undefined
      }
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
      {fullWithdrawal && (
        <div className="mt-3">
          <Alert
            type="error"
            message={`Damit wird ${row.student_name} von allen Angeboten abgemeldet. Danach ist kein Angebot mehr gebucht.`}
          />
        </div>
      )}
      <ReviewDiffPanel title="Änderungen">
        {row.diff.length === 0 && (
          <span className="text-sm text-gray-500">—</span>
        )}
        {row.diff.map((entry) => {
          const previewSelection = previewByOffering.get(entry.offering_id);
          const isExcluded = excluded.includes(entry.offering_id);
          const isRemoved = removed.has(entry.offering_id);
          const cascaded = isRemoved && !isExcluded;
          const previewChanged =
            previewSelection !== undefined &&
            previewSelection.new !== entry.new;
          const displayedNew = previewSelection?.new ?? entry.new;
          return (
            <div
              key={entry.offering_id}
              className={`text-sm ${isRemoved ? "opacity-50" : ""}`}
            >
              <span className="text-xs text-gray-500">{entry.label}: </span>
              {entry.automatic && (
                <StatusBadge tone="blue" label="Automatisch mitgebucht" />
              )}
              <div className="flex flex-wrap items-baseline gap-2">
                <span className="text-gray-400 line-through">{entry.old}</span>
                <span className="text-gray-400" aria-hidden="true">
                  →
                </span>
                <span
                  className={
                    isRemoved
                      ? "font-medium text-gray-500 line-through"
                      : "font-medium text-gray-900"
                  }
                >
                  {displayedNew}
                </span>
              </div>
              {entry.automatic &&
                !previewChanged &&
                !cascaded &&
                !isExcluded && (
                  <p className="mt-0.5 text-xs text-gray-500">
                    {automaticHint(entry)}
                  </p>
                )}
              {cascaded && (
                <p className="mt-0.5 text-xs text-gray-500">
                  Entfällt, weil{" "}
                  {`„${cascadeSourceName(entry, row.diff, removed)}“`} nicht
                  mitgebucht wird.
                </p>
              )}
              {entry.optoutable && !cascaded && (
                <label
                  htmlFor={`mitbuchen-${row.id}-${entry.offering_id}`}
                  className="mt-1 flex w-fit cursor-pointer items-center gap-2 text-xs text-gray-700"
                >
                  <Checkbox
                    id={`mitbuchen-${row.id}-${entry.offering_id}`}
                    checked={!isExcluded}
                    onChange={() => {
                      void toggleExcluded(entry.offering_id);
                    }}
                    disabled={busy}
                    aria-label={`${entry.label} automatisch mitbuchen`}
                  />
                  Automatisch mitbuchen
                </label>
              )}
              {isExcluded && (
                <p className="mt-0.5 text-xs text-gray-500">
                  Die Mitbuchungs-Regel gilt für diese Anfrage nicht.
                </p>
              )}
            </div>
          );
        })}
        {unchanged.length > 0 && (
          <div className="mt-3 border-t border-gray-200 pt-2">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              Bleibt gebucht
            </p>
            {unchanged.map((line) => (
              <p key={line.offering_id} className="text-sm text-gray-700">
                <span className="text-xs text-gray-500">{line.label}: </span>
                {line.days}
              </p>
            ))}
          </div>
        )}
        {row.note && (
          <p className="mt-2 text-xs text-gray-500">
            Nachricht der Eltern: {row.note}
          </p>
        )}
      </ReviewDiffPanel>
      <div className="mt-3 rounded-lg bg-gray-50 p-3">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Nach dem Freigeben bitte prüfen
        </p>
        <ul className="mt-1 list-disc space-y-0.5 pl-5 text-xs text-gray-600">
          <li>Gehzeiten des Kindes</li>
          <li>Zuordnung im Stundenplan</li>
          <li>Listen und Ausdrucke</li>
        </ul>
      </div>
    </RequestReviewCard>
  );
}
