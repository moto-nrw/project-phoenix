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
  let message = fallback;
  try {
    const payload = (await response.json()) as BackendEnvelope<unknown>;
    message =
      payload.error ??
      payload.message ??
      `${fallback} (HTTP ${response.status})`;
  } catch {
    /* ignore */
  }
  logger.error("phase_request_failed", {
    status: response.status,
    message,
  });
  const err = new Error(message) as Error & { status?: number };
  err.status = response.status;
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
