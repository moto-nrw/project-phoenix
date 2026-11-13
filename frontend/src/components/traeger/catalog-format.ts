// Wörter und Zustände der Anmeldephasen des Trägers (#4704, ADR 0195):
// was das Übertragen an einer Schule tut, wie sie geantwortet hat und wie
// ein Feld in der Vorschau heißt.

import { SELECTION_RULE_LABELS } from "~/lib/care-offering-api";
import {
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
} from "~/lib/date-helpers";
import {
  PHASE_AUDIENCE_LABELS,
  PHASE_CARE_SELECTION_LABELS,
  PHASE_KIND_LABELS,
  PHASE_OVERFLOW_LABELS,
} from "~/lib/enrollment-phase-labels";
import type { CareWeekday } from "~/lib/traeger-care-billing-api";
import type {
  TraegerCatalogAction,
  TraegerCatalogOutcome,
  TraegerCatalogSchool,
  TraegerFieldChange,
  TraegerPhase,
  TraegerPhaseDetail,
} from "~/lib/traeger-catalog-api";
import { WEEKDAY_SHORT } from "./care-billing-format";
import { formatEuro } from "./contribution-format";

type BadgeTone = "green" | "gray" | "warning" | "red" | "blue";

export interface Badge {
  readonly label: string;
  readonly tone: BadgeTone;
}

/** Die Wochentage, an denen ein Angebot stattfinden kann. */
const CATALOG_WEEKDAYS: readonly CareWeekday[] = [
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
  "sat",
  "sun",
];

/** Montag bis Freitag: dort braucht ein Betreuungsangebot eine Gehzeit. */
export const SCHOOL_WEEKDAYS: readonly CareWeekday[] = [
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
];

export function weekdayList(days: readonly string[]): string {
  return days.map((day) => WEEKDAY_SHORT[day as CareWeekday] ?? day).join(", ");
}

/** „01.08.2027 bis 31.07.2028“. */
export function serviceRange(start: string, end: string): string {
  return `${formatDate(start)} bis ${formatDate(end)}`;
}

/** „01.03.2027, 08:00“ in Berliner Zeit. */
function formatInstant(instant: string): string {
  return `${formatBerlinDate(instant)}, ${formatBerlinTime(instant)}`;
}

/** Das letzte Ergebnis einer Schule, für die Spalte „Stand“. */
export function outcomeBadge(outcome: TraegerCatalogOutcome | null): Badge {
  switch (outcome) {
    case "applied":
      return { label: "Übernommen", tone: "green" };
    case "rejected":
      return { label: "Abgelehnt", tone: "red" };
    case "stale":
      return { label: "Nicht übertragen", tone: "warning" };
    case "withdrawn":
      return { label: "Zurückgezogen", tone: "gray" };
    default:
      return { label: "Noch nicht übertragen", tone: "gray" };
  }
}

/** Was das Übertragen an einer Schule tut, für die Vorschau. */
export function actionBadge(action: TraegerCatalogAction): Badge {
  switch (action) {
    case "create":
      return { label: "Wird neu angelegt", tone: "blue" };
    case "update":
      return { label: "Wird geändert", tone: "green" };
    case "withdraw":
      return { label: "Wird zurückgezogen", tone: "gray" };
  }
}

/** Der Zustand einer Trägerphase in der Liste. */
export function phaseStatusBadge(phase: TraegerPhase): Badge {
  if (phase.withdrawnAt) return { label: "Zurückgezogen", tone: "gray" };
  if (phase.releasedVersion === null) {
    return { label: "Noch nicht übertragen", tone: "warning" };
  }
  return { label: "Übertragen", tone: "green" };
}

function sameJSON(left: unknown, right: unknown): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

/** A school still holds a copy that the next rollout withdraws. */
function holdsCopy(school: {
  appliedVersion: number | null;
  outcome: TraegerCatalogOutcome | null;
}): boolean {
  return school.appliedVersion !== null && school.outcome !== "withdrawn";
}

/**
 * The schools the phase is meant for: every school of the Träger with
 * „Alle Schulen“, the chosen ones otherwise.
 */
export function targetSchoolIds(
  phase: TraegerPhase,
  allSchoolIds: readonly string[],
): string[] {
  if (phase.withdrawnAt) return [];
  if (phase.allSchools) return [...allSchoolIds];
  return phase.schools
    .filter((school) => school.selected)
    .map((school) => school.schoolId);
}

/**
 * Whether the schools do not yet hold what the office released or keeps in
 * its draft: a draft that differs from the released version, a withdrawal
 * not carried out, or a school without the current version.
 */
export function hasPendingRollout(
  phase: TraegerPhaseDetail,
  allSchoolIds: readonly string[],
): boolean {
  const targets = new Set(targetSchoolIds(phase, allSchoolIds));
  const rows = new Map(phase.schools.map((row) => [row.schoolId, row]));
  if (phase.withdrawnAt) return phase.schools.some(holdsCopy);
  if (phase.releasedVersion === null || !sameJSON(phase.draft, phase.released))
    return true;
  const offeringsPending = phase.offerings.some((offering) =>
    offering.withdrawnAt
      ? offering.schools.some(holdsCopy)
      : offering.released === null ||
        !sameJSON(offering.draft, offering.released),
  );
  if (offeringsPending) return true;
  for (const schoolId of targets) {
    const row = rows.get(schoolId);
    if (
      row?.outcome !== "applied" ||
      row.appliedVersion !== phase.releasedVersion
    )
      return true;
  }
  return phase.schools.some(
    (row) => !targets.has(row.schoolId) && holdsCopy(row),
  );
}

/** One row of the schools card: target schools and schools still holding a copy. */
export interface SchoolStanding {
  readonly schoolId: string;
  readonly row: TraegerCatalogSchool | null;
  /** The school leaves the selection; the next rollout withdraws its copy. */
  readonly leaving: boolean;
}

export function schoolStandings(
  phase: TraegerPhase,
  allSchoolIds: readonly string[],
): SchoolStanding[] {
  const targets = targetSchoolIds(phase, allSchoolIds);
  const rows = new Map(phase.schools.map((row) => [row.schoolId, row]));
  const standings: SchoolStanding[] = targets.map((schoolId) => ({
    schoolId,
    row: rows.get(schoolId) ?? null,
    leaving: false,
  }));
  for (const row of phase.schools) {
    if (!targets.includes(row.schoolId) && (holdsCopy(row) || row.outcome)) {
      standings.push({ schoolId: row.schoolId, row, leaving: holdsCopy(row) });
    }
  }
  return standings;
}

// --- Felder der Vorschau ------------------------------------------------------

const FIELD_LABELS: Readonly<Record<string, string>> = {
  name: "Name",
  kind: "Art",
  service_start_date: "Leistungsbeginn",
  service_end_date: "Leistungsende",
  enrollment_open_at: "Anmeldung ab",
  enrollment_close_at: "Anmeldung bis",
  audience: "Zielgruppe",
  care_overflow_mode: "Verhalten bei voller Betreuung",
  care_offering_selection_mode: "Betreuungsauswahl",
  is_active: "Aktiv",
  description: "Beschreibung",
  days_of_week_mode: "Wahl der Tage",
  available_days: "Wochentage",
  includes_holiday_care: "Mit Ferienbetreuung",
  includes_lunch: "Mit Mittagessen",
  capacity: "Plätze",
  price_cents: "Preis im Anmeldeformular",
  is_required: "Pflichtangebot",
  counts_as_care: "Zählt als Betreuungstag",
  selection_group: "Auswahlgruppe",
  selection_rule: "Auswahlregel",
  sort_order: "Reihenfolge",
  pickup_times: "Gehzeiten",
};

export function fieldLabel(field: string): string {
  return FIELD_LABELS[field] ?? field;
}

const DAYS_MODE_LABELS: Readonly<Record<string, string>> = {
  fixed: "Feste Tage",
  parent_choice: "Eltern wählen die Tage",
};

function labelFrom(
  labels: Readonly<Record<string, string>>,
  value: unknown,
): string {
  return typeof value === "string" ? (labels[value] ?? value) : "–";
}

/** Ein Wert der Vorschau so, wie das Büro ihn im Formular sieht. */
export function formatFieldValue(field: string, value: unknown): string {
  if (value === null || value === undefined || value === "") return "–";
  switch (field) {
    case "kind":
      return labelFrom(PHASE_KIND_LABELS, value);
    case "audience":
      return labelFrom(PHASE_AUDIENCE_LABELS, value);
    case "care_overflow_mode":
      return labelFrom(PHASE_OVERFLOW_LABELS, value);
    case "care_offering_selection_mode":
      return labelFrom(PHASE_CARE_SELECTION_LABELS, value);
    case "selection_rule":
      return labelFrom(SELECTION_RULE_LABELS, value);
    case "days_of_week_mode":
      return labelFrom(DAYS_MODE_LABELS, value);
    case "service_start_date":
    case "service_end_date":
      return typeof value === "string" ? formatDate(value) : "–";
    case "enrollment_open_at":
    case "enrollment_close_at":
      return typeof value === "string" ? formatInstant(value) : "–";
    case "price_cents":
      return typeof value === "number" ? formatEuro(value) : "–";
    case "available_days":
      return Array.isArray(value) && value.length > 0
        ? weekdayList(value.map(String))
        : "–";
    case "pickup_times": {
      if (typeof value !== "object") return "–";
      const entries = CATALOG_WEEKDAYS.flatMap((day) => {
        const time = (value as Record<string, unknown>)[day];
        return typeof time === "string"
          ? [`${WEEKDAY_SHORT[day]} ${time}`]
          : [];
      });
      return entries.length > 0 ? entries.join(", ") : "–";
    }
    default:
      if (typeof value === "boolean") return value ? "Ja" : "Nein";
      return String(value);
  }
}

/** The fields a preview shows: changed ones and those the school keeps. */
export function shownChanges(
  changes: readonly TraegerFieldChange[],
): TraegerFieldChange[] {
  return changes.filter(
    (change) => change.state === "changes" || change.state === "keeps",
  );
}
