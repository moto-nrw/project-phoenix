import { apiErrorFromBody, transportFetch } from "./api-error";
// Calendar period API client. Talks to the Next.js proxy at
// /api/timetable/periods, which forwards to the Go backend
// /api/timetable/periods (CRUD via SchedulesRead/Create/Update/Delete).

import { createLogger } from "~/lib/logger";
import {
  type BackendCalendarPeriod,
  type CalendarPeriod,
  type CalendarPeriodInput,
  type CalendarPeriodSaveResult,
  mapPeriod,
  mapPeriods,
  mapPeriodWithWarnings,
} from "./calendar-period-helpers";

const logger = createLogger({ component: "CalendarPeriodApi" });

interface ApiEnvelope<T> {
  status?: string;
  message?: string;
  data: T;
}

async function unwrap<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw apiErrorFromBody(
      `Anfrage fehlgeschlagen (HTTP ${response.status})`,
      response.status,
      await response.json().catch(() => undefined),
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const envelope = (await response.json()) as ApiEnvelope<T>;
  return envelope.data;
}

class CalendarPeriodService {
  async list(): Promise<CalendarPeriod[]> {
    const response = await transportFetch("/api/timetable/periods", {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
    const raw = await unwrap<BackendCalendarPeriod[]>(response);
    return mapPeriods(raw ?? []);
  }

  async get(id: string): Promise<CalendarPeriod> {
    const response = await transportFetch(`/api/timetable/periods/${id}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
    const raw = await unwrap<BackendCalendarPeriod>(response);
    return mapPeriod(raw);
  }

  /**
   * POST /api/timetable/periods/bootstrap — idempotent default-period
   * setup. Returns the tenant's periods (creating a default school-year
   * period first when none exists) plus whether a period was created by
   * this call.
   */
  async bootstrap(): Promise<{ periods: CalendarPeriod[]; created: boolean }> {
    const response = await transportFetch("/api/timetable/periods/bootstrap", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify({}),
    });
    const raw = await unwrap<{
      periods: BackendCalendarPeriod[];
      created: boolean;
    }>(response);
    logger.info("periods_bootstrapped", {
      period_count: (raw.periods ?? []).length,
      created: raw.created,
    });
    return {
      periods: mapPeriods(raw.periods ?? []),
      created: raw.created,
    };
  }

  async create(body: CalendarPeriodInput): Promise<CalendarPeriodSaveResult> {
    const response = await transportFetch("/api/timetable/periods", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendCalendarPeriod>(response);
    logger.info("period_created", {
      period_id: raw.id,
      period_type: raw.period_type,
      warning_count: (raw.warnings ?? []).length,
    });
    return mapPeriodWithWarnings(raw);
  }

  async update(
    id: string,
    body: CalendarPeriodInput,
  ): Promise<CalendarPeriodSaveResult> {
    const response = await transportFetch(`/api/timetable/periods/${id}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendCalendarPeriod>(response);
    logger.info("period_updated", {
      period_id: raw.id,
      warning_count: (raw.warnings ?? []).length,
    });
    return mapPeriodWithWarnings(raw);
  }

  async delete(id: string): Promise<void> {
    const response = await transportFetch(`/api/timetable/periods/${id}`, {
      method: "DELETE",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
    if (!response.ok && response.status !== 204) {
      throw apiErrorFromBody(
        `Anfrage fehlgeschlagen (HTTP ${response.status})`,
        response.status,
        await response.json().catch(() => undefined),
      );
    }
    logger.info("period_deleted", { period_id: id });
  }
}

export const calendarPeriodService = new CalendarPeriodService();
