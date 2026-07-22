// Closing day API client (#1418 3b). Talks to the Next.js proxy at
// /api/timetable/closing-days, which forwards to the Go backend
// /api/timetable/closing-days (CRUD via SchedulesRead/Create/Update/Delete).

import { createLogger } from "~/lib/logger";
import {
  type BackendClosingDay,
  type ClosingDay,
  type ClosingDayInput,
  mapClosingDay,
  mapClosingDays,
} from "./closing-day-helpers";

const logger = createLogger({ component: "ClosingDayApi" });

interface ApiEnvelope<T> {
  status?: string;
  message?: string;
  data: T;
}

class ClosingDayApiError extends Error {
  readonly httpStatus: number;

  constructor(message: string, httpStatus: number) {
    super(message);
    this.name = "ClosingDayApiError";
    this.httpStatus = httpStatus;
  }
}

async function unwrap<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `Anfrage fehlgeschlagen (HTTP ${response.status})`;
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Body wasn't JSON — keep the generic message.
    }
    throw new ClosingDayApiError(message, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const envelope = (await response.json()) as ApiEnvelope<T>;
  return envelope.data;
}

class ClosingDayService {
  async list(): Promise<ClosingDay[]> {
    const response = await fetch("/api/timetable/closing-days", {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
    const raw = await unwrap<BackendClosingDay[]>(response);
    return mapClosingDays(raw ?? []);
  }

  async create(body: ClosingDayInput): Promise<ClosingDay> {
    const response = await fetch("/api/timetable/closing-days", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendClosingDay>(response);
    logger.info("closing_day_created", { closing_day_id: raw.id });
    return mapClosingDay(raw);
  }

  async update(id: string, body: ClosingDayInput): Promise<ClosingDay> {
    const response = await fetch(`/api/timetable/closing-days/${id}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendClosingDay>(response);
    logger.info("closing_day_updated", { closing_day_id: raw.id });
    return mapClosingDay(raw);
  }

  async delete(id: string): Promise<void> {
    const response = await fetch(`/api/timetable/closing-days/${id}`, {
      method: "DELETE",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
    await unwrap<void>(response);
    logger.info("closing_day_deleted", { closing_day_id: id });
  }
}

export const closingDayService = new ClosingDayService();
