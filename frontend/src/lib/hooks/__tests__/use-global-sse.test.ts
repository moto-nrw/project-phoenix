/**
 * Tests for use-global-sse.ts hook
 *
 * Tests:
 * - matchesCachePattern helper
 * - invalidateCaches helper
 * - useGlobalSSE hook behavior
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { SSEHookOptions } from "~/lib/sse-types";
import { renderHook } from "@testing-library/react";

// Mock dependencies
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    status: "authenticated",
    data: { user: { token: "test-token", tenantId: 1, roles: ["user"] } },
  })),
}));

vi.mock("swr", () => ({
  mutate: vi.fn(() => Promise.resolve()),
  // useGlobalSSE reads the SWR cache to gate reminder revalidation on the
  // feature-enabled flag; an empty cache means reminders are treated as
  // disabled, which is correct for these (non-reminders) event tests.
  useSWRConfig: vi.fn(() => ({
    cache: new Map(),
    mutate: vi.fn(() => Promise.resolve()),
  })),
}));

vi.mock("~/lib/hooks/use-sse", () => ({
  useSSE: vi.fn(() => ({
    status: "connected",
    isConnected: true,
    error: null,
    reconnectAttempts: 0,
  })),
}));

// Import after mocking
import { useGlobalSSE } from "../use-global-sse";
import { useSSE } from "~/lib/hooks/use-sse";
import { mutate } from "swr";
import { useSession } from "next-auth/react";

describe("useGlobalSSE", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  describe("hook initialization", () => {
    it("returns SSE connection state", () => {
      const { result } = renderHook(() => useGlobalSSE());

      expect(result.current.status).toBe("connected");
      expect(result.current.isConnected).toBe(true);
      expect(result.current.error).toBeNull();
      expect(result.current.reconnectAttempts).toBe(0);
    });

    it("calls useSSE with correct endpoint", () => {
      renderHook(() => useGlobalSSE());

      const expectedOptions: SSEHookOptions = {
        onMessage: expect.any(
          Function,
        ) as unknown as SSEHookOptions["onMessage"],
        enabled: true,
      };

      expect(useSSE).toHaveBeenCalledWith(
        "/api/sse/events",
        expect.objectContaining(expectedOptions),
      );
    });

    it("is disabled when not authenticated", () => {
      vi.mocked(useSession).mockReturnValueOnce({
        status: "unauthenticated",
        data: null,
        update: vi.fn(),
      });

      renderHook(() => useGlobalSSE());

      expect(useSSE).toHaveBeenCalledWith(
        "/api/sse/events",
        expect.objectContaining({
          enabled: false,
        }),
      );
    });

    it("is enabled for admin-only users without staff role", () => {
      vi.mocked(useSession).mockReturnValueOnce({
        status: "authenticated",
        data: {
          user: { token: "test-token", tenantId: 1, roles: ["admin"] },
        } as never,
        update: vi.fn(),
      });

      renderHook(() => useGlobalSSE());

      expect(useSSE).toHaveBeenCalledWith(
        "/api/sse/events",
        expect.objectContaining({
          enabled: true,
        }),
      );
    });

    it("is disabled when session is loading", () => {
      vi.mocked(useSession).mockReturnValueOnce({
        status: "loading",
        data: null,
        update: vi.fn(),
      });

      renderHook(() => useGlobalSSE());

      expect(useSSE).toHaveBeenCalledWith(
        "/api/sse/events",
        expect.objectContaining({
          enabled: false,
        }),
      );
    });

    it("is disabled when authenticated session has no access token", () => {
      vi.mocked(useSession).mockReturnValueOnce({
        status: "authenticated",
        data: {
          user: { tenantId: 1, roles: ["user"] },
        } as never,
        update: vi.fn(),
      });

      renderHook(() => useGlobalSSE());

      expect(useSSE).toHaveBeenCalledWith(
        "/api/sse/events",
        expect.objectContaining({
          enabled: false,
        }),
      );
    });

    it("is disabled for authenticated users without staff or admin roles", () => {
      vi.mocked(useSession).mockReturnValueOnce({
        status: "authenticated",
        data: {
          user: { token: "test-token", tenantId: 1, roles: ["guardian"] },
        } as never,
        update: vi.fn(),
      });

      renderHook(() => useGlobalSSE());

      expect(useSSE).toHaveBeenCalledWith(
        "/api/sse/events",
        expect.objectContaining({
          enabled: false,
        }),
      );
    });

    it("is disabled when authenticated session has no role list", () => {
      vi.mocked(useSession).mockReturnValueOnce({
        status: "authenticated",
        data: {
          user: { token: "test-token", tenantId: 1 },
        } as never,
        update: vi.fn(),
      });

      renderHook(() => useGlobalSSE());

      expect(useSSE).toHaveBeenCalledWith(
        "/api/sse/events",
        expect.objectContaining({
          enabled: false,
        }),
      );
    });

    it("passes tenantId as reconnectKey to useSSE", () => {
      vi.mocked(useSession).mockReturnValueOnce({
        status: "authenticated",
        data: {
          user: { token: "test-token", tenantId: 42, roles: ["user"] },
        } as never,
        update: vi.fn(),
      });

      renderHook(() => useGlobalSSE());

      expect(useSSE).toHaveBeenCalledWith(
        "/api/sse/events",
        expect.objectContaining({
          reconnectKey: 42,
        }),
      );
    });

    it("reconnects SSE when tenantId changes", () => {
      // First render with tenant 1
      vi.mocked(useSession).mockReturnValue({
        status: "authenticated",
        data: {
          user: { token: "test-token", tenantId: 1, roles: ["user"] },
        } as never,
        update: vi.fn(),
      });

      const { rerender } = renderHook(() => useGlobalSSE());

      expect(useSSE).toHaveBeenLastCalledWith(
        "/api/sse/events",
        expect.objectContaining({
          reconnectKey: 1,
        }),
      );

      // Simulate tenant switch to tenant 2
      vi.mocked(useSession).mockReturnValue({
        status: "authenticated",
        data: {
          user: { token: "new-token", tenantId: 2, roles: ["user"] },
        } as never,
        update: vi.fn(),
      });

      rerender();

      expect(useSSE).toHaveBeenLastCalledWith(
        "/api/sse/events",
        expect.objectContaining({
          reconnectKey: 2,
        }),
      );
    });
  });

  describe("event handling", () => {
    it("invalidates dashboard caches on student_checkin event as a fallback", () => {
      renderHook(() => useGlobalSSE());

      // Get the onMessage callback
      const calls = vi.mocked(useSSE).mock.calls as Array<
        [string, SSEHookOptions?]
      >;
      const onMessage = calls[0]?.[1]?.onMessage;
      expect(onMessage).toBeDefined();

      // Simulate event
      onMessage?.({
        type: "student_checkin",
        active_group_id: "123",
        data: {},
        timestamp: new Date().toISOString(),
      });

      // Flush debounce timer
      vi.advanceTimersByTime(500);

      // Should call mutate with pattern matching function
      expect(mutate).toHaveBeenCalled();

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const dashboardCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("active-supervision-dashboard")
        );
      });
      expect(dashboardCall).toBeDefined();
    });

    it("invalidates dashboard caches on student_checkout event as a fallback", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "student_checkout",
        active_group_id: "123",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const dashboardCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("dashboard-analytics")
        );
      });
      expect(dashboardCall).toBeDefined();
    });

    it("invalidates group and per-student caches on bulk_student_checkout event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      // Whole-session end (#848): one event carries every affected student.
      onMessage?.({
        type: "bulk_student_checkout",
        active_group_id: "123",
        data: { student_ids: ["42", "77"] },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;

      // Group (active_group_id) drives supervision-visits invalidation.
      const supervisionCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("tenant:supervision-visits-123")
        );
      });
      expect(supervisionCall).toBeDefined();

      // Every student in the batch gets its detail cache invalidated.
      for (const id of ["42", "77"]) {
        const detailCall = mutateCalls.find((call) => {
          const matcher = call[0];
          return (
            typeof matcher === "function" &&
            (matcher as (key: string) => boolean)(`tenant:student-detail-${id}`)
          );
        });
        expect(detailCall).toBeDefined();
      }
    });

    it("handles a bulk_student_checkout with no active_group_id and no student_ids without invalidating those caches", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      // Defensive path: a malformed/empty bulk event must not throw and must
      // not invalidate group or per-student caches (both guard conditions are
      // false). It still schedules a flush — the dashboard refresh is harmless.
      onMessage?.({
        type: "bulk_student_checkout",
        active_group_id: "",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;

      const touchedSupervisionOrStudent = mutateCalls.some((call) => {
        const matcher = call[0];
        if (typeof matcher !== "function") return false;
        const fn = matcher as (key: string) => boolean;
        return (
          fn("tenant:supervision-visits-123") || fn("tenant:student-detail-42")
        );
      });
      expect(touchedSupervisionOrStudent).toBe(false);
    });

    it("invalidates activity caches on activity_start event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "activity_start",
        active_group_id: "456",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      expect(mutate).toHaveBeenCalled();
    });

    it("invalidates activity caches on activity_end event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "activity_end",
        active_group_id: "456",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      expect(mutate).toHaveBeenCalled();
    });

    it("invalidates activity caches on activity_update event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "activity_update",
        active_group_id: "789",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      expect(mutate).toHaveBeenCalled();
    });

    it("invalidates active supervision caches on active_supervision_changed event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "active_supervision_changed",
        active_group_id: "456",
        data: { reason: "instance_started", student_id: "77" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const activeSupervisionCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)(
            "tenant:active-supervision-dashboard-0",
          ) &&
          (matcher as (key: string) => boolean)(
            "tenant:supervision-visits-456",
          ) &&
          (matcher as (key: string) => boolean)(
            "tenant:timetable-roster-active-group-456",
          )
        );
      });
      expect(activeSupervisionCall).toBeDefined();

      const matcher = activeSupervisionCall![0] as (key: string) => boolean;
      expect(matcher("tenant:supervision-visits-456")).toBe(true);
      expect(matcher("tenant:timetable-roster-123")).toBe(true);
      expect(matcher("tenant:timetable-roster-active-group-456")).toBe(true);
      expect(matcher("tenant:room-detail-12")).toBe(true);
      expect(matcher("tenant:tracking-supervisions-1")).toBe(true);
      expect(matcher("tenant:tracking-indicators-search")).toBe(true);
      expect(matcher("tenant:dashboard")).toBe(true);
      expect(matcher("tenant:timetable-week")).toBe(false);

      const studentDetailCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("tenant:student-detail-77")
        );
      });
      expect(studentDetailCall).toBeDefined();
    });

    it("invalidates dashboard caches on dashboard_counts_changed event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "dashboard_counts_changed",
        active_group_id: "123",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const dashboardCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("active-supervision-dashboard")
        );
      });
      expect(dashboardCall).toBeDefined();
    });

    it("invalidates staff time-account caches after a work-session change", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
      onMessage?.({
        type: "staff_time_tracking_changed",
        active_group_id: "",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const call = vi.mocked(mutate).mock.calls.find(([matcher]) => {
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)(
            "tenant:staff-time-accounts-all--",
          )
        );
      });
      expect(call).toBeDefined();

      const matcher = call![0] as (key: string) => boolean;
      for (const key of [
        "staff-time-accounts-all--",
        "staff-dashboard-summary-month",
        "staff-history-42-2026-07-01-2026-07-31",
        "staff-absences-42-2026-07-01-2026-07-31",
        "staff-pending-absences-42",
        "staff-month-summary-42-2026-7",
        "staff-balance-adjustments-42-2026-01-01-9999-12-31",
        "staff-schedule-42",
        "staff-schedule-targets-42-2026-07-01-2026-07-31",
        "staff-schedule-targets-account-42-2026-01-01-2026-07-31",
        "staff-shifts-visible-42-2026-07-01-2026-07-31",
        "time-tracking-current",
        "time-tracking-history-2026-07-01-2026-07-31",
        "time-tracking-absences-2026-07-01-2026-07-31",
        "time-tracking-table-2026-07-01-2026-07-31",
        "time-tracking-month-summary-2026-7",
        "time-tracking-schedule-targets-2026-07-01-2026-07-31",
        "time-tracking-own-schedule-42",
        "time-tracking-own-shifts-today-2026-07-25",
      ]) {
        expect(matcher(`tenant:${key}`), key).toBe(true);
      }
      expect(matcher("tenant:dashboard")).toBe(false);
      expect(matcher("tenant:staff-list")).toBe(false);
      expect(matcher("tenant:time-tracking-config")).toBe(false);
      expect(matcher("tenant:time-tracking-holidays-2026-07-01")).toBe(false);
    });

    it("student_checkout without active_group_id invalidates ogs-students caches", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      // Daily checkout: has student_id but NO active_group_id
      onMessage?.({
        type: "student_checkout",
        active_group_id: "",
        data: { student_id: "42" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      // Find the mutate call with the ogs-students matcher
      const mutateCalls = vi.mocked(mutate).mock.calls;
      const ogsStudentCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("ogs-students-5")
        );
      });
      expect(ogsStudentCall).toBeDefined();

      // Matcher must work with tenant-prefixed keys (useSWRAuth adds tenant slug prefix)
      const matcher = ogsStudentCall![0] as (key: string) => boolean;
      expect(matcher("my-tenant:ogs-students-5")).toBe(true);
      expect(matcher("my-tenant:rooms-list")).toBe(true);
      expect(matcher("my-tenant:tracking-supervisions-1-42,43")).toBe(true);
      expect(matcher("my-tenant:tracking-indicators-search-group1")).toBe(true);
    });

    it("student_checkout without active_group_id invalidates dashboard caches", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "student_checkout",
        active_group_id: "",
        data: { student_id: "42" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const dashboardCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("active-supervision-dashboard")
        );
      });
      expect(dashboardCall).toBeDefined();
    });

    it("student_checkout without active_group_id invalidates student-detail cache", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "student_checkout",
        active_group_id: "",
        data: { student_id: "42" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const studentDetailCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("student-detail-42")
        );
      });
      expect(studentDetailCall).toBeDefined();

      // Must also match tenant-prefixed key
      const matcher = studentDetailCall![0] as (key: string) => boolean;
      expect(matcher("my-tenant:student-detail-42")).toBe(true);
      // Must NOT match other student IDs
      expect(matcher("student-detail-99")).toBe(false);
    });

    it("student_updated invalidates student list, detail, status-day, and dashboard caches", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "student_updated",
        active_group_id: "",
        data: { source: "manual" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const studentListCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("my-tenant:ogs-students-1")
        );
      });
      expect(studentListCall).toBeDefined();

      const studentListMatcher = studentListCall![0] as (
        key: string,
      ) => boolean;
      expect(studentListMatcher("my-tenant:search-students--")).toBe(true);
      expect(studentListMatcher("my-tenant:database-students-list")).toBe(true);

      const studentDetailCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("my-tenant:student-detail-42")
        );
      });
      expect(studentDetailCall).toBeDefined();

      const studentDetailMatcher = studentDetailCall![0] as (
        key: string,
      ) => boolean;
      expect(
        studentDetailMatcher(
          "my-tenant:student-status-days-42-2026-06-01-2026-06-30",
        ),
      ).toBe(true);

      const dashboardCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("ogs-dashboard")
        );
      });
      expect(dashboardCall).toBeDefined();
    });

    it("handles mutate rejection in student_updated detail scope gracefully", async () => {
      vi.mocked(mutate).mockRejectedValue(new Error("SWR error"));
      const consoleSpy = vi
        .spyOn(console, "debug")
        .mockImplementation(() => undefined);

      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
      onMessage?.({
        type: "student_updated",
        active_group_id: "",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith(
        "swr_revalidation_failed",
        expect.objectContaining({ scope: "student_detail" }),
      );

      consoleSpy.mockRestore();
    });

    it("student_checkout without active_group_id does NOT invalidate supervision-visits", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      // Daily checkout: no active_group_id means pendingGroupIds stays empty
      onMessage?.({
        type: "student_checkout",
        active_group_id: "",
        data: { student_id: "42" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      // supervision-visits should NOT be invalidated (requires pendingGroupIds)
      const mutateCalls = vi.mocked(mutate).mock.calls;
      const supervisionCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("supervision-visits-999")
        );
      });
      expect(supervisionCall).toBeUndefined();
    });

    it("handles mutate rejection for ogs-students gracefully", async () => {
      // Make mutate reject to exercise the .catch() error handler
      vi.mocked(mutate).mockRejectedValue(new Error("SWR error"));

      const consoleSpy = vi
        .spyOn(console, "debug")
        .mockImplementation(() => undefined);

      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      // Daily checkout: has student_id but empty active_group_id
      onMessage?.({
        type: "student_checkout",
        active_group_id: "",
        data: { student_id: "42" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      // Flush microtask queue so .catch() handlers execute
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith(
        "swr_revalidation_failed",
        expect.objectContaining({ scope: "ogs_students" }),
      );

      consoleSpy.mockRestore();
    });

    it("handles mutate rejection for student-detail gracefully", async () => {
      vi.mocked(mutate).mockRejectedValue(new Error("SWR error"));

      const consoleSpy = vi
        .spyOn(console, "debug")
        .mockImplementation(() => undefined);

      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "student_checkout",
        active_group_id: "",
        data: { student_id: "42" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith(
        "swr_revalidation_failed",
        expect.objectContaining({ scope: "student_detail" }),
      );

      consoleSpy.mockRestore();
    });

    it("handles mutate rejection for dashboard gracefully on daily checkout", async () => {
      vi.mocked(mutate).mockRejectedValue(new Error("SWR error"));

      const consoleSpy = vi
        .spyOn(console, "debug")
        .mockImplementation(() => undefined);

      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "student_checkout",
        active_group_id: "",
        data: { student_id: "42" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith(
        "swr_revalidation_failed",
        expect.objectContaining({ scope: "dashboard" }),
      );

      consoleSpy.mockRestore();
    });

    it("invalidates arrival caches on arrival_schedule_changed event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "arrival_schedule_changed",
        active_group_id: "",
        data: { student_id: "42" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const arrivalCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("arrival-search-X")
        );
      });
      expect(arrivalCall).toBeDefined();

      const matcher = arrivalCall![0] as (key: string) => boolean;
      expect(matcher("tenant:arrival-supervisions-1")).toBe(true);
      expect(matcher("tenant:arrival-ogs-groups-1")).toBe(true);
      expect(matcher("tenant:arrival-data-42")).toBe(true);
      expect(matcher("tenant:unrelated-key")).toBe(false);

      const dashboardCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("ogs-dashboard")
        );
      });
      expect(dashboardCall).toBeDefined();
    });

    it("handles mutate rejection in arrival_schedule scope gracefully", async () => {
      vi.mocked(mutate).mockRejectedValue(new Error("SWR error"));

      const consoleSpy = vi
        .spyOn(console, "debug")
        .mockImplementation(() => undefined);

      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
      onMessage?.({
        type: "arrival_schedule_changed",
        active_group_id: "",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith(
        "swr_revalidation_failed",
        expect.objectContaining({ scope: "arrival_schedule" }),
      );

      consoleSpy.mockRestore();
    });

    it("handles mutate rejection in activity_supervision scope gracefully", async () => {
      vi.mocked(mutate).mockRejectedValue(new Error("SWR error"));
      const consoleSpy = vi
        .spyOn(console, "debug")
        .mockImplementation(() => undefined);

      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
      onMessage?.({
        type: "activity_start",
        active_group_id: "1",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith(
        "swr_revalidation_failed",
        expect.objectContaining({ scope: "activity_supervision" }),
      );

      consoleSpy.mockRestore();
    });

    it("handles mutate rejection in active_supervision scope gracefully", async () => {
      vi.mocked(mutate).mockRejectedValue(new Error("SWR error"));
      const consoleSpy = vi
        .spyOn(console, "debug")
        .mockImplementation(() => undefined);

      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
      onMessage?.({
        type: "active_supervision_changed",
        active_group_id: "1",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith(
        "swr_revalidation_failed",
        expect.objectContaining({ scope: "active_supervision" }),
      );

      consoleSpy.mockRestore();
    });

    it("handles mutate rejection in supervision_visits scope gracefully", async () => {
      vi.mocked(mutate).mockRejectedValue(new Error("SWR error"));
      const consoleSpy = vi
        .spyOn(console, "debug")
        .mockImplementation(() => undefined);

      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
      onMessage?.({
        type: "student_checkin",
        active_group_id: "42",
        data: { student_id: "1" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith(
        "swr_revalidation_failed",
        expect.objectContaining({ scope: "supervision_visits" }),
      );

      consoleSpy.mockRestore();
    });

    it("invalidates timetable caches on instance lifecycle events", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "instance_started",
        active_group_id: "",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const timetableCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("tenant:timetable-week")
        );
      });
      expect(timetableCall).toBeDefined();

      const matcher = timetableCall![0] as (key: string) => boolean;
      expect(matcher("tenant:timetable-month-2026-05")).toBe(true);
      expect(matcher("tenant:database-calendar-periods-list")).toBe(true);
      expect(matcher("tenant:supervision-visits-1")).toBe(false);
    });

    it("invalidates timetable + own-assignment caches on staffing_deviation_changed", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "staffing_deviation_changed",
        active_group_id: "",
        data: { source: "deviations" },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const timetableCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("tenant:timetable-week")
        );
      });
      expect(timetableCall).toBeDefined();

      const matcher = timetableCall![0] as (key: string) => boolean;
      expect(
        matcher("tenant:time-tracking-own-assignments-today-2026-07-14"),
      ).toBe(true);
      expect(matcher("tenant:supervision-visits-1")).toBe(false);
    });

    it("handles mutate rejection in timetable scope gracefully", async () => {
      vi.mocked(mutate).mockRejectedValue(new Error("SWR error"));
      const consoleSpy = vi
        .spyOn(console, "debug")
        .mockImplementation(() => undefined);

      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
      onMessage?.({
        type: "instance_completed",
        active_group_id: "",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);
      await vi.runAllTimersAsync();

      expect(consoleSpy).toHaveBeenCalledWith(
        "swr_revalidation_failed",
        expect.objectContaining({ scope: "timetable" }),
      );

      consoleSpy.mockRestore();
    });

    it("debounces rapid events into a single flush", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "student_checkin",
        active_group_id: "42",
        data: { student_id: "1" },
        timestamp: "t",
      });
      onMessage?.({
        type: "student_checkin",
        active_group_id: "42",
        data: { student_id: "2" },
        timestamp: "t",
      });
      onMessage?.({
        type: "student_checkin",
        active_group_id: "42",
        data: { student_id: "3" },
        timestamp: "t",
      });

      // Nothing before the debounce window
      expect(mutate).not.toHaveBeenCalled();

      vi.advanceTimersByTime(500);

      // Single flush triggers invalidations (multiple mutate scopes, but not 3× the count)
      expect(mutate).toHaveBeenCalled();
    });

    it("dispatches phoenix:tenant-settings-stale window event on tenant_settings_changed", () => {
      // Cross-origin tenant settings sync: receiving a tenant_settings_changed
      // SSE event must surface as a window event so the settings-cache bridge
      // can invalidate SWR. We don't mutate SWR directly here — the bridge
      // owns that, this hook only translates SSE → window event.
      renderHook(() => useGlobalSSE());
      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      const dispatched: Event[] = [];
      const listener = (event: Event) => {
        dispatched.push(event);
      };
      window.addEventListener("phoenix:tenant-settings-stale", listener);
      try {
        onMessage?.({
          type: "tenant_settings_changed",
          active_group_id: "",
          data: { source: "operations.student_photos_enabled" },
          timestamp: new Date().toISOString(),
        });

        expect(dispatched).toHaveLength(1);
        expect(dispatched[0]!.type).toBe("phoenix:tenant-settings-stale");
        const detail = (dispatched[0] as CustomEvent).detail as {
          source: string | null;
        };
        expect(detail.source).toBe("operations.student_photos_enabled");
      } finally {
        window.removeEventListener("phoenix:tenant-settings-stale", listener);
      }

      // Tenant invalidations must NOT trigger SWR mutate from this hook —
      // that's the bridge's job. This guard prevents accidental fan-out to
      // unrelated caches.
      vi.advanceTimersByTime(500);
      expect(vi.mocked(mutate)).not.toHaveBeenCalled();
    });

    it("falls back to null source on tenant_settings_changed without a source field", () => {
      renderHook(() => useGlobalSSE());
      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      const dispatched: Event[] = [];
      const listener = (event: Event) => {
        dispatched.push(event);
      };
      window.addEventListener("phoenix:tenant-settings-stale", listener);
      try {
        onMessage?.({
          type: "tenant_settings_changed",
          active_group_id: "",
          data: {},
          timestamp: new Date().toISOString(),
        });

        expect(dispatched).toHaveLength(1);
        const detail = (dispatched[0] as CustomEvent).detail as {
          source: string | null;
        };
        expect(detail.source).toBeNull();
      } finally {
        window.removeEventListener("phoenix:tenant-settings-stale", listener);
      }
    });

    it("silently ignores unknown event types without invalidating caches", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      // Unknown events should be silently ignored (no cache invalidation)
      onMessage?.({
        type: "unknown_event" as never,
        active_group_id: "123",
        data: {},
        timestamp: new Date().toISOString(),
      });

      // Advance past debounce window to confirm nothing fires
      vi.advanceTimersByTime(500);

      // Verify no caches were invalidated for unknown event type
      expect(vi.mocked(mutate)).not.toHaveBeenCalled();
    });

    // tenant_settings_changed crosses origins (operator subdomain ↔ tenant
    // subdomain) — BroadcastChannel can't deliver across origins, only SSE
    // can. The hook dispatches a window event so TenantProvider (which owns
    // serialised resolveTenant refetches) can respond without this hook
    // importing tenant internals.
    it("dispatches phoenix:tenant-settings-stale on tenant_settings_changed event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      const dispatched: CustomEvent[] = [];
      const handler = (e: Event) => {
        dispatched.push(e as CustomEvent);
      };
      window.addEventListener("phoenix:tenant-settings-stale", handler);

      onMessage?.({
        type: "tenant_settings_changed" as never,
        active_group_id: "",
        data: { source: "tenant_photos_disabled" },
        timestamp: new Date().toISOString(),
      });

      window.removeEventListener("phoenix:tenant-settings-stale", handler);

      expect(dispatched).toHaveLength(1);
      expect(dispatched[0]?.detail).toEqual({
        source: "tenant_photos_disabled",
      });
    });

    it("falls back to null source when tenant_settings_changed omits it", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      const dispatched: CustomEvent[] = [];
      const handler = (e: Event) => {
        dispatched.push(e as CustomEvent);
      };
      window.addEventListener("phoenix:tenant-settings-stale", handler);

      onMessage?.({
        type: "tenant_settings_changed" as never,
        active_group_id: "",
        data: {},
        timestamp: new Date().toISOString(),
      });

      window.removeEventListener("phoenix:tenant-settings-stale", handler);

      expect(dispatched).toHaveLength(1);
      expect(dispatched[0]?.detail).toEqual({ source: null });
    });
  });
});
