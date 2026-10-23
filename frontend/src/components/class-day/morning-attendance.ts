// Anwesenheit am Vormittag in der Klassenansicht (#4790).
//
// Die Lehrkraft markiert heute bei jedem Kind „Da“ oder „Fehlt“. Ein Kind mit
// Meldung (krank, entschuldigt, Klassenfahrt) kommt vom Backend schon als
// „Fehlt“ (source "report"). Markierungen sammeln sich in einem Entwurf und
// gehen erst mit „Speichern“ an das Backend.

import type { ClassDayRow, MorningAttendanceStatus } from "~/lib/class-day-api";
import type { MorningAttendanceMark } from "~/lib/school-class-day-api";

/** Entwurf: Schlüssel der Zeile → markierter Zustand. */
export type MorningAttendanceDraft = Readonly<
  Record<string, MorningAttendanceStatus>
>;

/** Eigener Schlüsselraum für Klassenlisteneinträge (student_id ist dort 0). */
export function morningRowKey(row: ClassDayRow): string {
  return row.list_entry
    ? `entry-${row.list_entry_id}`
    : `student-${row.student_id}`;
}

/** Der Zustand, den die Zeile zeigt: Entwurf vor gespeichertem Stand. */
export function morningStatus(
  row: ClassDayRow,
  draft: MorningAttendanceDraft,
): MorningAttendanceStatus | null {
  return draft[morningRowKey(row)] ?? row.morning_attendance?.status ?? null;
}

/** Krank, entschuldigt und Klassenfahrt erklären ein Fehlen am Vormittag. */
export function hasReportedAbsence(row: ClassDayRow): boolean {
  return (
    row.status === "sick" ||
    row.status === "excused" ||
    row.status === "class_trip"
  );
}

/** Fehlt, ohne dass eine Meldung vorliegt: das Kind, um das es geht. */
export function missingWithoutReport(
  row: ClassDayRow,
  status: MorningAttendanceStatus | null,
): boolean {
  return status === "absent" && !hasReportedAbsence(row);
}

/** Die Zahlen des Vormittags als eine Zeile. */
export function morningSummaryLine(
  rows: readonly ClassDayRow[],
  draft: MorningAttendanceDraft,
): string {
  let present = 0;
  let absent = 0;
  let unreported = 0;
  let open = 0;
  for (const row of rows) {
    const status = morningStatus(row, draft);
    if (status === "present") present++;
    else if (status === "absent") {
      absent++;
      if (missingWithoutReport(row, status)) unreported++;
    } else open++;
  }
  const parts = [
    `${present} da`,
    `${absent} ${absent === 1 ? "fehlt" : "fehlen"}`,
  ];
  if (unreported > 0) parts.push(`${unreported} ohne Meldung`);
  if (open > 0) parts.push(`${open} noch offen`);
  return parts.join(" · ");
}

/** Alle Kinder ohne Zustand auf „Da“ setzen; der übrige Entwurf bleibt. */
export function markOpenPresent(
  rows: readonly ClassDayRow[],
  draft: MorningAttendanceDraft,
): MorningAttendanceDraft {
  const next: Record<string, MorningAttendanceStatus> = { ...draft };
  for (const row of rows) {
    if (morningStatus(row, draft) === null)
      next[morningRowKey(row)] = "present";
  }
  return next;
}

/** Die Markierungen des Entwurfs für das Backend. */
export function draftMarks(
  rows: readonly ClassDayRow[],
  draft: MorningAttendanceDraft,
): MorningAttendanceMark[] {
  const marks: MorningAttendanceMark[] = [];
  for (const row of rows) {
    const status = draft[morningRowKey(row)];
    if (!status) continue;
    if (row.list_entry && row.list_entry_id) {
      marks.push({ list_entry_id: row.list_entry_id, status });
    } else if (!row.list_entry) {
      marks.push({ student_id: row.student_id, status });
    }
  }
  return marks;
}
