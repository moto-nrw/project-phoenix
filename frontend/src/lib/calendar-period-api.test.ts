import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    info: vi.fn(),
    error: vi.fn(),
    warn: vi.fn(),
  }),
}));

import { calendarPeriodService } from "./calendar-period-api";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

const backendPeriod = {
  id: 5,
  name: "Schuljahr 2026/2027",
  period_type: "school_year",
  start_date: "2026-08-01",
  end_date: "2027-07-31",
  week_cycle_length: 2,
  week_cycle_anchor: "2026-08-03",
  is_active: true,
};

describe("calendarPeriodService", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("lists and maps calendar periods", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: [backendPeriod] }));

    await expect(calendarPeriodService.list()).resolves.toEqual([
      expect.objectContaining({
        id: "5",
        name: "Schuljahr 2026/2027",
        periodType: "school_year",
        weekCycleLength: 2,
        weekCycleAnchor: "2026-08-03",
      }),
    ]);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/periods",
      expect.objectContaining({ method: "GET", credentials: "include" }),
    );
  });

  it("gets, creates and updates a period with the expected body", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ data: backendPeriod }))
      .mockResolvedValueOnce(jsonResponse({ data: backendPeriod }))
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            ...backendPeriod,
            warnings: [
              {
                code: "overlapping_active_periods",
                message: "Überschneidung mit aktiven Zeiträumen",
                overlapping_period_ids: [3],
                overlapping_period_names: ["Halbjahr 1"],
              },
            ],
          },
        }),
      );

    const body = {
      name: "Schuljahr 2026/2027",
      period_type: "school_year" as const,
      start_date: "2026-08-01",
      end_date: "2027-07-31",
      week_cycle_length: 2,
      week_cycle_anchor: "2026-08-03",
      is_active: true,
    };

    await expect(calendarPeriodService.get("5")).resolves.toMatchObject({
      id: "5",
    });
    // create/update return { period, warnings } per the period-warning
    // contract; warnings default to [] when the backend omits the field.
    await expect(calendarPeriodService.create(body)).resolves.toMatchObject({
      period: { id: "5" },
      warnings: [],
    });
    await expect(
      calendarPeriodService.update("5", body),
    ).resolves.toMatchObject({
      period: { id: "5" },
      warnings: [
        {
          code: "overlapping_active_periods",
          message: "Überschneidung mit aktiven Zeiträumen",
          overlappingPeriodIds: ["3"],
          overlappingPeriodNames: ["Halbjahr 1"],
        },
      ],
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/periods",
      expect.objectContaining({ method: "POST", body: JSON.stringify(body) }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/timetable/periods/5",
      expect.objectContaining({ method: "PUT", body: JSON.stringify(body) }),
    );
  });

  it("bootstraps default periods and maps the response", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ data: { periods: [backendPeriod], created: true } }),
    );

    await expect(calendarPeriodService.bootstrap()).resolves.toEqual({
      periods: [expect.objectContaining({ id: "5" })],
      created: true,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/periods/bootstrap",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
        body: "{}",
      }),
    );
  });

  it("deletes periods and accepts 204 without JSON", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(calendarPeriodService.delete("5")).resolves.toBeUndefined();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/periods/5",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("surfaces backend error messages and generic non-JSON failures", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        { error: "Periode ueberlappt", code: "PERIOD_OVERLAP" },
        { status: 409 },
      ),
    );
    await expect(calendarPeriodService.list()).rejects.toMatchObject({
      message: "Periode ueberlappt",
      httpStatus: 409,
      code: "PERIOD_OVERLAP",
    });

    fetchMock.mockResolvedValueOnce(
      new Response("bad gateway", {
        status: 502,
        headers: { "Content-Type": "text/plain" },
      }),
    );
    await expect(calendarPeriodService.delete("5")).rejects.toMatchObject({
      message: "Anfrage fehlgeschlagen (HTTP 502)",
      httpStatus: 502,
    });
  });
});
