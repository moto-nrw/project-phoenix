import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareOfferingAPI" });

export type DaysOfWeekMode = "fixed" | "parent_choice";

export interface CareOffering {
  id: string;
  calendar_period_id?: string | null;
  activity_group_id?: string | null;
  name: string;
  description?: string | null;
  days_of_week_mode: DaysOfWeekMode;
  available_days: string[];
  includes_holiday_care: boolean;
  includes_lunch: boolean;
  capacity?: number | null;
  price_cents?: number | null;
  application_window_start?: string | null;
  application_window_end?: string | null;
  service_start_date?: string | null;
  service_end_date?: string | null;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface CareOfferingInput {
  calendar_period_id?: number | null;
  activity_group_id?: number | null;
  name: string;
  description?: string | null;
  days_of_week_mode: DaysOfWeekMode;
  available_days: string[];
  includes_holiday_care: boolean;
  includes_lunch: boolean;
  capacity?: number | null;
  price_cents?: number | null;
  application_window_start?: string | null;
  application_window_end?: string | null;
  service_start_date?: string | null;
  service_end_date?: string | null;
  is_active: boolean;
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
  calendarPeriodId?: string | null,
): Promise<CareOffering[]> {
  const url = new URL(
    BASE,
    globalThis.window?.location.origin ?? "http://localhost",
  );
  if (calendarPeriodId) {
    url.searchParams.set("calendar_period_id", calendarPeriodId);
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
  target_calendar_period_id: number;
  days_offset: number;
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

export interface CalendarPeriodSummary {
  id: string;
  name: string;
  period_type: "school_year" | "semester" | "holiday" | "custom";
  start_date: string;
  end_date: string;
  is_active: boolean;
}

interface BackendCalendarPeriod {
  id: number;
  name: string;
  period_type: "school_year" | "semester" | "holiday" | "custom";
  start_date: string;
  end_date: string;
  is_active: boolean;
}

const CALENDAR_PERIODS_PATH = "/api/timetable/periods";

export async function listCalendarPeriods(): Promise<CalendarPeriodSummary[]> {
  const response = await fetch(CALENDAR_PERIODS_PATH, { cache: "no-store" });
  if (!response.ok) {
    throw await readError(response, "Schuljahre konnten nicht geladen werden");
  }
  const raw = (await response.json()) as BackendEnvelope<
    BackendCalendarPeriod[]
  >;
  const list = raw.data ?? (raw as unknown as BackendCalendarPeriod[]) ?? [];
  return list.map((p) => ({
    id: p.id.toString(),
    name: p.name,
    period_type: p.period_type,
    start_date: p.start_date,
    end_date: p.end_date,
    is_active: p.is_active,
  }));
}

export interface CreateCalendarPeriodInput {
  name: string;
  period_type: "school_year" | "semester" | "holiday" | "custom";
  start_date: string;
  end_date: string;
  week_cycle_length?: number;
}

export async function createCalendarPeriod(
  input: CreateCalendarPeriodInput,
): Promise<CalendarPeriodSummary> {
  const payload = {
    name: input.name,
    period_type: input.period_type,
    start_date: input.start_date,
    end_date: input.end_date,
    week_cycle_length: input.week_cycle_length ?? 1,
  };
  const response = await fetch(CALENDAR_PERIODS_PATH, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw await readError(response, "Schuljahr konnte nicht angelegt werden");
  }
  const raw = (await response.json()) as BackendEnvelope<BackendCalendarPeriod>;
  const created = raw.data ?? (raw as unknown as BackendCalendarPeriod);
  return {
    id: created.id.toString(),
    name: created.name,
    period_type: created.period_type,
    start_date: created.start_date,
    end_date: created.end_date,
    is_active: created.is_active,
  };
}
