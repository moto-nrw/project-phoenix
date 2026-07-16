import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockUseSWRAuth = vi.fn();
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

// The Berlin day is the unit under test's only clock. Mocking it to a day the
// system clock is NOT on (vitest pins TZ=Europe/Berlin, so `new Date()` would
// otherwise agree) is what makes the "keys off the Berlin month" assertions
// fail if someone reintroduces `new Date()`.
const mockBerlinToday = vi.fn();
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => mockBerlinToday() as string,
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: { getConfig: vi.fn(), getMonthSummary: vi.fn() },
}));
vi.mock("~/lib/staff-api", () => ({
  staffMonthSummaryService: { getMonthSummary: vi.fn() },
}));

import { useAccountBalance } from "./use-account-balance";

const CONFIG_KEY = "time-tracking-config";

/** Drives the two useSWRAuth calls (config, then month summary) by key. */
function mockSWR(opts: {
  config?: { accountStartDate: string };
  configLoading?: boolean;
  configError?: unknown;
  summary?: { closingBalanceMinutes: number };
  summaryError?: unknown;
}) {
  const summaryKeys: (string | null)[] = [];
  mockUseSWRAuth.mockImplementation((key: string | null) => {
    if (key === CONFIG_KEY) {
      return {
        data: opts.config,
        isLoading: opts.configLoading ?? false,
        error: opts.configError,
      };
    }
    summaryKeys.push(key);
    return {
      data: opts.summary,
      isLoading: false,
      error: opts.summaryError,
    };
  });
  return { summaryKey: () => summaryKeys.at(-1) ?? null };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockBerlinToday.mockReturnValue("2026-08-01");
});

describe("useAccountBalance", () => {
  it("keys the summary off the Berlin month, not the browser month", () => {
    // Berlin has rolled into August; a browser west of Berlin still says July.
    const swr = mockSWR({
      config: { accountStartDate: "" },
      summary: { closingBalanceMinutes: 90 },
    });

    const { result } = renderHook(() => useAccountBalance());

    expect(swr.summaryKey()).toBe("time-tracking-month-summary-2026-8");
    expect(result.current.balanceMinutes).toBe(90);
  });

  it("uses the admin key when a staffId is passed", () => {
    const swr = mockSWR({
      config: { accountStartDate: "" },
      summary: { closingBalanceMinutes: -30 },
    });

    const { result } = renderHook(() => useAccountBalance("42"));

    expect(swr.summaryKey()).toBe("staff-month-summary-42-2026-8");
    expect(result.current.balanceMinutes).toBe(-30);
  });

  it("reports no balance when the account starts later within the current month", () => {
    // The account has not started yet. The backend aggregates the anchor month
    // from the start DAY, so today it would answer a clean 0 for an empty span
    // — and printing that as "Stundenkonto Stand: 0:00" claims an account that
    // does not exist yet. Comparing only "YYYY-MM" missed exactly this case.
    const swr = mockSWR({
      config: { accountStartDate: "2026-08-20" },
      summary: { closingBalanceMinutes: 0 },
    });

    const { result } = renderHook(() => useAccountBalance());

    expect(swr.summaryKey()).toBeNull();
    expect(result.current.balanceMinutes).toBeNull();
    expect(result.current.isLoading).toBe(false);
  });

  it("resolves from the start day on", () => {
    // The boundary itself: on the start date the account exists, so the
    // balance is real and must be shown.
    const swr = mockSWR({
      config: { accountStartDate: "2026-08-01" },
      summary: { closingBalanceMinutes: 120 },
    });

    const { result } = renderHook(() => useAccountBalance());

    expect(swr.summaryKey()).toBe("time-tracking-month-summary-2026-8");
    expect(result.current.balanceMinutes).toBe(120);
  });

  it("reports no balance when the account start lies in a future month", () => {
    // The backend summarizes pre-anchor months standalone, so this month's
    // closingBalanceMinutes is its own Saldo, not a cumulative account balance.
    const swr = mockSWR({
      config: { accountStartDate: "2026-09-01" },
      summary: { closingBalanceMinutes: 480 },
    });

    const { result } = renderHook(() => useAccountBalance());

    expect(swr.summaryKey()).toBeNull();
    expect(result.current.balanceMinutes).toBeNull();
    expect(result.current.isLoading).toBe(false);
  });

  it("surfaces a config read failure instead of guessing an anchor", () => {
    const error = new Error("config unreachable");
    const swr = mockSWR({
      configError: error,
      summary: { closingBalanceMinutes: 480 },
    });

    const { result } = renderHook(() => useAccountBalance());

    expect(swr.summaryKey()).toBeNull();
    expect(result.current.balanceMinutes).toBeNull();
    expect(result.current.error).toBe(error);
  });

  it("waits for the anchor before fetching a balance", () => {
    const swr = mockSWR({ configLoading: true });

    const { result } = renderHook(() => useAccountBalance());

    expect(swr.summaryKey()).toBeNull();
    expect(result.current.isLoading).toBe(true);
    expect(result.current.balanceMinutes).toBeNull();
  });
});
