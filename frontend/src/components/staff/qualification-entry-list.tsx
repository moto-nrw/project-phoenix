"use client";

import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type { StammdatenQualifikation } from "~/lib/staff-api";
import {
  QUALIFICATION_STATUS_LABELS,
  type QualificationEntry,
} from "~/lib/staff-qualification-api";

/**
 * Die gespeicherten Zeilen der Stammdaten mit dem Status der Nachweise, dazu
 * die fehlenden Pflichtnachweise. Ohne Status (noch nicht geladen) zählt
 * nur „abgelaufen“ nach dem Berliner Tag.
 */
export function mergeQualificationEntries(
  rows: readonly StammdatenQualifikation[],
  entries: readonly QualificationEntry[] | undefined,
  today: string,
): QualificationEntry[] {
  const byId = new Map(
    (entries ?? []).flatMap((entry) =>
      entry.qualificationId ? [[entry.qualificationId, entry] as const] : [],
    ),
  );
  const stored = rows.map((row): QualificationEntry => {
    const entry = row.id ? byId.get(row.id) : undefined;
    return {
      staffId: entry?.staffId ?? "",
      qualificationId: row.id,
      typeId: row.typeId,
      name: entry?.name ?? row.name,
      acquiredOn: row.acquiredOn,
      expiresOn: row.expiresOn,
      mandatory: entry?.mandatory ?? false,
      status:
        entry?.status ??
        (row.expiresOn !== null && row.expiresOn < today ? "expired" : "valid"),
    };
  });
  const missing = (entries ?? []).filter((entry) => entry.status === "missing");
  return [...stored, ...missing];
}

/**
 * Nachweise einer Person mit Status (#4808): im Reiter Stammdaten und unter
 * „Meine Nachweise“ im Profil. Ein fehlender Pflichtnachweis steht mit
 * „Fehlt“ dabei. Reine Anzeige ohne Bedienelemente.
 */
export function QualificationEntryList({
  entries,
}: {
  readonly entries: readonly QualificationEntry[];
}) {
  return (
    <ul className="divide-y divide-gray-100">
      {entries.map((entry) => {
        const status = QUALIFICATION_STATUS_LABELS[entry.status];
        const meta =
          entry.status === "missing"
            ? "Pflicht für alle Mitarbeitenden, noch nicht eingetragen"
            : [
                entry.acquiredOn
                  ? `erworben ${formatDate(entry.acquiredOn)}`
                  : null,
                entry.expiresOn
                  ? `gültig bis ${formatDate(entry.expiresOn)}`
                  : null,
              ]
                .filter(Boolean)
                .join(" · ");
        return (
          <li
            key={
              entry.qualificationId ?? `missing-${entry.typeId ?? entry.name}`
            }
            className="flex flex-wrap items-center justify-between gap-2 py-2.5 first:pt-0 last:pb-0"
          >
            <div className="min-w-0">
              <p className="text-sm font-medium text-gray-800">{entry.name}</p>
              {meta && <p className="mt-0.5 text-xs text-gray-500">{meta}</p>}
            </div>
            {entry.status === "valid" ? null : (
              <StatusBadge label={status.label} tone={status.tone} />
            )}
          </li>
        );
      })}
    </ul>
  );
}
