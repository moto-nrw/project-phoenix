import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareOfferingAPI" });

export type DaysOfWeekMode = "fixed" | "parent_choice";

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
  sort_order: number;
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
  sort_order: number;
}

interface BackendEnvelope<T> {
  status?: string;
  data?: T;
  message?: string;
  error?: string;
}

const BASE = "/api/enrollment/care-offerings";

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
    // ignore parse errors
  }
  logger.error("care_offering_request_failed", {
    status: response.status,
    message,
  });
  return new Error(message);
}

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
  const response = await fetch(path, { cache: "no-store" });
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungsangebote konnten nicht geladen werden",
    );
  }
  const list = await readJSON<CareOffering[]>(response);
  return Array.isArray(list) ? list : [];
}

export async function createCareOffering(
  input: CareOfferingInput,
): Promise<CareOffering> {
  const response = await fetch(BASE, {
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
  return readJSON<CareOffering>(response);
}

export async function updateCareOffering(
  id: string,
  input: CareOfferingInput,
): Promise<CareOffering> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw await readError(
      response,
      "Betreuungsangebot konnte nicht gespeichert werden",
    );
  }
  return readJSON<CareOffering>(response);
}

export async function deleteCareOffering(id: string): Promise<void> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
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
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}/clone`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw await readError(response, "Klonen fehlgeschlagen");
  }
  return readJSON<CareOffering>(response);
}
