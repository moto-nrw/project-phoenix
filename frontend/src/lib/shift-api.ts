// API clients for planned staff shifts (Dienstplan, #1376 core slice).
// Admin CRUD goes through /api/staff/shifts (backend /api/staff-shifts,
// time_tracking:manage); staff read their own shifts via
// /api/time-tracking/shifts (time_tracking:own).

import { sessionFetch } from "./session-cache";
import {
  mapOwnAssignment,
  mapStaffScheduleOverview,
  mapStaffShift,
  type BackendOwnAssignment,
  type BackendStaffScheduleOverview,
  type BackendStaffShift,
  type OwnAssignment,
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
  /** Optional free-form shift note. Omitted updates preserve the stored note. */
  notes?: string;
  /** The shift does not take place (staff absent / gap left open, #1841). */
  cancelled?: boolean;
  /** Optional "why" for a flexible daily change (#1841). Sent verbatim, so the
   *  saved value always matches what the admin sees; "" or null clears it. */
  changeReason?: string | null;
  /** Id of the shift this one covers as a replacement (#1841). Create-only. */
  originShiftId?: string | null;
}

interface MoveShiftPayload {
  sourceStaffId: string;
  targetStaffId: string;
  /** "YYYY-MM-DD" */
  date: string;
  /** "HH:MM" */
  startTime: string;
  /** "HH:MM" */
  endTime: string;
  breakMinutes: number;
  shiftTypeId: string | null;
}

/** One replacement (Vertretung) covering part of a cancelled shift's gap. */
interface ReplacementPayload {
  staffId: string;
  /** "HH:MM" */
  startTime: string;
  /** "HH:MM" */
  endTime: string;
  breakMinutes: number;
  shiftTypeId: string | null;
}

/** Atomic cancel/reactivate + replacement-set change for one shift (#1841).
 *  The backend applies the origin flag flip and the whole replacement set in a
 *  single transaction, so a partial failure never leaves a half-changed plan. */
interface CancellationPayload {
  cancelled: boolean;
  /** Sent verbatim ("" clears the stored reason). */
  changeReason: string;
  /** The origin shift's own window/type as edited in the modal. Sent alongside
   *  the cancellation so a time/type change made in the same save is applied to
   *  the origin instead of being discarded, and a reactivation re-checks overlap
   *  against the edited window (#1841). "HH:MM". */
  startTime: string;
  endTime: string;
  breakMinutes: number;
  shiftTypeId: string | null;
  /** Only meaningful when cancelling; ignored (must be empty) on reactivation. */
  replacements: ReplacementPayload[];
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

/** Edited fields applied from the effective date on ("Ab jetzt dauerhaft" and
 *  the series editor). Omitted optional fields inherit the predecessor's
 *  value, so editing only the times never has to re-send the rhythm (#2028). */
interface SeriesSplitPayload {
  /** "YYYY-MM-DD" */
  effectiveDate: string;
  /** Concrete occurrence opened by the planner. If it is today, the backend
   * updates this row in place and only re-plans the series from tomorrow. */
  occurrenceShiftId?: string;
  startTime: string;
  endTime: string;
  breakMinutes: number;
  shiftTypeId: string | null;
  /** ISO weekdays; omitted keeps the days the series already runs on. */
  weekdays?: number[];
  /** 0 = jede Woche, 1 = Woche A, 2 = Woche B; omitted keeps the rhythm. */
  weekPattern?: number;
  /** "YYYY-MM-DD", exclusive. Omitted keeps the stored end, null runs the
   *  series to the end of the calendar period. */
  validUntil?: string | null;
}

/** The stored rule behind a series shift — weekdays, rhythm, window, and
 *  validity. The series editor loads it and writes changes back through
 *  splitSeries, so they apply from the opened occurrence onwards (#2028). */
export interface SeriesRule {
  id: string;
  staffId: string;
  /** ISO weekdays, 1 = Montag … 7 = Sonntag */
  weekdays: number[];
  startTime: string;
  endTime: string;
  breakMinutes: number;
  shiftTypeId: string | null;
  calendarPeriodId: string;
  weekPattern: number;
  /** "YYYY-MM-DD" */
  validFrom: string;
  /** "YYYY-MM-DD", exclusive; null = bis Periodenende */
  validUntil: string | null;
}

interface BackendSeriesRule {
  id: number;
  staff_id: number;
  weekdays: number[] | null;
  start_time: string;
  end_time: string;
  break_minutes: number;
  shift_type_id: number | null;
  calendar_period_id: number;
  week_pattern: number;
  valid_from: string;
  valid_until: string | null;
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

function mapSeriesRule(data: BackendSeriesRule): SeriesRule {
  return {
    id: data.id.toString(),
    staffId: data.staff_id.toString(),
    weekdays: data.weekdays ?? [],
    startTime: data.start_time,
    endTime: data.end_time,
    breakMinutes: data.break_minutes,
    shiftTypeId:
      data.shift_type_id != null ? data.shift_type_id.toString() : null,
    calendarPeriodId: data.calendar_period_id.toString(),
    weekPattern: data.week_pattern,
    validFrom: data.valid_from,
    validUntil: data.valid_until,
  };
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
    ...(payload.notes !== undefined ? { notes: payload.notes } : {}),
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

  /** One staff member's planned shifts in the range — backs the admin
   *  Plan|Ist|Grund detail view (#1844). Uses the backend staff_id filter. */
  async getShiftsForStaff(
    staffId: string,
    from: string,
    to: string,
  ): Promise<StaffShift[]> {
    const response = await sessionFetch(
      `/api/staff/shifts?staff_id=${staffId}&from=${from}&to=${to}`,
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

  /** Atomically move one concrete shift while preserving its row identity. */
  async moveShift(
    shiftId: string,
    payload: MoveShiftPayload,
  ): Promise<StaffShift> {
    const response = await sessionFetch(`/api/staff/shifts/${shiftId}/move`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        source_staff_id: Number.parseInt(payload.sourceStaffId, 10),
        target_staff_id: Number.parseInt(payload.targetStaffId, 10),
        date: payload.date,
        start_time: payload.startTime,
        end_time: payload.endTime,
        break_minutes: payload.breakMinutes,
        shift_type_id:
          payload.shiftTypeId != null
            ? Number.parseInt(payload.shiftTypeId, 10)
            : null,
      }),
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

  /** Atomically cancel/reactivate a shift and rebuild its replacement set. */
  async applyCancellation(
    shiftId: string,
    payload: CancellationPayload,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/shifts/${shiftId}/cancellation`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          cancelled: payload.cancelled,
          change_reason: payload.changeReason,
          start_time: payload.startTime,
          end_time: payload.endTime,
          break_minutes: payload.breakMinutes,
          shift_type_id:
            payload.shiftTypeId != null
              ? Number.parseInt(payload.shiftTypeId, 10)
              : null,
          replacements: payload.replacements.map((r) => ({
            staff_id: Number.parseInt(r.staffId, 10),
            start_time: r.startTime,
            end_time: r.endTime,
            break_minutes: r.breakMinutes,
            shift_type_id:
              r.shiftTypeId != null ? Number.parseInt(r.shiftTypeId, 10) : null,
          })),
        }),
      },
    );
    if (!response.ok) {
      throw await readShiftError(
        response,
        "Änderung konnte nicht gespeichert werden",
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
          ...(payload.occurrenceShiftId !== undefined
            ? {
                // Backend IDs are int64 values; serializing this as a number
                // would round values beyond Number.MAX_SAFE_INTEGER.
                occurrence_shift_id: payload.occurrenceShiftId,
              }
            : {}),
          start_time: payload.startTime,
          end_time: payload.endTime,
          break_minutes: payload.breakMinutes,
          shift_type_id:
            payload.shiftTypeId != null
              ? Number.parseInt(payload.shiftTypeId, 10)
              : null,
          // Only sent when the caller actually edited the rule: an absent key
          // means "keep what the series already has" on the backend.
          ...(payload.weekdays !== undefined
            ? { weekdays: payload.weekdays }
            : {}),
          ...(payload.weekPattern !== undefined
            ? { week_pattern: payload.weekPattern }
            : {}),
          ...(payload.validUntil !== undefined
            ? { valid_until: payload.validUntil }
            : {}),
        }),
      },
    );
    return readSeriesResult(response);
  }

  /** The stored rule behind a series shift, so the modal can show and edit
   *  weekdays, rhythm, and validity instead of only one occurrence (#2028). */
  async getSeries(seriesId: string): Promise<SeriesRule> {
    const response = await sessionFetch(`/api/staff/shifts/series/${seriesId}`);
    if (!response.ok) {
      throw await readShiftError(response, "Serie konnte nicht geladen werden");
    }
    const json = (await response.json()) as { data: BackendSeriesRule };
    return mapSeriesRule(json.data);
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

async function readAssignmentList(
  response: Response,
): Promise<OwnAssignment[]> {
  if (!response.ok) {
    throw await readShiftError(
      response,
      "Einsätze konnten nicht geladen werden",
    );
  }
  const json = (await response.json()) as {
    data: BackendOwnAssignment[] | null;
  };
  return (json.data ?? []).map(mapOwnAssignment);
}

class OwnShiftService {
  async getOwnShifts(from: string, to: string): Promise<StaffShift[]> {
    const response = await sessionFetch(
      `/api/time-tracking/shifts?from=${from}&to=${to}`,
    );
    return readShiftList(response);
  }

  /** The staff member's own Betreuungsplan blocks (Ort/Aufgabe + Vertretungen)
   *  for the range — "Mein Tag" (#1844). */
  async getOwnAssignments(from: string, to: string): Promise<OwnAssignment[]> {
    const response = await sessionFetch(
      `/api/time-tracking/assignments?from=${from}&to=${to}`,
    );
    return readAssignmentList(response);
  }
}

export const staffShiftService = new StaffShiftService();
export const staffShiftSeriesService = new StaffShiftSeriesService();
export const ownShiftService = new OwnShiftService();
