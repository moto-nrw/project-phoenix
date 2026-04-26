// Calendar period (Kalenderperiode) — domain types and mappers.
//
// Calendar periods anchor the materialization service: without an active
// period covering a candidate date, materialization silently no-ops. They
// also drive A/B week resolution via week_cycle_length + week_cycle_anchor.
//
// Backend reference: backend/models/schedule/calendar_period.go,
// backend/api/timetable/api.go (CalendarPeriodRequest / Response).

export type PeriodType = "school_year" | "semester" | "holiday" | "custom";

export const PERIOD_TYPES: PeriodType[] = [
  "school_year",
  "semester",
  "holiday",
  "custom",
];

export const PERIOD_TYPE_LABELS: Record<PeriodType, string> = {
  school_year: "Schuljahr",
  semester: "Halbjahr",
  holiday: "Ferien",
  custom: "Sonstiges",
};

/** Backend wire shape (snake_case, int64 IDs). */
export interface BackendCalendarPeriod {
  id: number;
  tenant_id: number;
  name: string;
  period_type: PeriodType;
  start_date: string; // YYYY-MM-DD
  end_date: string; // YYYY-MM-DD
  week_cycle_length: number;
  week_cycle_anchor?: string | null; // YYYY-MM-DD
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

/** Frontend shape (camelCase, string IDs). */
export interface CalendarPeriod {
  id: string;
  tenantId: string;
  name: string;
  periodType: PeriodType;
  startDate: string; // YYYY-MM-DD
  endDate: string; // YYYY-MM-DD
  weekCycleLength: number;
  weekCycleAnchor: string | null;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

/** POST/PUT body — backend expects YYYY-MM-DD strings, not Date objects. */
export interface CalendarPeriodInput {
  name: string;
  period_type: PeriodType;
  start_date: string;
  end_date: string;
  week_cycle_length: number;
  week_cycle_anchor?: string;
  is_active: boolean;
}

export function mapPeriod(raw: BackendCalendarPeriod): CalendarPeriod {
  return {
    id: String(raw.id),
    tenantId: String(raw.tenant_id),
    name: raw.name,
    periodType: raw.period_type,
    startDate: raw.start_date,
    endDate: raw.end_date,
    weekCycleLength: raw.week_cycle_length,
    weekCycleAnchor: raw.week_cycle_anchor ?? null,
    isActive: raw.is_active,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
  };
}

export function mapPeriods(raw: BackendCalendarPeriod[]): CalendarPeriod[] {
  return raw.map(mapPeriod);
}

/**
 * Formats "01.09.2025 – 31.07.2026" for the list view. Both dates are
 * already YYYY-MM-DD so we splice without going through Date.
 */
export function formatPeriodRange(p: CalendarPeriod): string {
  return `${germanDate(p.startDate)} – ${germanDate(p.endDate)}`;
}

function germanDate(iso: string): string {
  const [y, m, d] = iso.split("-");
  return `${d}.${m}.${y}`;
}
