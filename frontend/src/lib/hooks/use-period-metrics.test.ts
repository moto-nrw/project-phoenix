import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseSWRAuth = vi.fn();
vi.mock("~/lib/swr", () => ({
  // No SSE stream in these tests: readers keep their fallback interval.
  useLiveRefreshInterval: (fallbackMs = 60_000) => fallbackMs,
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

// The Berlin day is the hook's only clock. Mocking it to a day the system clock
// is NOT on (vitest pins TZ=Europe/Berlin, so `new Date()` would otherwise
// agree) is what makes the "keys off the Berlin week/month" assertions fail if
// someone reintroduces `new Date()`.
const mockBerlinToday = vi.fn();
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => mockBerlinToday() as string,
}));

vi.mock("~/lib/hooks/use-account-balance", () => ({
  useAccountBalance: () => ({
    balanceMinutes: 120,
    isLoading: false,
    error: undefined,
  }),
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: {
    getConfig: vi.fn(),
    getMonthSummary: vi.fn(),
    getDailyProjection: vi.fn(),
  },
}));
vi.mock("~/lib/staff-api", () => ({
  staffMonthSummaryService: {
    getMonthSummary: vi.fn(),
    getDailyProjection: vi.fn(),
  },
}));

import type { DayProjection } from "~/lib/time-tracking-helpers";

import { usePeriodMetrics } from "./use-period-metrics";

const SUMMARY = {
  year: 2026,
  month: 8,
  targetMinutes: 9600,
  targetMinutesToDate: 2400,
  actualMinutes: 1800,
  creditedSickMinutes: 480,
  creditedVacationMinutes: 240,
  creditedTrainingMinutes: 0,
  creditedOtherMinutes: 0,
  trainingDays: 0,
  sickDaysWithoutTarget: 0,
  vacationDaysWithoutTarget: 0,
  trainingDaysWithoutTarget: 0,
  otherDaysWithoutTarget: 0,
  balanceMinutes: 120,
  closingBalanceMinutes: 300,
};

type SWROptions = { refreshInterval?: number };

function day(overrides: Partial<DayProjection>): DayProjection {
  return {
    targetMinutes: 0,
    creditMinutes: 0,
    actualMinutes: 0,
    balanceMinutes: 0,
    ...overrides,
  };
}

/**
 * Routes each useSWRAuth call by key and records the keys it was asked for
 * plus the options each key was requested with.
 */
function mockSWR(
  data: Partial<Record<"summary" | "projection", unknown>>,
  errors: Partial<Record<"summary" | "projection", unknown>> = {},
) {
  const keys: string[] = [];
  const configs = new Map<string, SWROptions>();
  const mutates = new Map<string, ReturnType<typeof vi.fn>>();
  mockUseSWRAuth.mockImplementation(
    (key: string | null, ...rest: unknown[]) => {
      const mutate = vi.fn().mockResolvedValue(undefined);
      if (typeof key === "string") {
        keys.push(key);
        configs.set(key, (rest[1] ?? {}) as SWROptions);
        mutates.set(key, mutate);
      }
      if (key === "time-tracking-config") {
        return {
          data: { accountStartDate: "2026-05-13" },
          isLoading: false,
          mutate,
        };
      }
      if (typeof key === "string" && key.includes("month-summary")) {
        return {
          data: data.summary,
          isLoading: false,
          error: errors.summary,
          mutate,
        };
      }
      if (typeof key === "string" && key.includes("schedule-targets")) {
        return {
          // SWR keeps the last response while a revalidation fails.
          data: data.projection,
          isLoading: false,
          error: errors.projection,
          mutate,
        };
      }
      return { data: undefined, isLoading: false, mutate };
    },
  );
  return { keys, configs, mutates };
}

beforeEach(() => {
  vi.clearAllMocks();
  // Berlin has rolled into August; a browser west of Berlin still says July.
  mockBerlinToday.mockReturnValue("2026-08-05");
});

describe("usePeriodMetrics", () => {
  it("keys the month and week off the Berlin day, not the browser's", () => {
    const swr = mockSWR({ summary: SUMMARY, projection: new Map() });

    renderHook(() => usePeriodMetrics("42"));

    expect(swr.keys).toContain("staff-month-summary-42-2026-8");
    // Mon 2026-08-03 .. Sun 2026-08-09 around the mocked Wednesday.
    expect(swr.keys).toContain(
      "staff-schedule-targets-42-2026-08-03-2026-08-09",
    );
  });

  it("uses own-portal keys when no staffId is passed", () => {
    const swr = mockSWR({ summary: SUMMARY, projection: new Map() });

    renderHook(() => usePeriodMetrics());

    expect(swr.keys).toContain("time-tracking-month-summary-2026-8");
    // Same key the own Zeiterfassung table uses → SWR dedupes the overlap.
    expect(swr.keys).toContain(
      "time-tracking-schedule-targets-2026-08-03-2026-08-09",
    );
  });

  // The week used to credit absences from the absence list itself and could
  // then disagree with the Monatskarte for the same days (#6847). It reads
  // only the server's day projection now, never sessions or absences.
  it("loads no sessions or absences of its own", () => {
    const swr = mockSWR({ summary: SUMMARY, projection: new Map() });

    renderHook(() => usePeriodMetrics("42"));

    expect(
      swr.keys.filter((key) => /history|absences|table-/.test(key)),
    ).toEqual([]);
    expect(swr.keys).toContain("staff-month-summary-42-2026-8");
  });

  it("derives the month card from the server summary, credits included", () => {
    mockSWR({ summary: SUMMARY, projection: new Map() });

    const { result } = renderHook(() => usePeriodMetrics("42"));

    expect(result.current.month).toEqual({
      soll: 9600, // full month, not the to-date figure
      ist: 1800 + 480 + 240, // Ansatz B: absence credits count towards Ist
      delta: 120, // already priced against the Soll up to today, server-side
    });
  });

  it("sums the week from the server's day projection, credits included", () => {
    mockSWR({
      summary: SUMMARY,
      projection: new Map([
        // Mon worked, Tue on vacation (server credit), Wed today half worked.
        [
          "2026-08-03",
          day({ targetMinutes: 480, actualMinutes: 480, balanceMinutes: 0 }),
        ],
        ["2026-08-04", day({ targetMinutes: 240, creditMinutes: 240 })],
        [
          "2026-08-05",
          day({ targetMinutes: 480, actualMinutes: 120, balanceMinutes: -360 }),
        ],
        // Future: Soll only, as the server returns it.
        ["2026-08-06", day({ targetMinutes: 480 })],
      ]),
    });

    const { result } = renderHook(() => usePeriodMetrics("42"));

    expect(result.current.week).toEqual({
      soll: 1680,
      ist: 480 + 240 + 120,
      delta: -360,
    });
  });

  // An open session grows the day's Ist server-side. The month card and the
  // Stundenkonto poll every minute, so an unpolled week card would freeze at
  // check-in time and contradict them on the same screen.
  it("polls the week projection like the month card", () => {
    const swr = mockSWR({ summary: SUMMARY, projection: new Map() });

    renderHook(() => usePeriodMetrics());

    expect(
      swr.configs.get("time-tracking-schedule-targets-2026-08-03-2026-08-09")
        ?.refreshInterval,
    ).toBe(60_000);
  });

  it("reports no week while the projection is still loading", () => {
    mockSWR({ summary: SUMMARY, projection: undefined });

    const { result } = renderHook(() => usePeriodMetrics("42"));

    expect(result.current.week).toBeNull();
  });

  // A failed refresh still exposes SWR's cached projection. The week's figure
  // must nevertheless stay unavailable: the cached days might miss a sick
  // day's credit and read as a reliable value (#3885).
  it("reports no week and retries when refreshing the projection fails", async () => {
    const error = new Error("daily projection unavailable");
    const swr = mockSWR(
      {
        summary: SUMMARY,
        projection: new Map([["2026-08-03", day({ targetMinutes: 480 })]]),
      },
      { projection: error },
    );

    const { result } = renderHook(() => usePeriodMetrics());

    expect(result.current.week).toBeNull();
    expect(result.current.error).toBe(error);
    expect(result.current.failed).toBe(true);
    await result.current.retry();
    expect(
      swr.mutates.get("time-tracking-schedule-targets-2026-08-03-2026-08-09"),
    ).toHaveBeenCalledTimes(1);
  });

  it("exposes and retries a failed month summary", async () => {
    const error = new Error("month summary unavailable");
    const swr = mockSWR(
      { summary: undefined, projection: new Map() },
      { summary: error },
    );

    const { result } = renderHook(() => usePeriodMetrics());

    expect(result.current.month).toBeNull();
    expect(result.current.error).toBe(error);
    expect(result.current.failed).toBe(true);
    await result.current.retry();
    expect(
      swr.mutates.get("time-tracking-month-summary-2026-8"),
    ).toHaveBeenCalledTimes(1);
  });

  it("reports no month while the summary is still loading", () => {
    mockSWR({ summary: undefined, projection: new Map() });

    const { result } = renderHook(() => usePeriodMetrics("42"));

    expect(result.current.month).toBeNull();
  });
});
