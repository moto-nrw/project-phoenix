import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "~/lib/api-error";

const mockUseSWRAuth = vi.fn();
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]) => mockUseSWRAuth(...args),
}));

const mockUseUserContext = vi.fn();
vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: () => mockUseUserContext() as unknown,
}));

const mockGetSchedule = vi.fn();
vi.mock("~/lib/staff-api", () => ({
  staffScheduleService: {
    getSchedule: (...args: unknown[]) => mockGetSchedule(...args) as unknown,
  },
}));

const mockGetCurrentStaff = vi.fn();
vi.mock("~/lib/usercontext-api", () => ({
  userContextService: {
    getCurrentStaff: () => mockGetCurrentStaff() as unknown,
  },
}));

import { useOwnStaffSchedule } from "./use-own-staff-schedule";

const STAFF_KEY = "time-tracking-own-staff";
const SCHEDULE_PREFIX = "time-tracking-own-schedule-";

interface SWRCall {
  key: string | null;
  fetcher: () => unknown;
}

/**
 * Answers the hook's two useSWRAuth calls (staff fallback first, schedule
 * second) by position and records each call.
 */
function mockSWR(opts: {
  fallbackStaff?: { id: string };
  staffError?: unknown;
  schedule?: unknown;
  scheduleError?: unknown;
}) {
  const calls: SWRCall[] = [];
  const mutateStaff = vi.fn();
  const mutateSchedule = vi.fn();
  mockUseSWRAuth.mockImplementation(
    (key: string | null, fetcher: () => unknown) => {
      const isStaffCall = calls.length % 2 === 0;
      calls.push({ key, fetcher });
      if (key === null) {
        return {
          data: undefined,
          error: undefined,
          mutate: isStaffCall ? mutateStaff : mutateSchedule,
        };
      }
      return isStaffCall
        ? {
            data: opts.fallbackStaff,
            error: opts.staffError,
            mutate: mutateStaff,
          }
        : {
            data: opts.schedule,
            error: opts.scheduleError,
            mutate: mutateSchedule,
          };
    },
  );
  return {
    staffCall: () => calls.at(-2),
    scheduleCall: () => calls.at(-1),
    mutateStaff,
    mutateSchedule,
  };
}

function contextWith(currentStaff: { id: string } | null, isReady = true) {
  mockUseUserContext.mockReturnValue({
    userContext: isReady
      ? { currentStaff: currentStaff && { ...currentStaff, personId: "1" } }
      : undefined,
    isReady,
  });
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("useOwnStaffSchedule", () => {
  it("requests the schedule with the staff ID from the user context, without asking /api/me/staff", async () => {
    contextWith({ id: "42" });
    const schedule = { entries: [] };
    const swr = mockSWR({ schedule });

    const { result } = renderHook(() => useOwnStaffSchedule());

    // Same render, no prior request: the schedule key is ready at once.
    expect(swr.scheduleCall()?.key).toBe(`${SCHEDULE_PREFIX}42`);
    expect(swr.staffCall()?.key).toBeNull();
    expect(result.current.ownStaffId).toBe("42");
    expect(result.current.schedule).toBe(schedule);
    expect(result.current.error).toBeUndefined();

    await swr.scheduleCall()?.fetcher();
    expect(mockGetSchedule).toHaveBeenCalledWith("42");
    expect(mockGetCurrentStaff).not.toHaveBeenCalled();
  });

  it("waits for the user context before it requests anything", () => {
    contextWith(null, false);
    const swr = mockSWR({});

    const { result } = renderHook(() => useOwnStaffSchedule());

    expect(swr.staffCall()?.key).toBeNull();
    expect(swr.scheduleCall()?.key).toBeNull();
    expect(result.current.ownStaffId).toBeNull();
  });

  it("keeps today's result for an account without a staff record: /api/me/staff answers 404, no schedule is requested", async () => {
    contextWith(null);
    const notLinked = new ApiError("Get current staff failed: 404", 404, {
      code: "general.not_found",
    });
    const swr = mockSWR({ staffError: notLinked });

    const { result } = renderHook(() => useOwnStaffSchedule());

    expect(swr.staffCall()?.key).toBe(STAFF_KEY);
    expect(swr.scheduleCall()?.key).toBeNull();
    expect(result.current.ownStaffId).toBeNull();
    expect(result.current.error).toBe(notLinked);

    await swr.staffCall()?.fetcher();
    expect(mockGetCurrentStaff).toHaveBeenCalledTimes(1);
  });

  it("falls back to /api/me/staff when the context names no staff and uses its ID", () => {
    contextWith(null);
    const swr = mockSWR({ fallbackStaff: { id: "7" } });

    const { result } = renderHook(() => useOwnStaffSchedule());

    expect(swr.staffCall()?.key).toBe(STAFF_KEY);
    expect(swr.scheduleCall()?.key).toBe(`${SCHEDULE_PREFIX}7`);
    expect(result.current.ownStaffId).toBe("7");
  });

  it("reports a failed schedule load and retries both requests", () => {
    contextWith({ id: "42" });
    const failure = new Error("schedule failed");
    const swr = mockSWR({ scheduleError: failure });

    const { result } = renderHook(() => useOwnStaffSchedule());

    expect(result.current.error).toBe(failure);
    result.current.retry();
    expect(swr.mutateStaff).toHaveBeenCalledTimes(1);
    expect(swr.mutateSchedule).toHaveBeenCalledTimes(1);
  });
});
