// API clients for planned staff shifts (Dienstplan, #1376 core slice).
// Admin CRUD goes through /api/staff/shifts (backend /api/staff-shifts,
// time_tracking:manage); staff read their own shifts via
// /api/time-tracking/shifts (time_tracking:own).

import { sessionFetch } from "./session-cache";
import {
  mapStaffShift,
  type BackendStaffShift,
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
}

function toBackendBody(payload: ShiftPayload) {
  return {
    staff_id: Number.parseInt(payload.staffId, 10),
    date: payload.date,
    start_time: payload.startTime,
    end_time: payload.endTime,
    break_minutes: payload.breakMinutes,
  };
}

async function readShiftList(response: Response): Promise<StaffShift[]> {
  if (!response.ok) {
    throw new Error(`Failed to load shifts (${response.status})`);
  }
  const json = (await response.json()) as { data: BackendStaffShift[] | null };
  return (json.data ?? []).map(mapStaffShift);
}

async function readShift(response: Response): Promise<StaffShift> {
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || `Failed to save shift (${response.status})`);
  }
  const json = (await response.json()) as { data: BackendStaffShift };
  return mapStaffShift(json.data);
}

class StaffShiftService {
  async getShifts(from: string, to: string): Promise<StaffShift[]> {
    const response = await sessionFetch(
      `/api/staff/shifts?from=${from}&to=${to}`,
    );
    return readShiftList(response);
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
      throw new Error(`Failed to delete shift (${response.status})`);
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
