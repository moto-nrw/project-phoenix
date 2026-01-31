// Helper functions for time tracking data transformation and calculations

// Backend response types (snake_case, numbers for IDs)
export interface BackendWorkSession {
  id: number;
  staff_id: number;
  date: string;
  status: "present" | "home_office";
  check_in_time: string;
  check_out_time: string | null;
  break_minutes: number;
  notes: string;
  auto_checked_out: boolean;
  created_by: number;
  updated_by: number | null;
  created_at: string;
  updated_at: string;
}

export interface BackendWorkSessionBreak {
  id: number;
  session_id: number;
  started_at: string;
  ended_at: string | null;
  duration_minutes: number;
  created_at: string;
  updated_at: string;
}

export interface BackendWorkSessionHistory extends BackendWorkSession {
  net_minutes: number;
  is_overtime: boolean;
  is_break_compliant: boolean;
  breaks: BackendWorkSessionBreak[] | null;
}

// Frontend types (camelCase, string IDs)
export interface WorkSession {
  id: string;
  staffId: string;
  date: string;
  status: "present" | "home_office";
  checkInTime: string;
  checkOutTime: string | null;
  breakMinutes: number;
  notes: string;
  autoCheckedOut: boolean;
  createdBy: string;
  updatedBy: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface WorkSessionBreak {
  id: string;
  sessionId: string;
  startedAt: string;
  endedAt: string | null;
  durationMinutes: number;
}

export interface WorkSessionHistory extends WorkSession {
  netMinutes: number;
  isOvertime: boolean;
  isBreakCompliant: boolean;
  breaks: WorkSessionBreak[];
}

/**
 * Maps backend work session response to frontend type
 */
export function mapWorkSessionResponse(data: BackendWorkSession): WorkSession {
  return {
    id: data.id.toString(),
    staffId: data.staff_id.toString(),
    date: data.date.split("T")[0] ?? data.date,
    status: data.status,
    checkInTime: data.check_in_time,
    checkOutTime: data.check_out_time ?? null,
    breakMinutes: data.break_minutes,
    notes: data.notes ?? "",
    autoCheckedOut: data.auto_checked_out,
    createdBy: data.created_by.toString(),
    updatedBy: data.updated_by != null ? data.updated_by.toString() : null,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

/**
 * Maps backend break response to frontend type
 */
export function mapWorkSessionBreakResponse(
  data: BackendWorkSessionBreak,
): WorkSessionBreak {
  return {
    id: data.id.toString(),
    sessionId: data.session_id.toString(),
    startedAt: data.started_at,
    endedAt: data.ended_at ?? null,
    durationMinutes: data.duration_minutes,
  };
}

/**
 * Maps backend work session history response to frontend type
 */
export function mapWorkSessionHistoryResponse(
  data: BackendWorkSessionHistory,
): WorkSessionHistory {
  return {
    ...mapWorkSessionResponse(data),
    netMinutes: data.net_minutes,
    isOvertime: data.is_overtime,
    isBreakCompliant: data.is_break_compliant,
    breaks: (data.breaks ?? []).map(mapWorkSessionBreakResponse),
  };
}

/**
 * Formats duration in minutes to human-readable string
 * @param minutes - Duration in minutes
 * @returns Formatted string like "6h 30min" or "0min" or "--"
 */
export function formatDuration(minutes: number | null | undefined): string {
  if (minutes === null || minutes === undefined || Number.isNaN(minutes)) {
    return "--";
  }

  if (minutes === 0) {
    return "0min";
  }

  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;

  if (hours === 0) {
    return `${mins}min`;
  }

  if (mins === 0) {
    return `${hours}h`;
  }

  return `${hours}h ${mins}min`;
}

/**
 * Formats ISO timestamp to time string
 * @param isoString - ISO 8601 timestamp
 * @returns Formatted time like "08:15" or "--:--"
 */
export function formatTime(isoString: string | null | undefined): string {
  if (!isoString) {
    return "--:--";
  }

  try {
    const date = new Date(isoString);
    if (Number.isNaN(date.getTime())) {
      return "--:--";
    }

    const hours = date.getHours().toString().padStart(2, "0");
    const minutes = date.getMinutes().toString().padStart(2, "0");
    return `${hours}:${minutes}`;
  } catch {
    return "--:--";
  }
}

/**
 * Gets array of dates for a week (Monday to Sunday)
 * @param date - Any date within the week
 * @returns Array of 7 dates starting from Monday
 */
export function getWeekDays(date: Date): Date[] {
  const days: Date[] = [];
  const currentDay = date.getDay();
  const mondayOffset = currentDay === 0 ? -6 : 1 - currentDay;

  for (let i = 0; i < 7; i++) {
    const day = new Date(date);
    day.setDate(date.getDate() + mondayOffset + i);
    day.setHours(0, 0, 0, 0);
    days.push(day);
  }

  return days;
}

/**
 * Gets ISO week number for a date
 * @param date - Date to get week number for
 * @returns ISO week number (1-53)
 */
export function getWeekNumber(date: Date): number {
  const target = new Date(date.valueOf());
  const dayNr = (date.getDay() + 6) % 7;
  target.setDate(target.getDate() - dayNr + 3);
  const firstThursday = target.valueOf();
  target.setMonth(0, 1);
  if (target.getDay() !== 4) {
    target.setMonth(0, 1 + ((4 - target.getDay() + 7) % 7));
  }
  return 1 + Math.ceil((firstThursday - target.valueOf()) / 604800000);
}

/**
 * Returns array of compliance warnings for a work session
 * @param session - Work session with calculated fields
 * @returns Array of warning messages
 */
export function getComplianceWarnings(session: WorkSessionHistory): string[] {
  const warnings: string[] = [];

  if (!session.isBreakCompliant && session.netMinutes > 0) {
    if (session.netMinutes > 540 && session.breakMinutes < 45) {
      warnings.push("Pausenzeit < 45min bei >9h Arbeitszeit");
    } else if (session.netMinutes > 360 && session.breakMinutes < 30) {
      warnings.push("Pausenzeit < 30min bei >6h Arbeitszeit");
    }
  }

  if (session.autoCheckedOut) {
    warnings.push("Automatisch ausgestempelt");
  }

  return warnings;
}

/**
 * Calculates net working minutes from check-in/out times and break
 * @param checkIn - Check-in ISO timestamp
 * @param checkOut - Check-out ISO timestamp (null if still active)
 * @param breakMinutes - Break duration in minutes
 * @returns Net working minutes, or null if still active
 */
export function calculateNetMinutes(
  checkIn: string,
  checkOut: string | null,
  breakMinutes: number,
): number | null {
  if (!checkOut) {
    return null;
  }

  try {
    const checkInDate = new Date(checkIn);
    const checkOutDate = new Date(checkOut);

    if (
      Number.isNaN(checkInDate.getTime()) ||
      Number.isNaN(checkOutDate.getTime())
    ) {
      return null;
    }

    const totalMinutes = Math.floor(
      (checkOutDate.getTime() - checkInDate.getTime()) / 60000,
    );
    return Math.max(0, totalMinutes - breakMinutes);
  } catch {
    return null;
  }
}
