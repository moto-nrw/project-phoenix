import { describe, it, expect, beforeEach, vi, type Mock } from "vitest";

import {
  getMealPlanWeek,
  getDailyMealParticipants,
  getWeeklyMealParticipants,
  setDay,
  setMealParticipantDay,
  mondayOf,
  workWeekDates,
} from "./meal-plan-api";
import { sessionFetch } from "./session-cache";

vi.mock("./session-cache", () => ({
  sessionFetch: vi.fn(),
}));

const mockedFetch = sessionFetch as unknown as Mock;

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

beforeEach(() => {
  mockedFetch.mockReset();
});

describe("getWeeklyMealParticipants", () => {
  it("requests the week and maps each day's children", async () => {
    let seenURL = "";
    mockedFetch.mockImplementation(async (url: string) => {
      seenURL = url;
      return jsonResponse({
        data: {
          week_start: "2026-09-07",
          cutoff_time: "09:00",
          cutoff_lead_days: 1,
          cutoff_weekday: 3,
          cancellation_cutoff_time: "16:00",
          cancellation_cutoff_lead_days: 1,
          days: [
            {
              date: "2026-09-07",
              participants: [
                {
                  student_id: "42",
                  first_name: "Mia",
                  last_name: "Muster",
                  school_class: "2a",
                },
              ],
            },
            { date: "2026-09-08", closed: true, participants: [] },
          ],
        },
      });
    });

    const week = await getWeeklyMealParticipants("2026-09-09");
    expect(seenURL).toBe(
      "/api/meal-plan/participants/week?week_start=2026-09-09",
    );
    expect(week).toEqual({
      weekStart: "2026-09-07",
      cutoffTime: "09:00",
      cutoffLeadDays: 1,
      cutoffWeekday: 3,
      cancellationCutoffTime: "16:00",
      cancellationCutoffLeadDays: 1,
      days: [
        {
          date: "2026-09-07",
          closed: false,
          participants: [
            {
              studentId: "42",
              firstName: "Mia",
              lastName: "Muster",
              schoolClass: "2a",
            },
          ],
        },
        { date: "2026-09-08", closed: true, participants: [] },
      ],
    });
  });
});

describe("getDailyMealParticipants", () => {
  it("maps the closed flag, the available children and default cutoffs", async () => {
    mockedFetch.mockResolvedValue(
      jsonResponse({
        data: {
          date: "2026-09-07",
          cutoff_time: "09:00",
          closed: false,
          participants: [],
          available: [
            {
              student_id: "43",
              first_name: "Ben",
              last_name: "Beispiel",
              school_class: "3b",
            },
          ],
        },
      }),
    );

    await expect(getDailyMealParticipants("2026-09-07")).resolves.toEqual({
      date: "2026-09-07",
      cutoffTime: "09:00",
      cutoffLeadDays: 0,
      cutoffWeekday: 0,
      cancellationCutoffTime: "",
      cancellationCutoffLeadDays: 0,
      closed: false,
      participants: [],
      available: [
        {
          studentId: "43",
          firstName: "Ben",
          lastName: "Beispiel",
          schoolClass: "3b",
        },
      ],
    });
  });
});

describe("setMealParticipantDay", () => {
  it("signs a child up or off with PUT and resets the day with DELETE", async () => {
    mockedFetch.mockResolvedValue(jsonResponse({ status: "success" }));

    await setMealParticipantDay("43", "2026-09-07", true);
    await setMealParticipantDay("43", "2026-09-07", null);

    expect(mockedFetch).toHaveBeenNthCalledWith(
      1,
      "/api/meal-plan/participants/43/2026-09-07",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ participating: true }),
      }),
    );
    expect(mockedFetch).toHaveBeenNthCalledWith(
      2,
      "/api/meal-plan/participants/43/2026-09-07",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("throws the API error of a refused change", async () => {
    mockedFetch.mockResolvedValue(
      jsonResponse(
        { status: 400, detail: "invalid", code: "general.input" },
        { status: 400 },
      ),
    );

    await expect(
      setMealParticipantDay("43", "2026-09-04", false),
    ).rejects.toThrow();
  });
});

describe("getMealPlanWeek", () => {
  it("requests the week and maps backend entries, trimming empty notes to null", async () => {
    let seenURL = "";
    mockedFetch.mockImplementation(async (url: string) => {
      seenURL = url;
      return jsonResponse({
        data: [
          {
            date: "2026-07-06",
            position: 0,
            dish: "Nudeln",
            note: "  vegetarisch  ",
          },
          { date: "2026-07-06", position: 1, dish: "Salat", note: "   " },
          { date: "2026-07-07", position: 0, dish: "Suppe", note: null },
        ],
      });
    });

    const out = await getMealPlanWeek("2026-07-06");
    expect(seenURL).toContain("/api/meal-plan?week_start=2026-07-06");
    expect(out).toHaveLength(3);
    // A note with content is preserved verbatim (untrimmed value kept).
    expect(out[0]?.note).toBe("  vegetarisch  ");
    // A whitespace-only note collapses to null.
    expect(out[1]?.note).toBeNull();
    // A null note stays null.
    expect(out[2]?.note).toBeNull();
  });

  it("accepts a bare array response shape", async () => {
    mockedFetch.mockResolvedValue(
      jsonResponse([
        { date: "2026-07-06", position: 0, dish: "Reis", note: null },
      ]),
    );
    const out = await getMealPlanWeek("2026-07-06");
    expect(out).toHaveLength(1);
    expect(out[0]?.dish).toBe("Reis");
  });

  it("returns an empty list for an unexpected object shape", async () => {
    mockedFetch.mockResolvedValue(jsonResponse({ unexpected: true }));
    const out = await getMealPlanWeek("2026-07-06");
    expect(out).toEqual([]);
  });

  it("throws an ApiError with the status class on a non-OK response", async () => {
    mockedFetch.mockResolvedValue(
      new Response("boom", { status: 500, statusText: "Server Error" }),
    );
    await expect(getMealPlanWeek("2026-07-06")).rejects.toMatchObject({
      name: "ApiError",
      status: 500,
      code: "general.server",
    });
  });

  it("URL-encodes the week start", async () => {
    let seenURL = "";
    mockedFetch.mockImplementation(async (url: string) => {
      seenURL = url;
      return jsonResponse({ data: [] });
    });
    await getMealPlanWeek("2026 07 06");
    expect(seenURL).toContain("week_start=2026%2007%2006");
  });
});

describe("setDay", () => {
  it("PUTs the day with the dishes payload", async () => {
    let seenURL = "";
    let seenInit: RequestInit | undefined;
    mockedFetch.mockImplementation(async (url: string, init?: RequestInit) => {
      seenURL = url;
      seenInit = init;
      return jsonResponse({});
    });

    await setDay("2026-07-06", [{ dish: "Nudeln", note: "vegetarisch" }]);
    expect(seenURL).toContain("/api/meal-plan/2026-07-06");
    expect(seenInit?.method).toBe("PUT");
    expect(JSON.parse(seenInit?.body as string)).toEqual({
      dishes: [{ dish: "Nudeln", note: "vegetarisch" }],
    });
  });

  it("keeps code, field errors and request id of a refused save", async () => {
    mockedFetch.mockResolvedValue(
      new Response(
        JSON.stringify({
          code: "meals.invalid_dishes",
          errors: [{ field: "dishes", reason: "invalid" }],
          instance: "req-1",
        }),
        { status: 400, statusText: "Bad Request" },
      ),
    );
    await expect(setDay("2026-07-06", [])).rejects.toMatchObject({
      name: "ApiError",
      status: 400,
      code: "meals.invalid_dishes",
      errors: [{ field: "dishes", reason: "invalid" }],
      requestId: "req-1",
    });
  });
});

describe("mondayOf", () => {
  it("returns the ISO Monday for any weekday", () => {
    // 2026-07-08 is a Wednesday → Monday is 2026-07-06.
    expect(mondayOf(new Date(2026, 6, 8))).toBe("2026-07-06");
    // Monday maps to itself.
    expect(mondayOf(new Date(2026, 6, 6))).toBe("2026-07-06");
    // Sunday belongs to the week that started the previous Monday.
    expect(mondayOf(new Date(2026, 6, 12))).toBe("2026-07-06");
  });
});

describe("workWeekDates", () => {
  it("expands a Monday into the five Mon-Fri ISO dates", () => {
    expect(workWeekDates("2026-07-06")).toEqual([
      "2026-07-06",
      "2026-07-07",
      "2026-07-08",
      "2026-07-09",
      "2026-07-10",
    ]);
  });
});
