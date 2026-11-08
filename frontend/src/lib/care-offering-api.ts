import { createLogger } from "~/lib/logger";
import type { Translations } from "~/lib/enrollment-translations";
import {
  enrollmentErrorReader,
  readEnrollmentJSON,
  sendEnrollmentRequest,
} from "~/lib/enrollment-http";

const logger = createLogger({ component: "CareOfferingAPI" });

export type DaysOfWeekMode = "fixed" | "parent_choice";

type CareOfferingAvailabilityMatch = "all" | "any";
type CareOfferingAvailabilityOperator = "in" | "not_in";
export interface CareOfferingAvailabilityCondition {
  source: "grade_level";
  operator: CareOfferingAvailabilityOperator;
  value: number[];
}
export interface CareOfferingAvailabilityRule {
  match: CareOfferingAvailabilityMatch;
  conditions: CareOfferingAvailabilityCondition[];
}

/**
 * How many offerings within the same selection_group a parent must pick.
 * Mirrors the backend SelectionRule* constants.
 */
export type CareSelectionRule =
  | "optional"
  | "exactly_one"
  | "at_least_one"
  | "at_most_one"
  | "at_most_one_per_day";

/** German labels for the selection-rule dropdown in the admin editor. */
export const SELECTION_RULE_LABELS: Record<CareSelectionRule, string> = {
  optional: "Optional (frei wählbar)",
  exactly_one: "Genau eines (Pflicht)",
  at_least_one: "Mindestens eines (Pflicht)",
  at_most_one: "Höchstens eines",
  at_most_one_per_day: "Höchstens eines pro Tag",
};

export interface CareOffering {
  id: string;
  phase_id: string;
  activity_group_id?: string | null;
  name: string;
  description?: string | null;
  days_of_week_mode: DaysOfWeekMode;
  available_days: string[];
  includes_holiday_care: boolean;
  includes_lunch: boolean;
  capacity?: number | null;
  price_cents?: number | null;
  is_active: boolean;
  is_required: boolean;
  counts_as_care?: boolean;
  auto_add_grade_levels?: number[];
  availability_rule?: CareOfferingAvailabilityRule | null;
  auto_add_trigger_offering_ids?: string[];
  sort_order: number;
  selection_group?: string | null;
  // Optional in the type (older rows + existing fixtures may omit it);
  // the backend always sends it, defaulting to "optional".
  selection_rule?: CareSelectionRule;
  /** Buchungsgebundene Gehzeit pro Wochentag ({"mon":"14:30"}). */
  pickup_times?: Record<string, string> | null;
  /** Von der Schule gepflegte Übersetzungen von Name und Beschreibung (#3377). */
  translations?: Translations;
  created_at: string;
  updated_at: string;
}

export interface CareOfferingInput {
  phase_id: number;
  activity_group_id?: number | null;
  name: string;
  description?: string | null;
  days_of_week_mode: DaysOfWeekMode;
  available_days: string[];
  includes_holiday_care: boolean;
  includes_lunch: boolean;
  capacity?: number | null;
  price_cents?: number | null;
  is_active: boolean;
  is_required: boolean;
  counts_as_care: boolean;
  auto_add_grade_levels: number[];
  availability_rule?: CareOfferingAvailabilityRule | null;
  auto_add_trigger_offering_ids: string[];
  sort_order: number;
  selection_group?: string | null;
  // Optional in the type (older rows + existing fixtures may omit it);
  // the backend always sends it, defaulting to "optional".
  selection_rule?: CareSelectionRule;
  /** Für aktive Angebote, die als Betreuung zählen, je Betreuungstag erforderlich. */
  pickup_times?: Record<string, string> | null;
  /** Vollständiges Übersetzungsdokument; `{}` löscht alle Übersetzungen. */
  translations?: Translations;
}

const BASE = "/api/enrollment/care-offerings";

const readError = enrollmentErrorReader(logger, "care_offering_request_failed");

export async function listCareOfferings(
  phaseId?: string | null,
): Promise<CareOffering[]> {
  const url = new URL(
    BASE,
    globalThis.window?.location.origin ?? "http://localhost",
  );
  if (phaseId) {
    url.searchParams.set("phase_id", phaseId);
  }
  const path = `${url.pathname}${url.search}`;
  const response = await sendEnrollmentRequest(path, { cache: "no-store" });
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungsangebote konnten nicht geladen werden",
    );
  }
  const list = await readEnrollmentJSON<CareOffering[]>(response);
  return Array.isArray(list) ? list : [];
}

export async function createCareOffering(
  input: CareOfferingInput,
): Promise<CareOffering> {
  const response = await sendEnrollmentRequest(BASE, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungsangebot konnte nicht angelegt werden",
    );
  }
  return readEnrollmentJSON<CareOffering>(response);
}

export async function updateCareOffering(
  id: string,
  input: CareOfferingInput,
): Promise<CareOffering> {
  const response = await sendEnrollmentRequest(
    `${BASE}/${encodeURIComponent(id)}`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungsangebot konnte nicht gespeichert werden",
    );
  }
  return readEnrollmentJSON<CareOffering>(response);
}

export async function deleteCareOffering(id: string): Promise<void> {
  const response = await sendEnrollmentRequest(
    `${BASE}/${encodeURIComponent(id)}`,
    {
      method: "DELETE",
    },
  );
  if (response.status === 204) return;
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungsangebot konnte nicht gelöscht werden",
    );
  }
}

export interface CloneCareOfferingInput {
  target_phase_id: number;
}

export async function cloneCareOffering(
  id: string,
  input: CloneCareOfferingInput,
): Promise<CareOffering> {
  const response = await sendEnrollmentRequest(
    `${BASE}/${encodeURIComponent(id)}/clone`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
  if (!response.ok) {
    throw await readError(response, "Klonen fehlgeschlagen");
  }
  return readEnrollmentJSON<CareOffering>(response);
}
