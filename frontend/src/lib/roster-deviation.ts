import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";

/**
 * A Regeltermin whose hand-kept children list no longer matches the bookings
 * of a care offering (#5103). The backend detects it; the school decides
 * whether to connect the series to the offering or keep it by hand.
 */
export interface RosterDeviation {
  templateId: string;
  /** The offering the children list matches best. */
  offering: { id: string; name: string };
  /** Suggested filter, taken from the series' Klasse or Jahrgang target. */
  gradeLevels: number[];
  schoolClasses: string[];
  /** Children in the series without a booking on that weekday. */
  unbookedCount: number;
  /** Booked children the series does not list. */
  missingCount: number;
  /** Every offering that passes the rule, best first. */
  candidates: Array<{ id: string; name: string }>;
}

/** An offering source with its Klassen- or Jahrgangsfilter. */
export interface OfferingSelection {
  offeringIds: string[];
  gradeLevels: number[];
  schoolClasses: string[];
}

export interface OfferingPreviewChild {
  studentId: string;
  name: string;
}

interface OfferingPreviewWeekday {
  weekday: number;
  added: OfferingPreviewChild[];
  removed: OfferingPreviewChild[];
  keptCount: number;
}

interface OfferingPreviewSegment {
  templateId: string;
  validFrom: string;
  /** Exclusive end; undefined for the open segment. */
  validUntil?: string;
}

/** Who joins and who leaves per weekday when the series switches to a source. */
export interface OfferingPreview {
  from: string;
  weekdays: OfferingPreviewWeekday[];
  segments: OfferingPreviewSegment[];
}

interface BackendOfferingRef {
  id: number;
  name: string;
}

interface BackendRosterDeviation {
  template_id: number;
  offering: BackendOfferingRef;
  grade_levels?: number[] | null;
  school_classes?: string[] | null;
  unbooked_count: number;
  missing_count: number;
  candidates?: BackendOfferingRef[] | null;
}

export interface BackendRosterDeviationsResponse {
  deviations?: BackendRosterDeviation[] | null;
}

interface BackendPreviewChild {
  student_id: number;
  name: string;
}

export interface BackendOfferingPreview {
  from: string;
  weekdays?: Array<{
    weekday: number;
    added?: BackendPreviewChild[] | null;
    removed?: BackendPreviewChild[] | null;
    kept_count: number;
  }> | null;
  segments?: Array<{
    template_id: number;
    valid_from: string;
    valid_until?: string | null;
  }> | null;
}

export interface OfferingConnectionBody {
  offering_ids: number[];
  grade_levels: number[];
  school_classes: string[];
  effective_date: string;
}

const offeringRef = (offering: BackendOfferingRef) => ({
  id: String(offering.id),
  name: offering.name,
});

export function mapRosterDeviations(
  raw: BackendRosterDeviationsResponse,
): RosterDeviation[] {
  return (raw.deviations ?? []).map((deviation) => ({
    templateId: String(deviation.template_id),
    offering: offeringRef(deviation.offering),
    gradeLevels: deviation.grade_levels ?? [],
    schoolClasses: deviation.school_classes ?? [],
    unbookedCount: deviation.unbooked_count,
    missingCount: deviation.missing_count,
    candidates: (deviation.candidates ?? []).map(offeringRef),
  }));
}

const previewChild = (child: BackendPreviewChild): OfferingPreviewChild => ({
  studentId: String(child.student_id),
  name: child.name,
});

export function mapOfferingPreview(
  raw: BackendOfferingPreview,
): OfferingPreview {
  return {
    from: raw.from,
    weekdays: (raw.weekdays ?? []).map((day) => ({
      weekday: day.weekday,
      added: (day.added ?? []).map(previewChild),
      removed: (day.removed ?? []).map(previewChild),
      keptCount: day.kept_count,
    })),
    segments: (raw.segments ?? []).map((segment) => ({
      templateId: String(segment.template_id),
      validFrom: segment.valid_from,
      validUntil: segment.valid_until ?? undefined,
    })),
  };
}

function joinNames(names: readonly string[]): string {
  if (names.length <= 1) return names[0] ?? "";
  return `${names.slice(0, -1).join(", ")} und ${names.at(-1)}`;
}

function childCount(count: number, singular: string, plural: string): string {
  return `${count} ${count === 1 ? singular : plural}`;
}

/** The hint text on the Regeltermin card and in the details. */
export function describeRosterDeviation(deviation: RosterDeviation): string {
  const parts: string[] = [];
  if (deviation.unbookedCount > 0) {
    parts.push(
      `${childCount(deviation.unbookedCount, "Kind", "Kinder")} ohne Buchung`,
    );
  }
  if (deviation.missingCount > 0) {
    parts.push(
      deviation.missingCount === 1
        ? "1 angemeldetes Kind fehlt"
        : `${deviation.missingCount} angemeldete Kinder fehlen`,
    );
  }
  parts.push(`passt zu „${deviation.offering.name}“`);
  return `Weicht von Anmeldungen ab: ${parts.join(" · ")}`;
}

/** One line naming which children of the offering the series takes. */
export function describeSelectionFilter(selection: OfferingSelection): string {
  if (selection.schoolClasses.length > 0) {
    // Some schools store the class as „Klasse 1b“; the word stands once.
    const classes = selection.schoolClasses.map((schoolClass) =>
      schoolClass.trim().replace(/^klasse\s+/i, ""),
    );
    return classes.length === 1
      ? `Nur Kinder der Klasse ${classes[0]}`
      : `Nur Kinder der Klassen ${joinNames(classes)}`;
  }
  if (selection.gradeLevels.length > 0) {
    const grades = [...selection.gradeLevels].sort((a, b) => a - b).map(String);
    return grades.length === 1
      ? `Nur Kinder des Jahrgangs ${grades[0]}`
      : `Nur Kinder der Jahrgänge ${joinNames(grades)}`;
  }
  return "Alle angemeldeten Kinder des Angebots";
}

export function previewQuery(
  selection: OfferingSelection,
  from?: string,
): URLSearchParams {
  const params = new URLSearchParams();
  params.set("offering_ids", selection.offeringIds.join(","));
  if (selection.gradeLevels.length > 0) {
    params.set("grade_levels", selection.gradeLevels.join(","));
  }
  if (selection.schoolClasses.length > 0) {
    params.set("school_classes", selection.schoolClasses.join(","));
  }
  if (from) params.set("from", from);
  return params;
}

export function connectionBody(
  selection: OfferingSelection,
  effectiveDate: string,
): OfferingConnectionBody {
  return {
    offering_ids: selection.offeringIds.map(Number),
    grade_levels: selection.gradeLevels,
    school_classes: selection.schoolClasses,
    effective_date: effectiveDate,
  };
}

/**
 * Names the later periods of the series a switch also reaches; null when the
 * series has only one period from the switch date on.
 */
export function laterSegmentSentence(preview: OfferingPreview): string | null {
  const later = preview.segments
    .map((segment) => segment.validFrom)
    .filter((validFrom) => validFrom > preview.from)
    .sort();
  if (preview.segments.length < 2 || later.length === 0) return null;
  const periods = later.map((validFrom) => `ab ${formatDate(validFrom)}`);
  return `Gilt für alle Termine ab ${formatDate(preview.from)}, auch für ${
    periods.length === 1 ? "den Abschnitt" : "die Abschnitte"
  } ${joinNames(periods)}.`;
}

function previousDay(iso: string): string {
  const date = parseISODate(iso);
  date.setDate(date.getDate() - 1);
  return toISODate(date);
}

/**
 * The days from the preview date on that belong to another period of the
 * series than the edited one; an edit in the Regeltermin editor leaves them
 * as they are. Null when the edit reaches every remaining day.
 */
export function untouchedSegmentSentence(
  preview: OfferingPreview,
  editedTemplateId: string,
): string | null {
  const ranges = preview.segments
    .filter(
      (segment) =>
        segment.templateId !== editedTemplateId &&
        (segment.validUntil === undefined || segment.validUntil > preview.from),
    )
    .map((segment) => {
      const start =
        segment.validFrom > preview.from ? segment.validFrom : preview.from;
      return segment.validUntil
        ? `vom ${formatDate(start)} bis ${formatDate(previousDay(segment.validUntil))}`
        : `ab ${formatDate(start)}`;
    });
  if (ranges.length === 0) return null;
  return `Die Termine ${joinNames(ranges)} gehören zu ${
    ranges.length === 1 ? "einem früheren Abschnitt" : "anderen Abschnitten"
  } und behalten ihre Kinderliste. „Mit Angebot verbinden“ an der Karte des Regeltermins stellt alle Abschnitte auf einmal um.`;
}

const WEEKDAY_NAMES: Record<number, string> = {
  1: "Montag",
  2: "Dienstag",
  3: "Mittwoch",
  4: "Donnerstag",
  5: "Freitag",
  6: "Samstag",
  7: "Sonntag",
};

export function weekdayName(weekday: number): string {
  return WEEKDAY_NAMES[weekday] ?? `Tag ${weekday}`;
}
