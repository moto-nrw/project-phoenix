import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api-error";
import {
  DAY_LOG_STATUS_COLORS,
  DAY_LOG_STATUS_ORDER,
  dayLogExportUrl,
  dayLogSourceLabel,
  fetchDayLog,
  type DayLogResponse,
} from "./day-log-api";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

const dayLogPayload: DayLogResponse = {
  date: "2026-07-24",
  groups: [
    {
      group_id: "7",
      name: "Igel",
      students: [
        {
          student_id: "42",
          first_name: "Mia",
          last_name: "Muster",
          school_class: "1a",
          status: "present",
          label: "Anwesend",
          check_in_time: "2026-07-24T06:58:00Z",
        },
      ],
      counters: {
        present: 1,
        sick: 0,
        class_trip: 0,
        excused: 0,
        absent: 0,
        not_scheduled: 0,
        total: 1,
      },
    },
  ],
  counters: {
    present: 1,
    sick: 0,
    class_trip: 0,
    excused: 0,
    absent: 0,
    not_scheduled: 0,
    total: 1,
  },
};

describe("fetchDayLog", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("fetches the day log for a date and unwraps the data envelope", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: dayLogPayload }));

    await expect(fetchDayLog("2026-07-24")).resolves.toEqual(dayLogPayload);
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/day-log?date=2026-07-24",
      { cache: "no-store" },
    );
  });

  it("narrows the request to one group via group_id", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ data: dayLogPayload }));

    await fetchDayLog("2026-07-24", "7");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students/day-log?date=2026-07-24&group_id=7",
      { cache: "no-store" },
    );
  });

  it("carries the backend code, details, field errors and request ID", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        {
          error: "feature_disabled",
          code: "students.day_log_disabled",
          details: { setting: "day_log" },
          errors: [{ field: "date", reason: "unavailable" }],
          instance: "request-day-log-403",
        },
        { status: 403 },
      ),
    );

    const failure = fetchDayLog("2026-07-24");
    await expect(failure).rejects.toBeInstanceOf(ApiError);
    await expect(failure).rejects.toMatchObject({
      code: "students.day_log_disabled",
      details: { setting: "day_log" },
      errors: [{ field: "date", reason: "unavailable" }],
      requestId: "request-day-log-403",
      message: "day log request failed (403)",
    });
  });

  it("reads the state codes, never the error text", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse(
          { error: "no_permitted_groups", code: "students.day_log_no_groups" },
          { status: 403 },
        ),
      )
      .mockResolvedValueOnce(
        // A text alone no longer selects a state: it is the class code.
        jsonResponse({ error: "feature_disabled" }, { status: 403 }),
      );

    await expect(fetchDayLog("2026-07-24")).rejects.toMatchObject({
      code: "students.day_log_no_groups",
    });
    await expect(fetchDayLog("2026-07-24")).rejects.toMatchObject({
      code: "general.permission",
    });
  });

  it("falls back to the class code for unrecognized error bodies", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({ error: "failed to load day log" }, { status: 500 }),
    );

    await expect(fetchDayLog("2026-07-24")).rejects.toMatchObject({
      code: "general.server",
    });
  });

  it("falls back to the class code for non-JSON error bodies", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response("<html>gateway timeout</html>", { status: 504 }),
    );

    await expect(fetchDayLog("2026-07-24")).rejects.toMatchObject({
      code: "general.unavailable",
      message: "day log request failed (504)",
    });
  });

  it("turns a request that never reached the API into general.unavailable", async () => {
    fetchMock.mockRejectedValueOnce(new TypeError("Failed to fetch"));

    await expect(fetchDayLog("2026-07-24")).rejects.toMatchObject({
      code: "general.unavailable",
    });
  });
});

describe("dayLogExportUrl", () => {
  it("builds the export URL for all groups", () => {
    expect(dayLogExportUrl("2026-07-24", "pdf")).toBe(
      "/api/students/day-log/export?date=2026-07-24&format=pdf",
    );
  });

  it("builds the export URL for one group", () => {
    expect(dayLogExportUrl("2026-07-24", "xlsx", "7")).toBe(
      "/api/students/day-log/export?date=2026-07-24&format=xlsx&group_id=7",
    );
  });
});

describe("dayLogSourceLabel", () => {
  it("labels parent-portal sign-offs", () => {
    expect(dayLogSourceLabel("parent")).toBe("Eltern-App");
  });

  it("labels cancelled care days", () => {
    expect(dayLogSourceLabel("care_plan_cancelled")).toBe(
      "Abmeldung im Betreuungsplan",
    );
  });

  it("stays empty for staff-entered or missing sources", () => {
    expect(dayLogSourceLabel("staff")).toBe("");
    expect(dayLogSourceLabel(undefined)).toBe("");
  });
});

describe("status metadata", () => {
  it("covers every status exactly once, unexplained absence last", () => {
    expect(DAY_LOG_STATUS_ORDER).toHaveLength(6);
    expect(new Set(DAY_LOG_STATUS_ORDER).size).toBe(6);
    expect(DAY_LOG_STATUS_ORDER.at(-1)).toBe("absent");
    for (const status of DAY_LOG_STATUS_ORDER) {
      expect(DAY_LOG_STATUS_COLORS[status]).toMatch(/^#/);
    }
  });
});
