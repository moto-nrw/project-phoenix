import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentPhaseAPI" });

export type PhaseKind = "school_year" | "holiday" | "custom";
export type PhaseCareOverflowMode = "waitlist" | "reject" | "allow";

export interface Phase {
  id: string;
  name: string;
  kind: PhaseKind;
  service_start_date: string; // YYYY-MM-DD
  service_end_date: string; // YYYY-MM-DD
  enrollment_open_at?: string | null; // RFC3339
  enrollment_close_at?: string | null; // RFC3339
  form_schema_id?: string | null;
  show_status_reason_to_parent: boolean;
  care_overflow_mode: PhaseCareOverflowMode;
  is_active: boolean;
  // Rollover columns — populated only on phases created by the
  // "verlängern" flow. NULL/false on fresh phases.
  rollover_source_phase_id?: string | null;
  rollover_mode?: RolloverMode | null;
  rollover_auto_approve?: boolean;
  rollover_deadline?: string | null;
  rollover_bumps_grade?: boolean;
  created_at: string;
  updated_at: string;
}

export interface PhaseInput {
  name: string;
  kind: PhaseKind;
  service_start_date: string;
  service_end_date: string;
  enrollment_open_at?: string | null;
  enrollment_close_at?: string | null;
  form_schema_id?: string | null;
  show_status_reason_to_parent: boolean;
  care_overflow_mode: PhaseCareOverflowMode;
  is_active: boolean;
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
  message?: string;
  error?: string;
  code?: string;
}

const BASE = "/api/enrollment/phases";

// Backend → German UI message map. Keep in sync with the ErrCodeRollover*
// constants in backend/api/enrollment/rollover_handlers.go. Falls back to
// the backend's raw `error` text when the code isn't known so we never
// hide information from the admin.
const ROLLOVER_ERROR_MESSAGES: Record<string, string> = {
  "rollover.source_not_found": "Die Quellphase wurde nicht gefunden.",
  "rollover.invalid_request":
    "Die Eingaben sind unvollständig oder ungültig. Bitte alle Pflichtfelder prüfen.",
  "rollover.review_invalid": "Die Aktion ist in diesem Zustand nicht erlaubt.",
  "rollover.review_not_found":
    "Dieser Eintrag in der Prüfliste existiert nicht mehr.",
  "rollover.duplicate_name":
    "Es existiert bereits eine Phase mit diesem Namen. Bitte einen anderen Namen wählen.",
  "rollover.source_already_rolled":
    "Aus dieser Phase wurde bereits eine Anschlussphase erstellt. Bitte zuerst die bestehende Anschlussphase löschen, bevor du eine neue anlegst.",
};

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

// Substring-keyed map for raw validation errors returned by the
// phase model's Validate() method (backend/models/enrollment/phase.go).
// These come back without a code, so we match on the English text.
const PHASE_VALIDATION_MESSAGES: Array<[string, string]> = [
  [
    "service_end_date must be on or after service_start_date",
    "Das Ende des Betreuungszeitraums muss am gleichen Tag oder nach dem Beginn liegen.",
  ],
  [
    "enrollment_close_at must be after enrollment_open_at",
    "Die Schließung des Anmeldefensters muss nach der Öffnung liegen.",
  ],
];

function translatePhaseError(raw: string | undefined): string | undefined {
  if (!raw) return undefined;
  for (const [needle, german] of PHASE_VALIDATION_MESSAGES) {
    if (raw.includes(needle)) return german;
  }
  return undefined;
}

async function readError(response: Response, fallback: string): Promise<Error> {
  let message = fallback;
  let code: string | undefined;
  try {
    const payload = (await response.json()) as BackendEnvelope<unknown>;
    code = payload.code;
    const localized = code ? ROLLOVER_ERROR_MESSAGES[code] : undefined;
    const validationLocalized = translatePhaseError(payload.error);
    message =
      localized ??
      validationLocalized ??
      payload.error ??
      payload.message ??
      `${fallback} (HTTP ${response.status})`;
  } catch {
    /* ignore */
  }
  // 4xx are expected user-input failures; only 5xx is a real error.
  if (response.status >= 500) {
    logger.error("phase_request_failed", {
      status: response.status,
      message,
      code,
    });
  } else {
    logger.warn("phase_request_rejected", {
      status: response.status,
      message,
      code,
    });
  }
  const err = new Error(message) as Error & { status?: number; code?: string };
  err.status = response.status;
  err.code = code;
  return err;
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
 * Deletes a phase. Returns 409 from the backend when the phase still
 * has care offerings or submissions referencing it — caller should
 * surface a "deactivate instead" hint.
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

export interface RolloverSummary {
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

export type ReviewDecision = "keep" | "drop" | "defer";

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
