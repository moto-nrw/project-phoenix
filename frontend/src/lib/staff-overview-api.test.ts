import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";

import {
  mapDashboardSummary,
  mapTimeAccountRow,
  staffOverviewService,
  type BackendDashboardSummary,
  type BackendTimeTrackingOverviewRow,
} from "./staff-overview-api";

const sessionFetch = vi.hoisted(() => vi.fn());
vi.mock("./session-cache", () => ({ sessionFetch }));

function jsonResponse(data: unknown): Response {
  return {
    ok: true,
    json: () => Promise.resolve({ data }),
  } as unknown as Response;
}

const summaryFixture: BackendDashboardSummary = {
  active_staff_count: 18,
  sick_today: 2,
  vacation_today: 1,
  currently_clocked_in: 12,
  expected_clocked_in: 15,
  period: { soll_minutes: 9600, ist_minutes: 9000, delta_minutes: -120 },
  saldo_school_total_minutes: 1440,
  pending_requests_count: 3,
};

describe("mapDashboardSummary", () => {
  it("flattens the period block without recomputing the delta", () => {
    const mapped = mapDashboardSummary(summaryFixture);

    expect(mapped.sollMinutes).toBe(9600);
    expect(mapped.istMinutes).toBe(9000);
    // The backend delta credits sick and vacation days, so it is NOT
    // ist − soll (that would be −600 here). Recomputing it in the frontend is
    // exactly the drift this tranche exists to prevent.
    expect(mapped.deltaMinutes).toBe(-120);
    expect(mapped.deltaMinutes).not.toBe(
      summaryFixture.period.ist_minutes - summaryFixture.period.soll_minutes,
    );
    expect(mapped.saldoSchoolTotalMinutes).toBe(1440);
  });
});

describe("mapTimeAccountRow", () => {
  it("turns the int64 staff id into a string", () => {
    const row: BackendTimeTrackingOverviewRow = {
      staff_id: 42,
      first_name: "Anna",
      last_name: "Müller",
      name: "Anna Müller",
      employment_type: "part_time",
      soll_minutes: 600,
      ist_minutes: 660,
      balance_minutes: 60,
      remaining_vacation_days: 12.5,
    };

    expect(mapTimeAccountRow(row).staffId).toBe("42");
    expect(mapTimeAccountRow(row).remainingVacationDays).toBe(12.5);
  });
});

describe("staffOverviewService", () => {
  beforeEach(() => {
    sessionFetch.mockReset();
  });
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("requests the selected period", async () => {
    sessionFetch.mockResolvedValue(jsonResponse(summaryFixture));

    await staffOverviewService.getDashboardSummary("week");

    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/staff/dashboard-summary?period=week",
    );
  });

  it("omits year/month unless both are given", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse({ year: 2026, month: 7, rows: [] }),
    );

    await staffOverviewService.getTimeAccounts({ year: 2026 });

    // A bare year would be rejected by the backend; the client must not send it.
    expect(sessionFetch).toHaveBeenCalledWith(
      "/api/staff/time-tracking/overview",
    );
  });

  it("forwards month, employment type and saldo bounds", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse({ year: 2026, month: 6, rows: [] }),
    );

    await staffOverviewService.getTimeAccounts({
      year: 2026,
      month: 6,
      employmentType: "part_time",
      saldoMin: -600,
      saldoMax: 600,
    });

    const url = sessionFetch.mock.calls[0]?.[0] as string;
    expect(url).toContain("year=2026");
    expect(url).toContain("month=6");
    expect(url).toContain("employment_type=part_time");
    expect(url).toContain("saldo_min=-600");
    expect(url).toContain("saldo_max=600");
  });

  it("sends saldo_min=0 rather than dropping a zero bound", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse({ year: 2026, month: 7, rows: [] }),
    );

    await staffOverviewService.getTimeAccounts({ saldoMin: 0 });

    expect(sessionFetch.mock.calls[0]?.[0]).toContain("saldo_min=0");
  });

  it("survives a null rows array", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse({ year: 2026, month: 7, rows: null }),
    );

    await expect(staffOverviewService.getTimeAccounts()).resolves.toMatchObject(
      {
        rows: [],
      },
    );
  });

  it("throws a German message on a failed response", async () => {
    sessionFetch.mockResolvedValue({ ok: false } as Response);

    await expect(staffOverviewService.getTimeAccounts()).rejects.toThrow(
      "Zeitkonten konnten nicht geladen werden",
    );
  });
});
