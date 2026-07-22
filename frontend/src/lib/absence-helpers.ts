// Shared helpers for staff-absence UI (#1419). Previously duplicated in
// abwesenheiten-tab.tsx and leave-requests-card.tsx; the /staff inbox is the
// third consumer, so the copies were consolidated here.

export const ABSENCE_TYPE_LABEL: Record<string, string> = {
  vacation: "Urlaub",
  sick: "Krank",
  training: "Fortbildung",
  other: "Sonstige",
};

export const ABSENCE_TYPE_COLOR: Record<string, string> = {
  vacation: "bg-[#5080D8]/15 text-[#5080D8]",
  sick: "bg-[#EAB308]/15 text-amber-700",
  training: "bg-[#7C3AED]/15 text-purple-700",
  other: "bg-gray-100 text-gray-600",
};

// Noun form for action labels ("Krankmeldung löschen", not "Krank löschen");
// unknown types fall back to the generic noun.
export function absenceTypeNoun(absenceType: string): string {
  if (absenceType === "sick") return "Krankmeldung";
  if (absenceType === "vacation") return "Urlaub";
  if (absenceType === "training") return "Fortbildung";
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
  readonly className: string;
}

// Status pill meta. `requestedLabel` lets the MA self-service card keep its
// "Wartet auf Antwort" wording while admin views show the shorter "Wartet".
export function absenceStatusMeta(
  status: string,
  options?: { readonly requestedLabel?: string },
): AbsenceStatusMeta {
  switch (status) {
    case "requested":
      return {
        label: options?.requestedLabel ?? "Wartet",
        className: "bg-amber-50 text-amber-700",
      };
    case "question":
      return {
        label: "Rückfrage",
        className: "bg-[#7C3AED]/15 text-purple-700",
      };
    case "approved":
      return {
        label: "Genehmigt",
        className: "bg-[#83CD2D]/15 text-[#4a7a15]",
      };
    case "declined":
      return { label: "Abgelehnt", className: "bg-red-50 text-red-700" };
    case "canceled":
      return { label: "Storniert", className: "bg-gray-100 text-gray-500" };
    case "reported":
      return { label: "Eingetragen", className: "bg-gray-100 text-gray-700" };
    default:
      return { label: status, className: "bg-gray-100 text-gray-600" };
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
