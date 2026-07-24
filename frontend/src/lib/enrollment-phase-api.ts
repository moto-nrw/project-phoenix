import { createLogger } from "~/lib/logger";
import { readEnrollmentError } from "~/lib/enrollment-error-messages";

const logger = createLogger({ component: "EnrollmentPhaseAPI" });

export type PhaseKind = "school_year" | "holiday" | "custom";
// Who may apply to a phase (#1663). "open" = everyone (incl. anonymous);
// "new_students" = anonymous allowed but children already enrolled at the
// school are rejected server-side; "linked_parents" = hidden from the public
// listing, only parent accounts with a guardian link + submit permission may
// apply via the parents portal.
export type PhaseAudience =
  "open" | "new_students" | "existing_students" | "linked_parents";
export type PhaseCareOverflowMode = "waitlist" | "reject" | "allow";
export type PhaseCareOfferingSelectionMode =
  "optional" | "at_least_one" | "exactly_one";

export interface Phase {
  id: string;
  name: string;
  kind: PhaseKind;
  service_start_date: string; // YYYY-MM-DD
  service_end_date: string; // YYYY-MM-DD
  enrollment_open_at?: string | null; // RFC3339
  enrollment_close_at?: string | null; // RFC3339
  form_schema_id?: string | null;
  calendar_period_id?: string | null;
  show_status_reason_to_parent: boolean;
  care_overflow_mode: PhaseCareOverflowMode;
  care_offering_selection_mode: PhaseCareOfferingSelectionMode;
  is_active: boolean;
  // Rollover columns — populated only on phases created by the
  // "verlängern" flow. NULL/false on fresh phases.
  rollover_source_phase_id?: string | null;
  rollover_mode?: RolloverMode | null;
  rollover_auto_approve?: boolean;
  rollover_deadline?: string | null;
  rollover_bumps_grade?: boolean;
  // Concrete-class config (#1833). available_school_classes is the pick
  // list the public form offers from grade 2; require_school_class makes
  // choosing mandatory. Only meaningful when enrollment.collect_school_class
  // is on.
  available_school_classes?: string[];
  require_school_class?: boolean;
  // Eligibility config (#1663). audience gates who may apply;
  // eligible_school_classes restricts submissions to children declaring one
  // of these classes ([] = no restriction). Omitted on legacy phases.
  audience?: PhaseAudience;
  eligible_school_classes?: string[];
  created_at: string;
  updated_at: string;
}

/**
 * Blast radius of deleting a phase, fetched for the confirmation modal.
 * `requests` and `care_offerings` are permanently deleted; `students_kept`
 * survive (their enrollment back-link is cleared, the student records stay).
 */
export interface PhaseDeleteImpact {
  requests: number;
  care_offerings: number;
  students_kept: number;
}

export interface PhaseInput {
  name: string;
  kind: PhaseKind;
  service_start_date: string;
  service_end_date: string;
  enrollment_open_at?: string | null;
  enrollment_close_at?: string | null;
  form_schema_id?: string | null;
  calendar_period_id?: string | null;
  show_status_reason_to_parent: boolean;
  care_overflow_mode: PhaseCareOverflowMode;
  care_offering_selection_mode: PhaseCareOfferingSelectionMode;
  is_active: boolean;
  available_school_classes?: string[];
  require_school_class?: boolean;
  audience?: PhaseAudience;
  eligible_school_classes?: string[];
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
  message?: string;
  error?: string;
  code?: string;
}

const BASE = "/api/enrollment/phases";

async function readJSON<T>(response: Response): Promise<T> {
  const raw = (await response.json()) as BackendEnvelope<T>;
  if (
    raw &&
    typeof raw === "object" &&
    "data" in raw &&
    raw.data !== undefined
  ) {
    return raw.data as T;
  }
  return raw as unknown as T;
}

async function readError(response: Response, fallback: string): Promise<Error> {
  return readEnrollmentError(
    response,
    fallback,
    logger,
    "phase_request_failed",
  );
}

/**
 * Maps a Phase back to the full PUT body. The backend rebuilds the whole
 * phase from the request, so partial updates would blank fields — every
 * caller that wants to change one field must send the complete input.
 */
export function phaseToInput(p: Phase): PhaseInput {
  return {
    name: p.name,
    kind: p.kind,
    service_start_date: p.service_start_date,
    service_end_date: p.service_end_date,
    enrollment_open_at: p.enrollment_open_at ?? null,
    enrollment_close_at: p.enrollment_close_at ?? null,
    form_schema_id: p.form_schema_id ?? null,
    calendar_period_id: p.calendar_period_id ?? null,
    show_status_reason_to_parent: p.show_status_reason_to_parent,
    care_overflow_mode: p.care_overflow_mode,
    care_offering_selection_mode: p.care_offering_selection_mode ?? "optional",
    is_active: p.is_active,
    available_school_classes: p.available_school_classes ?? [],
    require_school_class: p.require_school_class ?? false,
    // Default legacy phases (no stored audience) to "open" so a round-trip
    // save never silently changes their eligibility.
    audience: p.audience ?? "open",
    eligible_school_classes: p.eligible_school_classes ?? [],
  };
}

/**
 * Links or unlinks a phase to a calendar period (periodId null = unlink).
 * Refetches the phase first and builds the PUT body from that fresh state:
 * the endpoint replaces the whole row, so writing from a possibly stale
 * list snapshot would silently revert concurrent edits to the phase.
 */
export async function setPhaseCalendarPeriod(
  phase: Phase,
  periodId: string | null,
): Promise<Phase> {
  const fresh = await getPhase(phase.id);
  return updatePhase(phase.id, {
    ...phaseToInput(fresh),
    calendar_period_id: periodId,
  });
}

async function getPhase(id: string): Promise<Phase> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    cache: "no-store",
  });
  if (!response.ok) {
    throw await readError(response, "Phase konnte nicht geladen werden");
  }
  return readJSON<Phase>(response);
}

export async function listPhases(): Promise<Phase[]> {
  const response = await fetch(BASE, { cache: "no-store" });
  if (!response.ok) {
    throw await readError(response, "Phasen konnten nicht geladen werden");
  }
  const list = await readJSON<Phase[]>(response);
  return Array.isArray(list) ? list : [];
}

export async function createPhase(input: PhaseInput): Promise<Phase> {
  const response = await fetch(BASE, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw await readError(response, "Phase konnte nicht erstellt werden");
  }
  return readJSON<Phase>(response);
}

export async function updatePhase(
  id: string,
  input: PhaseInput,
): Promise<Phase> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw await readError(response, "Phase konnte nicht gespeichert werden");
  }
  return readJSON<Phase>(response);
}

/**
 * Fetches the blast radius of deleting a phase so the confirmation modal
 * can warn the admin what will be removed vs kept.
 */
export async function getPhaseDeleteImpact(
  id: string,
): Promise<PhaseDeleteImpact> {
  const response = await fetch(
    `${BASE}/${encodeURIComponent(id)}/delete-impact`,
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Löschvorschau konnte nicht geladen werden",
    );
  }
  return readJSON<PhaseDeleteImpact>(response);
}

/**
 * Permanently deletes a phase and all of its enrollment records
 * (submissions, per-child decisions, care offerings). Students created
 * from the phase are preserved by the backend. Always allowed — there is
 * no "has enrollments" guard anymore.
 */
export async function deletePhase(id: string): Promise<void> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (response.status === 204) return;
  if (!response.ok) {
    throw await readError(response, "Phase konnte nicht gelöscht werden");
  }
}

// --- Rollover (annual phase renewal) ---

export type RolloverMode = "opt_in" | "opt_out";

export interface RolloverInput {
  name: string;
  kind: PhaseKind;
  service_start_date: string;
  service_end_date: string;
  enrollment_open_at?: string | null;
  enrollment_close_at?: string | null;
  form_schema_id?: string | null;
  rollover_mode: RolloverMode;
  rollover_auto_approve: boolean;
  rollover_deadline: string; // RFC3339
  rollover_bumps_grade?: boolean;
}

interface RolloverSummary {
  source_child_count: number;
  rolled_count: number;
  review_count: number;
  review_by_reason: Record<string, number>;
  request_count: number;
  enqueued_emails: number;
  skipped_empty_email: number;
}

export interface RolloverResult {
  phase: Phase;
  summary: RolloverSummary;
}

/**
 * Creates a new phase from an existing source phase, carrying forward
 * every approved enrollment. Returns the new phase plus a summary.
 */
export async function createRollover(
  sourcePhaseID: string,
  input: RolloverInput,
): Promise<RolloverResult> {
  const response = await fetch(
    `${BASE}/${encodeURIComponent(sourcePhaseID)}/rollover`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
  if (!response.ok) {
    throw await readError(response, "Rollover konnte nicht erstellt werden");
  }
  return readJSON<RolloverResult>(response);
}

export interface ReviewQueueItem {
  child_id: string;
  request_id: string;
  first_name: string;
  last_name: string;
  target_grade_level?: number | null;
  review_reason?: string | null;
  source_grade_level?: number | null;
  source_first_name?: string;
  source_last_name?: string;
  guardian_first_name: string;
  guardian_last_name: string;
  guardian_email: string;
  status_token: string;
}

export async function listRolloverReview(
  phaseID: string,
): Promise<ReviewQueueItem[]> {
  const response = await fetch(
    `${BASE}/${encodeURIComponent(phaseID)}/review`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw await readError(response, "Prüfliste konnte nicht geladen werden");
  }
  const list = await readJSON<ReviewQueueItem[]>(response);
  return Array.isArray(list) ? list : [];
}

type ReviewDecision = "keep" | "drop" | "defer";

export interface ReviewDecisionInput {
  decision: ReviewDecision;
  new_grade_level?: number | null;
}

export async function decideRolloverReview(
  requestChildID: string,
  input: ReviewDecisionInput,
): Promise<void> {
  const response = await fetch(
    `/api/enrollment/admin/request-children/${encodeURIComponent(requestChildID)}/rollover-review`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(input),
    },
  );
  if (!response.ok) {
    throw await readError(
      response,
      "Entscheidung konnte nicht gespeichert werden",
    );
  }
}
