// API clients for planned staff shifts (Dienstplan, #1376 core slice).
// Admin CRUD goes through /api/staff/shifts (backend /api/staff-shifts,
// time_tracking:manage); staff read their own shifts via
// /api/time-tracking/shifts (time_tracking:own).

import { sessionFetch } from "./session-cache";
import {
  mapStaffScheduleOverview,
  mapStaffShift,
  type BackendStaffScheduleOverview,
  type BackendStaffShift,
  type StaffScheduleOverview,
  type StaffShift,
} from "./shift-helpers";

interface ShiftPayload {
  staffId: string;
  /** "YYYY-MM-DD" */
  date: string;
  /** "HH:MM" */
  startTime: string;
  /** "HH:MM" */
  endTime: string;
  breakMinutes: number;
  /** Id of the linked shift type (Schichtart), or null if untyped */
  shiftTypeId: string | null;
}

export class ShiftApiError extends Error {
  readonly status: number;
  readonly detail: string;

  constructor(status: number, detail: string) {
    super(`HTTP ${status}: ${detail}`);
    this.name = "ShiftApiError";
    this.status = status;
    this.detail = detail;
  }
}

function toBackendBody(payload: ShiftPayload) {
  return {
    staff_id: Number.parseInt(payload.staffId, 10),
    date: payload.date,
    start_time: payload.startTime,
    end_time: payload.endTime,
    break_minutes: payload.breakMinutes,
    shift_type_id:
      payload.shiftTypeId != null
        ? Number.parseInt(payload.shiftTypeId, 10)
        : null,
  };
}

async function readShiftError(
  response: Response,
  fallback: string,
): Promise<ShiftApiError> {
  let detail = "";
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    try {
      const body = (await response.json()) as { error?: string };
      detail = body.error ?? "";
    } catch {
      detail = "";
    }
  } else {
    detail = await response.text();
  }
  return new ShiftApiError(response.status, detail || fallback);
}

async function readShiftList(response: Response): Promise<StaffShift[]> {
  if (!response.ok) {
    throw await readShiftError(
      response,
      "Schichten konnten nicht geladen werden",
    );
  }
  const json = (await response.json()) as { data: BackendStaffShift[] | null };
  return (json.data ?? []).map(mapStaffShift);
}

async function readShift(response: Response): Promise<StaffShift> {
  if (!response.ok) {
    throw await readShiftError(
      response,
      "Schicht konnte nicht gespeichert werden",
    );
  }
  const json = (await response.json()) as { data: BackendStaffShift };
  return mapStaffShift(json.data);
}

async function readOverview(
  response: Response,
): Promise<StaffScheduleOverview> {
  if (!response.ok) {
    throw await readShiftError(
      response,
      "Dienstplan konnte nicht geladen werden",
    );
  }
  const json = (await response.json()) as {
    data: BackendStaffScheduleOverview;
  };
  return mapStaffScheduleOverview(json.data);
}

class StaffShiftService {
  async getShifts(from: string, to: string): Promise<StaffShift[]> {
    const response = await sessionFetch(
      `/api/staff/shifts?from=${from}&to=${to}`,
    );
    return readShiftList(response);
  }

  async getOverview(from: string, to: string): Promise<StaffScheduleOverview> {
    const response = await sessionFetch(
      `/api/staff/shifts/overview?from=${from}&to=${to}`,
    );
    return readOverview(response);
  }

  async createShift(payload: ShiftPayload): Promise<StaffShift> {
    const response = await sessionFetch("/api/staff/shifts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(toBackendBody(payload)),
    });
    return readShift(response);
  }

  async updateShift(
    shiftId: string,
    payload: ShiftPayload,
  ): Promise<StaffShift> {
    const response = await sessionFetch(`/api/staff/shifts/${shiftId}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(toBackendBody(payload)),
    });
    return readShift(response);
  }

  async deleteShift(shiftId: string): Promise<void> {
    const response = await sessionFetch(`/api/staff/shifts/${shiftId}`, {
      method: "DELETE",
    });
    if (!response.ok) {
      throw await readShiftError(
        response,
        "Schicht konnte nicht gelöscht werden",
      );
    }
  }
}

class OwnShiftService {
  async getOwnShifts(from: string, to: string): Promise<StaffShift[]> {
    const response = await sessionFetch(
      `/api/time-tracking/shifts?from=${from}&to=${to}`,
    );
    return readShiftList(response);
  }
}

export const staffShiftService = new StaffShiftService();
export const ownShiftService = new OwnShiftService();
