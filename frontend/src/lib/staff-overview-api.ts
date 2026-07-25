// Tenant-wide time-tracking views for the /staff Schul-Dashboard
// (#1417 Tranche 2a). Two endpoints with deliberately different permissions:
//
//   /api/staff/dashboard-summary        users:read           aggregated KPIs
//   /api/staff/time-tracking/overview   time_tracking:manage per-person rows
//
// Callers must gate the second one on the permission (see hasPermission in
// ~/lib/auth-utils); it is not a fallback path but a different audience.

import { sessionFetch } from "./session-cache";

// ─── Wire shapes (backend snake_case) ────────────────────────────────────────

export interface BackendDashboardSummary {
  active_staff_count: number;
  sick_today: number;
  vacation_today: number;
  currently_clocked_in: number;
  expected_clocked_in: number;
  period: {
    soll_minutes: number;
    ist_minutes: number;
    /**
     * NOT ist − soll. It is the sum of the per-staff month balances
     * (Ist + Gutschriften + Buchungen − Soll bis heute): sick and vacation days
     * are credited with the day's target, which is the whole premise of the
     * Stundenkonto. Any label must say so, or the school leadership reads a
     * different number than the Monatskarte shows.
     */
    delta_minutes: number;
  };
  /**
   * Account balance, not a period quantity — identical for week and month.
   * Never render it inside the period row.
   */
  saldo_school_total_minutes: number;
  pending_requests_count: number;
}

export interface BackendTimeTrackingOverviewRow {
  staff_id: number;
  first_name: string;
  last_name: string;
  name: string;
  /** "" when the staff member has no employment type recorded. */
  employment_type: string;
  soll_minutes: number;
  ist_minutes: number;
  balance_minutes: number;
  remaining_vacation_days: number;
}

export interface BackendTimeTrackingOverview {
  year: number;
  month: number;
  rows: BackendTimeTrackingOverviewRow[];
}

// ─── Frontend shapes (camelCase, string ids) ─────────────────────────────────

export type DashboardPeriod = "week" | "month";

export interface DashboardSummary {
  activeStaffCount: number;
  sickToday: number;
  vacationToday: number;
  currentlyClockedIn: number;
  expectedClockedIn: number;
  sollMinutes: number;
  istMinutes: number;
  deltaMinutes: number;
  saldoSchoolTotalMinutes: number;
  pendingRequestsCount: number;
}

export interface TimeAccountRow {
  staffId: string;
  firstName: string;
  lastName: string;
  name: string;
  employmentType: string;
  sollMinutes: number;
  istMinutes: number;
  balanceMinutes: number;
  remainingVacationDays: number;
}

export interface TimeAccountOverview {
  year: number;
  month: number;
  rows: TimeAccountRow[];
}

export function mapDashboardSummary(
  data: BackendDashboardSummary,
): DashboardSummary {
  return {
    activeStaffCount: data.active_staff_count,
    sickToday: data.sick_today,
    vacationToday: data.vacation_today,
    currentlyClockedIn: data.currently_clocked_in,
    expectedClockedIn: data.expected_clocked_in,
    sollMinutes: data.period.soll_minutes,
    istMinutes: data.period.ist_minutes,
    deltaMinutes: data.period.delta_minutes,
    saldoSchoolTotalMinutes: data.saldo_school_total_minutes,
    pendingRequestsCount: data.pending_requests_count,
  };
}

export function mapTimeAccountRow(
  data: BackendTimeTrackingOverviewRow,
): TimeAccountRow {
  return {
    staffId: data.staff_id.toString(),
    firstName: data.first_name,
    lastName: data.last_name,
    name: data.name,
    employmentType: data.employment_type,
    sollMinutes: data.soll_minutes,
    istMinutes: data.ist_minutes,
    balanceMinutes: data.balance_minutes,
    remainingVacationDays: data.remaining_vacation_days,
  };
}

// ─── Fetchers ────────────────────────────────────────────────────────────────

async function readJson<T>(response: Response, failure: string): Promise<T> {
  if (!response.ok) {
    throw new Error(failure);
  }
  const json = (await response.json()) as { data: T };
  return json.data;
}

export interface TimeAccountQuery {
  /** Both or neither — a bare month is rejected by the backend. */
  year?: number;
  month?: number;
  employmentType?: string;
  /** Inclusive bounds in minutes; negative values are normal. */
  saldoMin?: number;
  saldoMax?: number;
}

export const staffOverviewService = {
  async getDashboardSummary(
    period: DashboardPeriod,
  ): Promise<DashboardSummary> {
    const response = await sessionFetch(
      `/api/staff/dashboard-summary?period=${period}`,
    );
    return mapDashboardSummary(
      await readJson<BackendDashboardSummary>(
        response,
        "Schul-Übersicht konnte nicht geladen werden",
      ),
    );
  },

  /**
   * Requires time_tracking:manage. Sorting is deliberately NOT requested from
   * the backend: the table sorts client-side, so clicking a column header
   * costs no round trip. The server-side `sort`/`order` parameters stay
   * available for future paginated callers.
   */
  async getTimeAccounts(
    query: TimeAccountQuery = {},
  ): Promise<TimeAccountOverview> {
    const hasYear = query.year !== undefined;
    const hasMonth = query.month !== undefined;
    if (hasYear !== hasMonth) {
      throw new Error("Jahr und Monat müssen gemeinsam angegeben werden");
    }

    const params = new URLSearchParams();
    if (hasYear && hasMonth) {
      params.set("year", String(query.year));
      params.set("month", String(query.month));
    }
    if (query.employmentType)
      params.set("employment_type", query.employmentType);
    if (query.saldoMin !== undefined)
      params.set("saldo_min", String(query.saldoMin));
    if (query.saldoMax !== undefined)
      params.set("saldo_max", String(query.saldoMax));

    const suffix = params.toString();
    const response = await sessionFetch(
      `/api/staff/time-tracking/overview${suffix ? `?${suffix}` : ""}`,
    );
    const data = await readJson<BackendTimeTrackingOverview>(
      response,
      "Zeitkonten konnten nicht geladen werden",
    );
    return {
      year: data.year,
      month: data.month,
      rows: (data.rows ?? []).map(mapTimeAccountRow),
    };
  },
};
