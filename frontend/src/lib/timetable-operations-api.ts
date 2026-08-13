import type {
  AttendancePatchBody,
  BackendStartOperationResult,
  BackendTimetableRoster,
  PlannedTimetableInstance,
  StartOperationResult,
  TimetableRoster,
} from "./timetable-operations-types";
import type { CreateInstanceBody } from "./timetable-types";
import {
  mapPlannedInstance,
  mapRoster,
  mapStartOperation,
} from "./timetable-operations-types";

interface ApiEnvelope<T> {
  data: T;
}

interface PlannedNowOptions {
  date?: string;
  horizonMinutes?: number;
  limit?: number;
  includeRoster?: boolean;
}

export class TimetableOperationsApiError extends Error {
  readonly httpStatus: number;
  readonly code?: string;

  constructor(message: string, httpStatus: number, code?: string) {
    super(message);
    this.name = "TimetableOperationsApiError";
    this.httpStatus = httpStatus;
    this.code = code;
  }
}

async function unwrap<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `Anfrage fehlgeschlagen (HTTP ${response.status})`;
    let code: string | undefined;
    try {
      const body = (await response.json()) as { error?: string; code?: string };
      if (body.error) message = body.error;
      code = body.code;
    } catch {
      // Keep generic error.
    }
    throw new TimetableOperationsApiError(message, response.status, code);
  }
  const envelope = (await response.json()) as ApiEnvelope<T>;
  return envelope.data;
}

export const timetableOperationsApi = {
  async plannedNow(
    dateOrOptions?: string | PlannedNowOptions,
  ): Promise<PlannedTimetableInstance[]> {
    const options =
      typeof dateOrOptions === "string"
        ? { date: dateOrOptions }
        : dateOrOptions;
    const params = new URLSearchParams();
    if (options?.date) params.set("date", options.date);
    if (options?.horizonMinutes !== undefined) {
      params.set("horizon_minutes", options.horizonMinutes.toString());
    }
    if (options?.limit !== undefined) {
      params.set("limit", options.limit.toString());
    }
    if (options?.includeRoster !== undefined) {
      params.set("include_roster", String(options.includeRoster));
    }
    const suffix = params.size > 0 ? `?${params.toString()}` : "";
    const raw = await unwrap<{
      instances: Parameters<typeof mapPlannedInstance>[0][];
    }>(
      await fetch(`/api/timetable/operations/planned-now${suffix}`, {
        credentials: "include",
        headers: { Accept: "application/json" },
      }),
    );
    return raw.instances.map(mapPlannedInstance);
  },

  async start(instanceId: string): Promise<StartOperationResult> {
    const raw = await unwrap<BackendStartOperationResult>(
      await fetch(`/api/timetable/operations/instances/${instanceId}/start`, {
        method: "POST",
        credentials: "include",
        headers: { Accept: "application/json" },
      }),
    );
    return mapStartOperation(raw);
  },

  async createAndStartSpontaneous(
    body: CreateInstanceBody,
  ): Promise<StartOperationResult> {
    const raw = await unwrap<BackendStartOperationResult>(
      await fetch("/api/timetable/operations/spontaneous/start", {
        method: "POST",
        credentials: "include",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
      }),
    );
    return mapStartOperation(raw);
  },

  async roster(instanceId: string): Promise<TimetableRoster> {
    const raw = await unwrap<BackendTimetableRoster>(
      await fetch(`/api/timetable/operations/instances/${instanceId}/roster`, {
        credentials: "include",
        headers: { Accept: "application/json" },
      }),
    );
    return mapRoster(raw);
  },

  async rosterByActiveGroup(activeGroupId: string): Promise<TimetableRoster> {
    const raw = await unwrap<BackendTimetableRoster>(
      await fetch(
        `/api/timetable/operations/active-groups/${activeGroupId}/roster`,
        {
          credentials: "include",
          headers: { Accept: "application/json" },
        },
      ),
    );
    return mapRoster(raw);
  },

  async checkIn(
    instanceId: string,
    studentId: string,
  ): Promise<TimetableRoster> {
    const raw = await unwrap<BackendTimetableRoster>(
      await fetch(
        `/api/timetable/operations/instances/${instanceId}/students/${studentId}/check-in`,
        {
          method: "POST",
          credentials: "include",
          headers: { Accept: "application/json" },
        },
      ),
    );
    return mapRoster(raw);
  },

  async checkOut(
    instanceId: string,
    studentId: string,
  ): Promise<TimetableRoster> {
    const raw = await unwrap<BackendTimetableRoster>(
      await fetch(
        `/api/timetable/operations/instances/${instanceId}/students/${studentId}/check-out`,
        {
          method: "POST",
          credentials: "include",
          headers: { Accept: "application/json" },
        },
      ),
    );
    return mapRoster(raw);
  },

  async patchAttendance(
    instanceId: string,
    studentId: string,
    body: AttendancePatchBody,
  ): Promise<void> {
    await unwrap<unknown>(
      await fetch(
        `/api/timetable/operations/instances/${instanceId}/students/${studentId}/attendance`,
        {
          method: "PATCH",
          credentials: "include",
          headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
          },
          body: JSON.stringify(body),
        },
      ),
    );
  },

  async complete(
    instanceId: string,
    confirmedPresentStudentIds: string[],
  ): Promise<{ reopenUntil?: string }> {
    const raw = await unwrap<{ reopen_until?: string }>(
      await fetch(
        `/api/timetable/operations/instances/${instanceId}/complete`,
        {
          method: "POST",
          credentials: "include",
          headers: {
            Accept: "application/json",
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            confirmed_present_student_ids:
              confirmedPresentStudentIds.map(Number),
          }),
        },
      ),
    );
    return { reopenUntil: raw.reopen_until };
  },

  async reopen(instanceId: string): Promise<StartOperationResult> {
    const raw = await unwrap<BackendStartOperationResult>(
      await fetch(`/api/timetable/operations/instances/${instanceId}/reopen`, {
        method: "POST",
        credentials: "include",
        headers: { Accept: "application/json" },
      }),
    );
    return mapStartOperation(raw);
  },
};
