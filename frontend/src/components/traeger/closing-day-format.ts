/**
 * Anzeige der Schließtage und Feiertage des Trägers (#4702): Zeiträume, Tage
 * und die Schulen, für die ein Schließtag gilt.
 */
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import type {
  TraegerCalendar,
  TraegerCalendarSchool,
  TraegerClosingDay,
} from "~/lib/traeger-closing-days";

/** „Fr, 07.05.2027“ */
export function formatDayWithWeekday(iso: string): string {
  return parseISODate(iso).toLocaleDateString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

/** „07.05.2027“ oder „07.05.2027 bis 08.05.2027“. */
export function formatClosingRange(
  day: Pick<TraegerClosingDay, "startDate" | "endDate">,
): string {
  if (day.startDate === day.endDate) return formatDayWithWeekday(day.startDate);
  return `${formatDate(day.startDate)} bis ${formatDate(day.endDate)}`;
}

/** Die Kalendertage von start bis end, beide eingeschlossen. */
export function isoDaysBetween(start: string, end: string): string[] {
  const days: string[] = [];
  const cursor = parseISODate(start);
  for (let i = 0; i < 400; i++) {
    const iso = toISODate(cursor);
    if (iso > end) break;
    days.push(iso);
    cursor.setDate(cursor.getDate() + 1);
  }
  return days;
}

/** „1 Tag“, „3 Tage“ */
export function dayCountLabel(
  day: Pick<TraegerClosingDay, "startDate" | "endDate">,
): string {
  const count = isoDaysBetween(day.startDate, day.endDate).length;
  return count === 1 ? "1 Tag" : `${count} Tage`;
}

/** „Alle Schulen“ oder die Namen der Schulen, für die der Tag gilt. */
export function closingDaySchoolsLabel(
  day: Pick<TraegerClosingDay, "schoolIds">,
  schools: readonly TraegerCalendarSchool[],
): string {
  if (schools.length > 0 && schools.every((s) => day.schoolIds.includes(s.id)))
    return "Alle Schulen";
  const names = schools
    .filter((school) => day.schoolIds.includes(school.id))
    .map((school) => school.name);
  return names.length > 0 ? names.join(", ") : "–";
}

export type CalendarDayKind = "holiday" | "closing";

export interface CalendarDayMark {
  readonly kind: CalendarDayKind;
  /** Was an dem Tag ist, für Tooltip und Vorlesen. */
  readonly labels: readonly string[];
}

/**
 * Die markierten Tage der Jahresübersicht. Ein Feiertag nennt sein
 * Bundesland, wenn die Schulen in mehreren Ländern liegen. Ist ein Tag
 * beides, zeigt er den Feiertag.
 */
export function calendarDayMarks(
  calendar: TraegerCalendar,
): Map<string, CalendarDayMark> {
  const marks = new Map<string, { kind: CalendarDayKind; labels: string[] }>();
  const severalStates = calendar.regions.length > 1;
  for (const day of calendar.closingDays) {
    for (const iso of isoDaysBetween(day.startDate, day.endDate)) {
      const mark = marks.get(iso) ?? { kind: "closing", labels: [] };
      mark.labels.push(`Schließtag: ${day.reason}`);
      marks.set(iso, mark);
    }
  }
  for (const region of calendar.regions) {
    for (const holiday of region.holidays) {
      const mark = marks.get(holiday.date) ?? { kind: "holiday", labels: [] };
      mark.kind = "holiday";
      mark.labels.unshift(
        severalStates
          ? `Feiertag in ${region.name}: ${holiday.name}`
          : `Feiertag: ${holiday.name}`,
      );
      marks.set(holiday.date, mark);
    }
  }
  return marks;
}

/** Die Schulen eines Bundeslands, für die Liste der Feiertage. */
export function schoolsInState(
  schools: readonly TraegerCalendarSchool[],
  code: string,
): readonly TraegerCalendarSchool[] {
  return schools.filter((school) => school.federalStateCode === code);
}
