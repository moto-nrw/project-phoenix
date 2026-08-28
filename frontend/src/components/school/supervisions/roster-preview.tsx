"use client";

// Wer bei dieser Aufsicht erwartet wird — sichtbar, BEVOR sie startet (#2527).
//
// Die Frage vor dem Start ist nicht "wie viele", sondern "wer". Der Server
// liefert die Liste auch für einen geplanten Block; sie hier zu verschweigen
// und nur eine Zahl zu zeigen, macht die Seite leer und die Vorbereitung
// unmöglich. Rein zum Lesen: gebucht wird erst, wenn die Aufsicht läuft.

import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { Alert } from "~/components/ui/alert";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { rosterPickupTimeLabel } from "~/lib/timetable-roster-helpers";
import { isCareDayExpected } from "~/lib/timetable-types";
import type { TimetableRosterRow } from "~/lib/timetable-operations-types";

const ABSENCE_LABELS: Record<string, string> = {
  late: "Verspätet",
  excused: "Entschuldigt",
  sick: "Krank",
  field_trip: "Ausflug",
  other: "Sonstiges",
};

function childLine(row: TimetableRosterRow): string {
  return [row.schoolClass, row.groupName].filter(Boolean).join(" · ");
}

function ChildRow({
  row,
  pickupTimesLoaded,
  pickupTimesRedacted,
  onOpen,
}: Readonly<{
  row: TimetableRosterRow;
  pickupTimesLoaded?: boolean;
  pickupTimesRedacted?: boolean;
  onOpen: (row: TimetableRosterRow) => void;
}>) {
  const absence = row.substatus ? ABSENCE_LABELS[row.substatus] : null;
  const detail = childLine(row);
  const pickupTimeLabel = rosterPickupTimeLabel(
    row.pickupTime,
    pickupTimesLoaded,
    pickupTimesRedacted,
  );

  return (
    <li>
      <button
        type="button"
        onClick={() => onOpen(row)}
        className="flex w-full items-center gap-3 border-b border-gray-100 px-4 py-2.5 text-left transition-colors last:border-b-0 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <span className="min-w-0 flex-1">
          <span className="block truncate text-sm font-medium text-gray-900 underline decoration-gray-300 underline-offset-4">
            {row.studentName || `Kind ${row.studentId}`}
          </span>
          {detail ? (
            <span className="block truncate text-xs text-gray-500">
              {detail}
            </span>
          ) : null}
          {pickupTimeLabel === null ? null : (
            <span className="mt-0.5 block text-xs font-medium text-gray-700">
              Gehzeit: {pickupTimeLabel}
            </span>
          )}
        </span>
        {absence ? (
          <StatusDotBadge label={absence} color={LOCATION_COLORS.EXCUSED} />
        ) : null}
      </button>
    </li>
  );
}

function Section({
  title,
  rows,
  hint,
  pickupTimesLoaded,
  pickupTimesRedacted,
  onOpen,
}: Readonly<{
  title: string;
  rows: TimetableRosterRow[];
  hint?: string;
  pickupTimesLoaded?: boolean;
  pickupTimesRedacted?: boolean;
  onOpen: (row: TimetableRosterRow) => void;
}>) {
  if (rows.length === 0) return null;
  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <div className="border-b border-gray-100 bg-gray-50 px-4 py-2">
        <h3 className="text-sm font-semibold text-gray-900">
          {title} ({rows.length})
        </h3>
        {hint ? <p className="mt-0.5 text-xs text-gray-500">{hint}</p> : null}
      </div>
      <ul>
        {rows.map((row) => (
          <ChildRow
            key={row.studentId}
            row={row}
            pickupTimesLoaded={pickupTimesLoaded}
            pickupTimesRedacted={pickupTimesRedacted}
            onOpen={onOpen}
          />
        ))}
      </ul>
    </section>
  );
}

/**
 * Teilt die Liste in "kommt" und "heute nicht eingeplant" — dieselbe
 * Unterscheidung, die der laufende Roster trifft. Ein Kind, das der
 * Betreuungsplan heute nicht hierher setzt, darf nicht unter "wird erwartet"
 * stehen; es taucht trotzdem auf, weil es kommen kann.
 */
export function SupervisionRosterPreview({
  rows,
  pickupTimesLoaded,
  pickupTimesRedacted,
  onOpenStudent,
}: Readonly<{
  rows: readonly TimetableRosterRow[];
  pickupTimesLoaded?: boolean;
  pickupTimesRedacted?: boolean;
  onOpenStudent: (row: TimetableRosterRow) => void;
}>) {
  const expected = rows.filter(
    (row) => row.planned && isCareDayExpected(row.careDayStatus),
  );
  const notScheduled = rows.filter(
    (row) => row.planned && !isCareDayExpected(row.careDayStatus),
  );

  if (rows.length === 0) return null;

  return (
    <div className="space-y-4">
      {pickupTimesLoaded === false && !pickupTimesRedacted ? (
        <Alert
          type="warning"
          announce="polite"
          message="Die Gehzeiten konnten nicht geladen werden. Die Anwesenheitsliste bleibt verfügbar."
        />
      ) : null}
      <Section
        title="Diese Kinder werden erwartet"
        hint="Tippen Sie auf einen Namen. Sie sehen dann, wann das Kind geht, wer es abholen darf und wen Sie im Notfall anrufen."
        rows={expected}
        pickupTimesLoaded={pickupTimesLoaded}
        pickupTimesRedacted={pickupTimesRedacted}
        onOpen={onOpenStudent}
      />
      <Section
        title="Heute nicht eingeplant"
        hint="Diese Kinder sind heute nicht für die Betreuung gebucht. Kommen sie trotzdem, können Sie sie nach dem Start eintragen."
        rows={notScheduled}
        pickupTimesLoaded={pickupTimesLoaded}
        pickupTimesRedacted={pickupTimesRedacted}
        onOpen={onOpenStudent}
      />
    </div>
  );
}
