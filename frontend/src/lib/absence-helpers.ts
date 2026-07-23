// Shared helpers for staff-absence UI (#1419). Previously duplicated in
// abwesenheiten-tab.tsx and leave-requests-card.tsx; the /staff inbox is the
// third consumer, so the copies were consolidated here.

import { LOCATION_COLORS } from "~/lib/location-helper";

export const ABSENCE_TYPE_LABEL: Record<string, string> = {
  vacation: "Urlaub",
  sick: "Krank",
  training: "Fortbildung",
  other: "Sonstige",
  comp_time: "Freizeitausgleich",
};

// Brand hex per absence type, rendered via StatusDotBadge (colored dot +
// tinted label on a gray-50 pill) — never generic Tailwind hues.
export const ABSENCE_TYPE_HEX: Record<string, string> = {
  vacation: LOCATION_COLORS.OTHER_ROOM,
  sick: LOCATION_COLORS.SICK,
  training: LOCATION_COLORS.EXCUSED,
  other: LOCATION_COLORS.UNKNOWN,
  comp_time: LOCATION_COLORS.TRANSIT,
};

// Noun form for action labels ("Krankmeldung löschen", not "Krank löschen");
// unknown types fall back to the generic noun.
export function absenceTypeNoun(absenceType: string): string {
  if (absenceType === "sick") return "Krankmeldung";
  if (absenceType === "vacation") return "Urlaub";
  if (absenceType === "training") return "Fortbildung";
  if (absenceType === "comp_time") return "Freizeitausgleich";
  return "Abwesenheit";
}

export function formatAbsenceDate(iso: string): string {
  return new Date(iso).toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

export function formatAbsenceRange(start: string, end: string): string {
  return start === end
    ? formatAbsenceDate(start)
    : `${formatAbsenceDate(start)} - ${formatAbsenceDate(end)}`;
}

function isWorkday(date: Date): boolean {
  const dow = date.getDay();
  return dow !== 0 && dow !== 6;
}

// Mon-Fri working days inclusive (Feiertage kommen in Tranche 3). Used as a
// fallback when the row predates the backend-computed working-days field.
export function countWorkdaysInclusive(
  startISO: string,
  endISO: string,
): number {
  const start = new Date(`${startISO.slice(0, 10)}T00:00:00`);
  const end = new Date(`${endISO.slice(0, 10)}T00:00:00`);
  if (end.getTime() < start.getTime()) return 0;
  let n = 0;
  const cur = new Date(start);
  while (cur.getTime() <= end.getTime()) {
    if (isWorkday(cur)) n += 1;
    cur.setDate(cur.getDate() + 1);
  }
  return n;
}

export function formatDayCount(days: number): string {
  // German decimal comma, drop trailing ",0" so "5" prints as "5 Tage".
  const rounded = Math.round(days * 10) / 10;
  const display = Number.isInteger(rounded)
    ? rounded.toString()
    : rounded.toFixed(1).replace(".", ",");
  return `${display} ${rounded === 1 ? "Tag" : "Tage"}`;
}

// Shape-neutral input for dayCountFor — both the snake_case admin row
// (StaffAbsenceRow) and the camelCase MA-side StaffAbsence map onto it.
export interface AbsenceDayCountInput {
  readonly workingDays?: number | null;
  readonly dateStart: string;
  readonly dateEnd: string;
  readonly halfDay?: boolean;
  readonly startHalfDay?: boolean;
  readonly endHalfDay?: boolean;
  readonly hasBoundaryFields: boolean;
}

export function dayCountFor(a: AbsenceDayCountInput): number {
  if (a.workingDays != null) return a.workingDays;
  const startISO = a.dateStart.slice(0, 10);
  const endISO = a.dateEnd.slice(0, 10);
  const base = countWorkdaysInclusive(startISO, endISO);
  if (base <= 0) return base;
  if (!a.hasBoundaryFields) return a.halfDay ? base - 0.5 : base;
  const start = new Date(`${startISO}T00:00:00`);
  const end = new Date(`${endISO}T00:00:00`);
  const sameDay = startISO === endISO;
  let days = base;
  if (a.startHalfDay && isWorkday(start)) days -= 0.5;
  if (a.endHalfDay && !sameDay && isWorkday(end)) days -= 0.5;
  if (a.endHalfDay && sameDay && !a.startHalfDay && isWorkday(end)) {
    days -= 0.5;
  }
  return days;
}

export interface AbsenceStatusMeta {
  readonly label: string;
  readonly color: string;
}

// Status pill meta (label + LOCATION_COLORS hex, rendered via StatusDotBadge).
// `requestedLabel` lets the MA self-service card keep its "Wartet auf
// Antwort" wording while admin views show the shorter "Wartet".
export function absenceStatusMeta(
  status: string,
  options?: { readonly requestedLabel?: string },
): AbsenceStatusMeta {
  switch (status) {
    case "requested":
      return {
        label: options?.requestedLabel ?? "Wartet",
        color: LOCATION_COLORS.SICK,
      };
    case "question":
      return { label: "Rückfrage", color: LOCATION_COLORS.EXCUSED };
    case "approved":
      return { label: "Genehmigt", color: LOCATION_COLORS.GROUP_ROOM };
    case "declined":
      return { label: "Abgelehnt", color: LOCATION_COLORS.HOME };
    case "canceled":
      return { label: "Storniert", color: LOCATION_COLORS.UNKNOWN };
    case "reported":
      return { label: "Eingetragen", color: LOCATION_COLORS.UNKNOWN };
    default:
      return { label: status, color: LOCATION_COLORS.UNKNOWN };
  }
}

// Custom event dispatched after any absence decision/request mutation so the
// sidebar pending counter refetches without a page reload (#1419).
export const ABSENCES_REFRESH_EVENT = "staff-absences-refresh";

export function dispatchAbsencesRefresh(): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new Event(ABSENCES_REFRESH_EVENT));
  }
}
