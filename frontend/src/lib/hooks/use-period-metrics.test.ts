import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseSWRAuth = vi.fn();
vi.mock("~/lib/swr", () => ({
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
    getScheduleTargets: vi.fn(),
    getHistory: vi.fn(),
    getAbsences: vi.fn(),
  },
}));
vi.mock("~/lib/staff-api", () => ({
  staffMonthSummaryService: {
    getMonthSummary: vi.fn(),
    getScheduleTargets: vi.fn(),
  },
  staffHistoryService: { getHistory: vi.fn() },
  staffAbsenceService: { getAbsences: vi.fn() },
}));

import { usePeriodMetrics } from "./use-period-metrics";

const SUMMARY = {
  year: 2026,
  month: 8,
  targetMinutes: 9600,
  targetMinutesToDate: 2400,
  actualMinutes: 1800,
  creditedSickMinutes: 480,
  creditedVacationMinutes: 240,
  creditedOtherMinutes: 0,
  balanceMinutes: 120,
  closingBalanceMinutes: 300,
};

type SWROptions = { refreshInterval?: (latest: unknown) => number };

/**
 * Routes each useSWRAuth call by key and records the keys it was asked for
 * plus the options each key was requested with.
 */
function mockSWR(
  data: Partial<
    Record<"summary" | "targets" | "sessions" | "absences", unknown>
  >,
) {
  const keys: string[] = [];
  const configs = new Map<string, SWROptions>();
  mockUseSWRAuth.mockImplementation(
    (key: string | null, ...rest: unknown[]) => {
      if (typeof key === "string") {
        keys.push(key);
        configs.set(key, (rest[1] ?? {}) as SWROptions);
      }
      if (key === "time-tracking-config") {
        return { data: { accountStartDate: "2026-05-13" }, isLoading: false };
      }
      if (typeof key === "string" && key.includes("month-summary")) {
        return { data: data.summary, isLoading: false };
      }
      if (typeof key === "string" && key.includes("schedule-targets")) {
        return { data: data.targets, isLoading: false };
      }
      if (typeof key === "string" && key.includes("absences")) {
        // Own portal: raw camelCase StaffAbsence[]; admin: StaffAbsenceRow[].
        return { data: data.absences ?? [], isLoading: false };
      }
      // The own history key is SHARED with the Zeiterfassung table and carries
      // that table's shape; the admin key carries a flat session array.
      if (typeof key === "string" && key.startsWith("time-tracking-table-")) {
        return {
          data: data.sessions
            ? { sessions: data.sessions, weeklySummaries: [] }
            : undefined,
          isLoading: false,
        };
      }
      return { data: data.sessions, isLoading: false };
    },
  );
  return { keys, configs };
}

beforeEach(() => {
  vi.clearAllMocks();
  // Berlin has rolled into August; a browser west of Berlin still says July.
  mockBerlinToday.mockReturnValue("2026-08-05");
});

describe("usePeriodMetrics", () => {
  it("keys the month and week off the Berlin day, not the browser's", () => {
    const swr = mockSWR({ summary: SUMMARY, targets: new Map(), sessions: [] });

    renderHook(() => usePeriodMetrics("42"));

    expect(swr.keys).toContain("staff-month-summary-42-2026-8");
    // Mon 2026-08-03 .. Sun 2026-08-09 around the mocked Wednesday.
    expect(swr.keys).toContain(
      "staff-schedule-targets-42-2026-08-03-2026-08-09",
    );
    expect(swr.keys).toContain("staff-history-42-2026-08-03-2026-08-09");
  });

  it("uses own-portal keys when no staffId is passed", () => {
    const swr = mockSWR({ summary: SUMMARY, targets: new Map(), sessions: [] });

    renderHook(() => usePeriodMetrics());

    expect(swr.keys).toContain("time-tracking-month-summary-2026-8");
    // Same key the own Zeiterfassung table uses → SWR dedupes the overlap.
    expect(swr.keys).toContain("time-tracking-table-2026-08-03-2026-08-09");
  });

  it("derives the month card from the server summary, credits included", () => {
    mockSWR({ summary: SUMMARY, targets: new Map(), sessions: [] });

    const { result } = renderHook(() => usePeriodMetrics("42"));

    expect(result.current.month).toEqual({
      soll: 9600, // full month, not the to-date figure
      ist: 1800 + 480 + 240, // Ansatz B: absence credits count towards Ist
      delta: 120, // already priced against the Soll up to today, server-side
    });
  });

  it("computes the week from the date-valid targets, not a schedule", () => {
    mockSWR({
      summary: SUMMARY,
      targets: new Map([
        ["2026-08-03", 480],
        ["2026-08-04", 240],
      ]),
      sessions: [
        {
          date: "2026-08-03",
          status: "present",
          net_minutes: 480,
          check_in_time: "08:00",
          check_out_time: "16:00",
          break_minutes: 0,
        },
      ],
    });

    const { result } = renderHook(() => usePeriodMetrics("42"));

    // soll = the full week (480 Mon + 240 Tue), delta = Ist against the Soll up
    // to the mocked Wednesday (also 720, since Wed has no target).
    expect(result.current.week).toEqual({ soll: 720, ist: 480, delta: -240 });
  });

  // The own keys are shared with the Zeiterfassung table, and one SWR key is
  // one cache entry: the hook must read that table's shape rather than expect
  // its own, or whichever consumer lost the race gets the other's payload.
  it("reads the own history under the table's shape and adapts it", () => {
    mockSWR({
      summary: SUMMARY,
      targets: new Map([["2026-08-03", 480]]),
      // camelCase WorkSessionHistory, as the own /history endpoint returns it.
      sessions: [
        {
          id: "1",
          date: "2026-08-03",
          status: "present",
          netMinutes: 300,
          checkInTime: "08:00",
          checkOutTime: "13:00",
          breakMinutes: 0,
        },
      ],
    });

    const { result } = renderHook(() => usePeriodMetrics());

    // 300 only survives the camelCase -> snake_case adapter.
    expect(result.current.week?.ist).toBe(300);
  });

  // /history computes an open session's net_minutes at request time only. The
  // month card and the Stundenkonto poll every minute, so an unpolled week card
  // would freeze at check-in time and contradict them on the same screen.
  it("polls the week history while a session is open, and not otherwise", () => {
    const swr = mockSWR({ summary: SUMMARY, targets: new Map(), sessions: [] });

    renderHook(() => usePeriodMetrics("42"));

    const refresh = swr.configs.get(
      "staff-history-42-2026-08-03-2026-08-09",
    )?.refreshInterval;
    expect(refresh?.([{ check_out_time: null }])).toBe(60_000);
    expect(refresh?.([{ check_out_time: "2026-08-03T16:00:00Z" }])).toBe(0);
    expect(refresh?.(undefined)).toBe(0);
  });

  it("polls the own week history under the table's shape", () => {
    const swr = mockSWR({ summary: SUMMARY, targets: new Map(), sessions: [] });

    renderHook(() => usePeriodMetrics());

    const refresh = swr.configs.get(
      "time-tracking-table-2026-08-03-2026-08-09",
    )?.refreshInterval;
    // camelCase, as the own /history endpoint returns it.
    expect(refresh?.({ sessions: [{ checkOutTime: null }] })).toBe(60_000);
    expect(
      refresh?.({ sessions: [{ checkOutTime: "2026-08-03T16:00:00Z" }] }),
    ).toBe(0);
  });

  it("reports no week rather than pricing Ist against a missing Soll", () => {
    mockSWR({ summary: SUMMARY, targets: undefined, sessions: [] });

    const { result } = renderHook(() => usePeriodMetrics("42"));

    expect(result.current.week).toBeNull();
  });

  it("reports no month while the summary is still loading", () => {
    mockSWR({ summary: undefined, targets: new Map(), sessions: [] });

    const { result } = renderHook(() => usePeriodMetrics("42"));

    expect(result.current.month).toBeNull();
  });
});
