import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";

import {
  getSchoolHolidays,
  holidaysAsCalendarEvents,
} from "./school-holidays-api";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

describe("getSchoolHolidays", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("asks for the window and maps the federal state", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          federal_state: "DE-BY",
          federal_state_name: "Bayern",
          holidays: [{ date: "2027-01-06", name: "Heilige Drei Könige" }],
        },
      }),
    );

    await expect(
      getSchoolHolidays("2026-08-01", "2027-07-31"),
    ).resolves.toEqual({
      federalState: "DE-BY",
      federalStateName: "Bayern",
      holidays: [{ date: "2027-01-06", name: "Heilige Drei Könige" }],
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/timetable/holidays?from=2026-08-01&to=2027-07-31",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("reads a missing list as no holidays", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse({
        data: {
          federal_state: "DE-NW",
          federal_state_name: "Nordrhein-Westfalen",
          holidays: null,
        },
      }),
    );

    const result = await getSchoolHolidays("2026-07-01", "2026-07-31");
    expect(result.holidays).toEqual([]);
  });

  it("splits a school year longer than the API window and combines its holidays", async () => {
    fetchMock
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            federal_state: "DE-NW",
            federal_state_name: "Nordrhein-Westfalen",
            holidays: [
              { date: "2026-10-03", name: "Tag der Deutschen Einheit" },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          data: {
            federal_state: "DE-NW",
            federal_state_name: "Nordrhein-Westfalen",
            holidays: [{ date: "2027-09-05", name: "Beispieltag" }],
          },
        }),
      );

    await expect(
      getSchoolHolidays("2026-08-01", "2027-09-05"),
    ).resolves.toEqual({
      federalState: "DE-NW",
      federalStateName: "Nordrhein-Westfalen",
      holidays: [
        { date: "2026-10-03", name: "Tag der Deutschen Einheit" },
        { date: "2027-09-05", name: "Beispieltag" },
      ],
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/timetable/holidays?from=2026-08-01&to=2027-09-04",
      expect.objectContaining({ method: "GET" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/timetable/holidays?from=2027-09-05&to=2027-09-05",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("throws the API error with its code", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(
        { status: 403, code: "general.permission", detail: "forbidden" },
        { status: 403 },
      ),
    );

    const error: unknown = await getSchoolHolidays(
      "2026-07-01",
      "2026-07-31",
    ).catch((err: unknown) => err);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).code).toBe("general.permission");
  });
});

describe("holidaysAsCalendarEvents", () => {
  it("turns each holiday into a read-only all-day entry naming the state", () => {
    expect(
      holidaysAsCalendarEvents(
        [{ date: "2026-10-03", name: "Tag der Deutschen Einheit" }],
        "Nordrhein-Westfalen",
      ),
    ).toEqual([
      {
        id: "public_holiday:2026-10-03",
        source: "public_holiday",
        title: "Tag der Deutschen Einheit",
        description: "Gesetzlicher Feiertag in Nordrhein-Westfalen.",
        start_date: "2026-10-03",
        end_date: "2026-10-03",
        start_time: "00:00",
        end_time: "23:59",
        all_day: true,
        can_respond: false,
        can_edit: false,
      },
    ]);
  });
});
