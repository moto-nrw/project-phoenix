"use client";

// Die Gegenüberstellung „bisher → gewünscht“ eines Änderungswunschs der
// Eltern. Die Prüfung der Schule und das Träger-Büro (#5979) zeigen sie
// gleich; das Modul hält nur die Anzeige, damit die Vertragsseite des
// Trägers nicht die ganze Prüfung der Schule lädt.

import type { ReactNode } from "react";
import { ReviewDiffPanel } from "~/components/students/review-diff-panel";
import { Checkbox } from "~/components/ui/checkbox";
import { SectionLabel } from "~/components/ui/section-label";
import { StatusBadge } from "~/components/ui/status-badge";
import type {
  OfferingRequestDiffLine,
  OfferingRequestPreviewSelection,
  StaffOfferingRequest,
} from "~/lib/offering-request-review-api";

const NO_EXCLUDED_OFFERINGS: readonly string[] = [];

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

/** Shared request comparison, used by school review and the contract office. */
export function OfferingRequestChanges({
  row,
  preview,
  excluded = NO_EXCLUDED_OFFERINGS,
  busy = false,
  onToggleExcluded,
  waitlist,
}: Readonly<{
  row: Pick<StaffOfferingRequest, "id" | "diff" | "unchanged" | "note">;
  preview?: readonly OfferingRequestPreviewSelection[];
  excluded?: readonly string[];
  busy?: boolean;
  onToggleExcluded?: (offeringId: string) => Promise<void>;
  /** The school's course waitlist places; the contract office shows none. */
  waitlist?: ReactNode;
}>) {
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

  return (
    <ReviewDiffPanel title="Änderungen">
      {row.diff.length === 0 && (
        <span className="text-sm text-gray-500">–</span>
      )}
      {row.diff.map((entry) => {
        const previewSelection = previewByOffering.get(entry.offering_id);
        const isExcluded = excluded.includes(entry.offering_id);
        const isRemoved = removed.has(entry.offering_id);
        const cascaded = isRemoved && !isExcluded;
        const previewChanged =
          previewSelection !== undefined && previewSelection.new !== entry.new;
        const displayedNew = previewSelection?.new ?? entry.new;
        return (
          <div
            key={entry.offering_id}
            className={`text-sm wrap-anywhere ${isRemoved ? "opacity-50" : ""}`}
          >
            <span className="text-xs text-gray-500">{entry.label}: </span>
            {/* Eine Kursanfrage sieht sonst aus wie jede andere
                    Angebotsänderung (#3075). Das Mitarbeiterportal ist
                    deutsch und lädt die Eltern-Texte nicht: der Schlüssel
                    stand dort roh im Badge (#4767). */}
            {entry.is_course && <StatusBadge tone="green" label="Kurs" />}
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
            {entry.automatic && !previewChanged && !cascaded && !isExcluded && (
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
            {entry.optoutable && onToggleExcluded && !cascaded && (
              <label
                htmlFor={`mitbuchen-${row.id}-${entry.offering_id}`}
                className="mt-1 flex w-fit cursor-pointer items-center gap-2 text-xs text-gray-700"
              >
                <Checkbox
                  id={`mitbuchen-${row.id}-${entry.offering_id}`}
                  checked={!isExcluded}
                  onChange={() => {
                    void onToggleExcluded?.(entry.offering_id);
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
      {waitlist}
      {unchanged.length > 0 && (
        <div className="mt-3 border-t border-gray-200 pt-2">
          <SectionLabel as="p">Bleibt gebucht</SectionLabel>
          {unchanged.map((line) => (
            <p
              key={line.offering_id}
              className="text-sm wrap-anywhere text-gray-700"
            >
              <span className="text-xs text-gray-500">{line.label}: </span>
              {line.days}
            </p>
          ))}
        </div>
      )}
      {row.note && (
        <p className="mt-2 text-xs wrap-anywhere whitespace-pre-line text-gray-500">
          Nachricht der Eltern: {row.note}
        </p>
      )}
    </ReviewDiffPanel>
  );
}
