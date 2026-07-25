import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./session-cache", () => ({
  sessionFetch: vi.fn(),
}));

import { sessionFetch } from "./session-cache";
import {
  staffBalanceAdjustmentService,
  staffMonthCloseService,
} from "./staff-api";

const mockedSessionFetch = vi.mocked(sessionFetch);

const snapshot = {
  staff_id: 12,
  year: 2026,
  month: 6,
  closing_balance_minutes: 90,
  carry_in_minutes: 30,
  target_minutes: 7_200,
  actual_minutes: 7_260,
  credited_minutes: 0,
  adjustment_minutes: 0,
  closed_at: "2026-07-01T08:00:00Z",
  closed_by: 4,
  close_reason: "Lohnlauf",
  source: "manual",
};

function jsonResponse(data: unknown): Response {
  return {
    ok: true,
    json: () => Promise.resolve(data),
  } as Response;
}

function errorResponse(code: string, message = "backend error"): Response {
  return {
    ok: false,
    text: () => Promise.resolve(JSON.stringify({ code, error: message })),
  } as Response;
}

describe("staffMonthCloseService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches and maps the school month-close status", async () => {
    mockedSessionFetch
      .mockResolvedValueOnce(jsonResponse({ data: [snapshot] }))
      .mockResolvedValueOnce(jsonResponse({ data: null }));

    const result = await staffMonthCloseService.getStatus(2026, 6);
    const empty = await staffMonthCloseService.getStatus(2026, 5);

    expect(mockedSessionFetch).toHaveBeenNthCalledWith(
      1,
      "/api/staff/time-tracking/month-close?year=2026&month=6",
    );
    expect(result).toEqual([
      {
        staffId: "12",
        year: 2026,
        month: 6,
        closingBalanceMinutes: 90,
        closedAt: "2026-07-01T08:00:00Z",
        closedBy: "4",
        closeReason: "Lohnlauf",
      },
    ]);
    expect(empty).toEqual([]);
  });

  it("reports a failed status request", async () => {
    mockedSessionFetch.mockResolvedValueOnce({
      ok: false,
      statusText: "Bad Gateway",
    } as Response);

    await expect(staffMonthCloseService.getStatus(2026, 6)).rejects.toThrow(
      "Failed to fetch month close status: Bad Gateway",
    );
  });

  it("closes a month with backend request keys and maps the result", async () => {
    mockedSessionFetch.mockResolvedValueOnce(
      jsonResponse({
        data: {
          year: 2026,
          month: 6,
          closed_staff: 19,
          skipped_staff: 2,
          snapshots: [snapshot],
        },
      }),
    );

    const result = await staffMonthCloseService.closeMonth({
      year: 2026,
      month: 6,
      reason: "Lohnlauf",
    });

    expect(mockedSessionFetch).toHaveBeenCalledWith(
      "/api/staff/time-tracking/month-close",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          year: 2026,
          month: 6,
          reason: "Lohnlauf",
        }),
      },
    );
    expect(result).toEqual({
      year: 2026,
      month: 6,
      closedStaff: 19,
      skippedStaff: 2,
    });
  });

  it.each([
    ["month_not_closable", "Dieser Monat kann noch nicht abgeschlossen werden"],
    ["later_month_closed", "Ein späterer Monat ist bereits abgeschlossen"],
    ["unexpected_code", "backend error"],
  ])("maps close error %s", async (code, message) => {
    mockedSessionFetch.mockResolvedValueOnce(errorResponse(code));

    await expect(
      staffMonthCloseService.closeMonth({
        year: 2026,
        month: 6,
        reason: "Lohnlauf",
      }),
    ).rejects.toThrow(message);
  });

  it("reopens one staff month with backend request keys", async () => {
    mockedSessionFetch.mockResolvedValueOnce({ ok: true } as Response);

    await staffMonthCloseService.reopenMonth("12", {
      year: 2026,
      month: 6,
      reason: "Fehlende Schicht",
    });

    expect(mockedSessionFetch).toHaveBeenCalledWith(
      "/api/staff/12/time-tracking/month-close/reopen",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          year: 2026,
          month: 6,
          reason: "Fehlende Schicht",
        }),
      },
    );
  });

  it.each([
    ["month_not_closed", "Dieser Monat ist nicht abgeschlossen."],
    [
      "later_month_closed",
      "Für diese Person ist ein späterer Monat noch abgeschlossen",
    ],
    ["unexpected_code", "backend error"],
  ])("maps reopen error %s", async (code, message) => {
    mockedSessionFetch.mockResolvedValueOnce(errorResponse(code));

    await expect(
      staffMonthCloseService.reopenMonth("12", {
        year: 2026,
        month: 6,
        reason: "Fehlende Schicht",
      }),
    ).rejects.toThrow(message);
  });
});

describe("staffBalanceAdjustmentService closed-month errors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("explains a rejected adjustment in a closed month", async () => {
    mockedSessionFetch.mockResolvedValueOnce(
      errorResponse("adjustment_in_closed_month"),
    );

    await expect(
      staffBalanceAdjustmentService.create("12", {
        type: "payout",
        minutesDelta: -60,
        effectiveDate: "2026-06-30",
        note: "Korrektur",
      }),
    ).rejects.toThrow(
      "Der gewählte Monat ist abgeschlossen. Buche die Korrektur mit einem Datum im offenen Monat oder öffne den Monatsabschluss wieder.",
    );
  });

  it("explains a rejected reset in a closed month", async () => {
    mockedSessionFetch.mockResolvedValueOnce(
      errorResponse("adjustment_in_closed_month"),
    );

    await expect(
      staffBalanceAdjustmentService.reset("12", {
        effectiveDate: "2026-06-30",
        carryoverMinutes: 0,
        note: "Korrektur",
      }),
    ).rejects.toThrow(
      "Der gewählte Monat ist abgeschlossen. Wähle ein Datum im offenen Monat oder öffne den Monatsabschluss wieder.",
    );
  });
});
