import { describe, it, expect, beforeEach, vi, type Mock } from "vitest";

import {
  getMealPlanWeek,
  setDay,
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

  it("throws on a non-OK response", async () => {
    mockedFetch.mockResolvedValue(
      new Response("boom", { status: 500, statusText: "Server Error" }),
    );
    await expect(getMealPlanWeek("2026-07-06")).rejects.toThrow(
      /Failed to fetch meal plan/,
    );
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

  it("throws and logs on a non-OK response", async () => {
    mockedFetch.mockResolvedValue(
      new Response("nope", { status: 409, statusText: "Conflict" }),
    );
    await expect(setDay("2026-07-06", [])).rejects.toThrow(
      /Failed to save meal plan day/,
    );
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
