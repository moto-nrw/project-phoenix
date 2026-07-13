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
  /** The shift does not take place (staff absent / gap left open, #1841). */
  cancelled?: boolean;
  /** Optional "why" for a flexible daily change (#1841). Sent verbatim, so the
   *  saved value always matches what the admin sees; "" clears a stored reason. */
  changeReason?: string;
  /** Id of the shift this one covers as a replacement (#1841). Create-only. */
  originShiftId?: string | null;
}

interface SeriesPayload {
  staffId: string;
  /** ISO weekdays, 1 = Montag … 7 = Sonntag */
  weekdays: number[];
  /** "HH:MM" */
  startTime: string;
  /** "HH:MM" */
  endTime: string;
  breakMinutes: number;
  shiftTypeId: string | null;
  calendarPeriodId: string;
  /** 0 = jede Woche, 1 = Woche A, 2 = Woche B */
  weekPattern: number;
  /** "YYYY-MM-DD" */
  validFrom: string;
  /** "YYYY-MM-DD", exclusive; null = bis Periodenende */
  validUntil: string | null;
}

/** Edited fields applied from the effective date on ("Ab jetzt dauerhaft").
 *  Rhythm fields stay with the series — the backend inherits them. */
interface SeriesSplitPayload {
  /** "YYYY-MM-DD" */
  effectiveDate: string;
  startTime: string;
  endTime: string;
  breakMinutes: number;
  shiftTypeId: string | null;
}

export interface SeriesResult {
  seriesId: string;
  created: number;
  /** Days skipped because an existing shift would overlap ("YYYY-MM-DD") */
  skippedDates: string[];
}

interface BackendSeriesResult {
  series_id: number;
  created: number;
  skipped_dates: string[] | null;
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
  // The flexible-change fields are only sent when they carry meaning, so a
  // plain create/edit stays byte-for-byte the classic Dienstplan body (#1841):
  //  - cancelled omitted → the backend treats it as false (no absence)
  //  - change_reason sent whenever supplied (incl. ""), so the stored reason
  //    mirrors the field; omitted leaves the stored reason untouched on update
  //  - origin_shift_id sent only for a replacement shift
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
    ...(payload.cancelled ? { cancelled: true } : {}),
    ...(payload.changeReason !== undefined
      ? { change_reason: payload.changeReason }
      : {}),
    ...(payload.originShiftId != null
      ? { origin_shift_id: Number.parseInt(payload.originShiftId, 10) }
      : {}),
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

async function readSeriesResult(response: Response): Promise<SeriesResult> {
  if (!response.ok) {
    throw await readShiftError(
      response,
      "Schicht-Serie konnte nicht gespeichert werden",
    );
  }
  const json = (await response.json()) as { data: BackendSeriesResult };
  return {
    seriesId: json.data.series_id.toString(),
    created: json.data.created,
    skippedDates: json.data.skipped_dates ?? [],
  };
}

class StaffShiftSeriesService {
  async createSeries(payload: SeriesPayload): Promise<SeriesResult> {
    const response = await sessionFetch("/api/staff/shifts/series", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        staff_id: Number.parseInt(payload.staffId, 10),
        weekdays: payload.weekdays,
        start_time: payload.startTime,
        end_time: payload.endTime,
        break_minutes: payload.breakMinutes,
        shift_type_id:
          payload.shiftTypeId != null
            ? Number.parseInt(payload.shiftTypeId, 10)
            : null,
        calendar_period_id: Number.parseInt(payload.calendarPeriodId, 10),
        week_pattern: payload.weekPattern,
        valid_from: payload.validFrom,
        valid_until: payload.validUntil,
      }),
    });
    return readSeriesResult(response);
  }

  async splitSeries(
    seriesId: string,
    payload: SeriesSplitPayload,
  ): Promise<SeriesResult> {
    const response = await sessionFetch(
      `/api/staff/shifts/series/${seriesId}/split`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          effective_date: payload.effectiveDate,
          start_time: payload.startTime,
          end_time: payload.endTime,
          break_minutes: payload.breakMinutes,
          shift_type_id:
            payload.shiftTypeId != null
              ? Number.parseInt(payload.shiftTypeId, 10)
              : null,
        }),
      },
    );
    return readSeriesResult(response);
  }

  async endSeries(seriesId: string, from: string): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/shifts/series/${seriesId}?from=${from}`,
      { method: "DELETE" },
    );
    if (!response.ok) {
      throw await readShiftError(
        response,
        "Schicht-Serie konnte nicht beendet werden",
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
export const staffShiftSeriesService = new StaffShiftSeriesService();
export const ownShiftService = new OwnShiftService();
