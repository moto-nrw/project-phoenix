/**
 * Tests for use-global-sse.ts hook
 *
 * Tests:
 * - matchesCachePattern helper
 * - invalidateCaches helper
 * - useGlobalSSE hook behavior
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { SSEEvent, SSEHookOptions, SSEHookState } from "~/lib/sse-types";
import { act, renderHook } from "@testing-library/react";

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

const mockMinuteClock = vi.hoisted(() => ({
  current: new Date("2026-01-01T22:59:00Z"),
}));

vi.mock("~/lib/pickup-helpers", () => ({
  useMinuteClock: () => mockMinuteClock.current,
}));

// Import after mocking
import {
  useGlobalSSE,
  DEBOUNCE_MS,
  FLUSH_JITTER_MS,
  MAX_FLUSH_WAIT_MS,
} from "../use-global-sse";
import { useSSE } from "~/lib/hooks/use-sse";
import { mutate } from "swr";
import { useSession } from "next-auth/react";
import {
  clearOwnAttendanceMutation,
  isOwnAttendanceEvent,
  markOwnAttendanceMutation,
} from "~/lib/sse-optimistic-mutations";

describe("useGlobalSSE", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    mockMinuteClock.current = new Date("2026-01-01T22:59:00Z");
    // Pin the per-burst flush jitter (#2057) to 0 so the existing
    // advanceTimersByTime(500) timing assertions stay deterministic. Jitter
    // bounds get their own explicit tests.
    vi.spyOn(Math, "random").mockReturnValue(0);
  });

  afterEach(() => {
    window.history.replaceState({}, "", "/");
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  /** Fires an event through the captured onMessage callback. */
  function fire(event: {
    type: SSEEvent["type"];
    active_group_id?: string;
    data?: SSEEvent["data"];
  }) {
    const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
    onMessage?.({
      type: event.type,
      active_group_id: event.active_group_id ?? "",
      data: event.data ?? {},
      timestamp: new Date().toISOString(),
    });
  }

  /** Every key from the inventory that ANY recorded mutate matcher accepts. */
  function matchedKeys(inventory: readonly string[]): string[] {
    const matchers = vi
      .mocked(mutate)
      .mock.calls.map((call) => call[0])
      .filter((m): m is (key: string) => boolean => typeof m === "function");
    return inventory.filter((key) => matchers.some((m) => m(key)));
  }

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

    it("revalidates the supervision dashboard after an SSE reconnect", () => {
      let connectionState: SSEHookState = {
        status: "connected",
        isConnected: true,
        error: null,
        reconnectAttempts: 0,
      };
      vi.mocked(useSSE).mockImplementation(() => connectionState);

      const { rerender } = renderHook(() => useGlobalSSE());
      vi.mocked(mutate).mockClear();

      connectionState = {
        status: "reconnecting",
        isConnected: false,
        error: "SSE-Verbindungsfehler",
        reconnectAttempts: 1,
      };
      rerender();

      connectionState = {
        status: "connected",
        isConnected: true,
        error: null,
        reconnectAttempts: 0,
      };
      rerender();

      expect(
        matchedKeys([
          "tenant:active-supervision-dashboard-0-224",
          "tenant:timetable-week-2026-09-01",
        ]),
      ).toEqual(["tenant:active-supervision-dashboard-0-224"]);
    });
  });

  describe("event handling", () => {
    it("load budget: one remote check-in revalidates the supervision aggregate once", () => {
      renderHook(() => useGlobalSSE());

      // Current backends emit the scoped movement and one combined tenant
      // refresh (#2115).
      fire({
        type: "student_checkin",
        active_group_id: "123",
        data: { student_id: "42", group_ids: ["7"] },
      });
      fire({
        type: "dashboard_counts_changed",
        active_group_id: "123",
        data: { reason: "student_moved", group_ids: ["7"] },
      });

      vi.advanceTimersByTime(DEBOUNCE_MS);

      const aggregateRevalidations = vi
        .mocked(mutate)
        .mock.calls.filter(([matcher]) => {
          return (
            typeof matcher === "function" &&
            (matcher as (key: string) => boolean)(
              "tenant:active-supervision-dashboard-0",
            )
          );
        });

      expect(aggregateRevalidations).toHaveLength(1);
    });

    it("keeps the optimistic roster response for the user's own check-in", () => {
      markOwnAttendanceMutation("student_checkin", "42");
      renderHook(() => useGlobalSSE());

      fire({
        type: "student_checkin",
        active_group_id: "123",
        data: { student_id: "42", group_ids: ["7"] },
      });
      fire({
        type: "dashboard_counts_changed",
        active_group_id: "123",
        data: { reason: "student_moved", group_ids: ["7"] },
      });
      vi.advanceTimersByTime(DEBOUNCE_MS);

      const matchers = vi
        .mocked(mutate)
        .mock.calls.map(([matcher]) => matcher)
        .filter(
          (matcher): matcher is (key: string) => boolean =>
            typeof matcher === "function",
        );
      expect(
        matchers.some((matcher) => matcher("tenant:timetable-roster-88")),
      ).toBe(false);
      expect(
        matchers.filter((matcher) =>
          matcher("tenant:active-supervision-dashboard-0"),
        ),
      ).toHaveLength(1);
      expect(isOwnAttendanceEvent("student_checkin", "42")).toBe(false);

      clearOwnAttendanceMutation("student_checkin", "42");
    });

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
          (matcher as (key: string) => boolean)(
            "tenant:active-supervision-dashboard-0",
          )
        );
      });
      expect(dashboardCall).toBeDefined();

      // #2057: the count-dashboard invalidation is an explicit key list — it
      // must NOT drag the OGS BFF or staff time tracking into every check-in.
      const dashboardAnalyticsCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("tenant:dashboard-analytics")
        );
      });
      expect(dashboardAnalyticsCall).toBeDefined();
      const matcher = dashboardAnalyticsCall![0] as (key: string) => boolean;
      expect(matcher("tenant:ogs-dashboard")).toBe(false);
      expect(matcher("tenant:staff-dashboard-summary-month")).toBe(false);
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

    it("invalidates group and per-student caches on bulk_student_checkin event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "bulk_student_checkin",
        active_group_id: "123",
        data: { student_ids: ["42", "77"], group_ids: ["7"] },
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      const mutateCalls = vi.mocked(mutate).mock.calls;
      const supervisionCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)(
            "tenant:active-supervision-dashboard-0",
          )
        );
      });
      expect(supervisionCall).toBeDefined();

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

      const ogsCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("tenant:ogs-students-7") &&
          !(matcher as (key: string) => boolean)("tenant:ogs-students-8")
        );
      });
      expect(ogsCall).toBeDefined();
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
          (matcher as (key: string) => boolean)(
            "tenant:active-supervision-dashboard-0",
          )
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

      const touchedStudentDetail = mutateCalls.some((call) => {
        const matcher = call[0];
        if (typeof matcher !== "function") return false;
        return (matcher as (key: string) => boolean)(
          "tenant:student-detail-42",
        );
      });
      expect(touchedStudentDetail).toBe(false);
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
            "tenant:timetable-roster-active-group-456",
          )
        );
      });
      expect(activeSupervisionCall).toBeDefined();

      const matcher = activeSupervisionCall![0] as (key: string) => boolean;
      // Per-room visits and tracking ride in the aggregate since #2096 —
      // the dashboard key stands in for the removed supervision-visits- and
      // tracking-supervisions- keys.
      expect(matcher("tenant:active-supervision-dashboard-0")).toBe(true);
      expect(matcher("tenant:timetable-roster-123")).toBe(true);
      expect(matcher("tenant:timetable-roster-active-group-456")).toBe(true);
      expect(matcher("tenant:room-detail-12")).toBe(true);
      expect(matcher("tenant:tracking-indicators-search")).toBe(true);
      // #2057: no bare "dashboard" substring any more — dashboard-analytics
      // is covered by the count-dashboard block, and the substring dragged
      // ogs-dashboard + staff-dashboard-summary into every check-in.
      expect(matcher("tenant:dashboard")).toBe(false);
      expect(matcher("tenant:ogs-dashboard")).toBe(false);
      expect(matcher("tenant:timetable-week")).toBe(false);

      // active_supervision_changed alone must NOT invalidate the per-group
      // OGS student lists: it fires tenant-wide alongside every scoped
      // dashboard_counts_changed, and honoring it would re-broaden every
      // scoped event (#2057).
      const ogsStudentsCall = mutateCalls.find((call) => {
        const m = call[0];
        return (
          typeof m === "function" &&
          (m as (key: string) => boolean)("tenant:ogs-students-5")
        );
      });
      expect(ogsStudentsCall).toBeUndefined();

      // #2085: the tenant-wide event carries no child id any more, and the
      // hook must not act on one even if a stale backend still sends it —
      // honoring it would re-open the leak's client half (a colleague outside
      // gdpr.student_data_scope refetching that child's detail cache). The
      // group-scoped student_checkin/checkout events own that invalidation.
      const studentDetailCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("tenant:student-detail-77")
        );
      });
      expect(studentDetailCall).toBeUndefined();
    });

    it("still invalidates the child detail cache from the group-scoped student_checkin (#2085)", () => {
      renderHook(() => useGlobalSSE());

      // What an entitled supervisor's client actually receives: the scoped
      // check-in on the active-group / edu:{id} topic, alongside the id-less
      // tenant-wide refresh.
      fire({
        type: "student_checkin",
        active_group_id: "456",
        data: { student_id: "77", group_ids: ["5"] },
      });
      fire({
        type: "active_supervision_changed",
        active_group_id: "456",
        data: { reason: "student_moved" },
      });

      vi.advanceTimersByTime(500);

      expect(matchedKeys(["tenant:student-detail-77"])).toEqual([
        "tenant:student-detail-77",
      ]);
    });

    it("revalidates only the already-open child on an id-less tenant refresh", () => {
      window.history.replaceState({}, "", "/tenant/students/88?tab=care-plan");
      renderHook(() => useGlobalSSE());

      // A mixed-version backend may still send student_id. The tenant-wide
      // payload must not choose the cache: only the viewer's current,
      // access-checked detail route may do that.
      fire({
        type: "active_supervision_changed",
        active_group_id: "456",
        data: { reason: "timetable_attendance_updated", student_id: "77" },
      });

      vi.advanceTimersByTime(500);

      expect(
        matchedKeys([
          "tenant:student-detail-77",
          "tenant:care-plan-day-77-2026-08-02",
          "tenant:student-detail-88",
          "tenant:care-plan-day-88-2026-08-02",
          "tenant:care-plan-week-88-2026-08-03-2026-08-07",
        ]),
      ).toEqual([
        "tenant:student-detail-88",
        "tenant:care-plan-day-88-2026-08-02",
        "tenant:care-plan-week-88-2026-08-03-2026-08-07",
      ]);
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
          (matcher as (key: string) => boolean)(
            "tenant:active-supervision-dashboard-0",
          )
        );
      });
      expect(dashboardCall).toBeDefined();

      const analyticsCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("tenant:dashboard-analytics")
        );
      });
      expect(analyticsCall).toBeDefined();
      const matcher = analyticsCall![0] as (key: string) => boolean;
      // #2057: the OGS BFF must not ride the count events — its live data is
      // covered by the (scoped) ogs-students refetch.
      expect(matcher("tenant:ogs-dashboard")).toBe(false);
      expect(matcher("tenant:staff-dashboard-summary-month")).toBe(false);
    });

    it("treats a reasoned dashboard event as the combined supervision refresh", () => {
      renderHook(() => useGlobalSSE());

      fire({
        type: "dashboard_counts_changed",
        active_group_id: "123",
        data: { reason: "student_moved", group_ids: ["7"] },
      });
      vi.advanceTimersByTime(DEBOUNCE_MS);

      expect(
        matchedKeys([
          "tenant:active-supervision-dashboard-0",
          "tenant:room-detail-12",
          "tenant:ogs-students-7",
          "tenant:ogs-students-8",
        ]),
      ).toEqual([
        "tenant:active-supervision-dashboard-0",
        "tenant:room-detail-12",
        "tenant:ogs-students-7",
      ]);
    });

    it("keeps the dashboard broad fallback on an unscoped combined refresh", () => {
      renderHook(() => useGlobalSSE());

      fire({
        type: "dashboard_counts_changed",
        active_group_id: "123",
        data: { reason: "student_moved" },
      });
      vi.advanceTimersByTime(DEBOUNCE_MS);

      expect(
        matchedKeys([
          "tenant:ogs-students-7",
          "tenant:search-students-gall-name-asc",
          "tenant:room-detail-12",
        ]),
      ).toEqual([
        "tenant:ogs-students-7",
        "tenant:search-students-gall-name-asc",
        "tenant:room-detail-12",
      ]);
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
        "staff-month-close-2026-7",
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

      // Find the mutate call with the ogs-students matcher. The event carries
      // no group_ids (old backend / student without OGS group), so the broad
      // fallback must invalidate every group's list (#2057).
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

      // The cross-group list caches live in their own mutate call since the
      // ogs-students split (#2057) — same trigger, separate matcher.
      const listCall = mutateCalls.find((call) => {
        const m = call[0];
        return (
          typeof m === "function" &&
          (m as (key: string) => boolean)("my-tenant:rooms-list")
        );
      });
      expect(listCall).toBeDefined();
      const trackingCall = mutateCalls.find((call) => {
        const m = call[0];
        return (
          typeof m === "function" &&
          (m as (key: string) => boolean)(
            "my-tenant:tracking-indicators-search-group1",
          )
        );
      });
      expect(trackingCall).toBeDefined();
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
          (matcher as (key: string) => boolean)(
            "tenant:active-supervision-dashboard-0",
          )
        );
      });
      expect(dashboardCall).toBeDefined();

      const analyticsCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("tenant:dashboard-analytics")
        );
      });
      expect(analyticsCall).toBeDefined();
      const matcher = analyticsCall![0] as (key: string) => boolean;
      expect(matcher("tenant:ogs-dashboard")).toBe(false);
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

      // The cross-group list caches live in their own mutate call since the
      // ogs-students split (#2057) — same trigger, separate matcher. The
      // Kindersuche got its own matcher on top with the group scoping (#2097),
      // so the two families are looked up separately here.
      const searchListCall = mutateCalls.find((call) => {
        const m = call[0];
        return (
          typeof m === "function" &&
          (m as (key: string) => boolean)("my-tenant:search-students-gall-")
        );
      });
      expect(searchListCall).toBeDefined();

      const databaseListCall = mutateCalls.find((call) => {
        const m = call[0];
        return (
          typeof m === "function" &&
          (m as (key: string) => boolean)("my-tenant:database-students-list")
        );
      });
      expect(databaseListCall).toBeDefined();

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

      // The former ogs-dashboard BFF key is gone (#2056): the aggregated OGS
      // live view rides the ogs-students-{gid} keys, so no matcher may target
      // the retired key any more.
      const dashboardCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("ogs-dashboard")
        );
      });
      expect(dashboardCall).toBeUndefined();
    });

    it("pickup and student_updated events refresh the timetable attendance caches (#2360)", () => {
      // A pulled-forward day pickup time rewrites timetable attendance rows
      // and the early_pickup_time marker. The planner list and the
      // Web-Anwesenheit roster disable focus revalidation, so these events
      // are their only live update path — while the template/phase/staff
      // caches under the same "timetable-" prefix must stay untouched.
      const attendanceKeys = [
        "my-tenant:timetable-week-2026-08-17-2026-08-21",
        "my-tenant:timetable-day-2026-08-17-2026-08-17",
        "my-tenant:timetable-roster-42",
      ] as const;
      const unrelatedTimetableKeys = [
        "my-tenant:timetable-templates-7",
        "my-tenant:timetable-enrollment-phases",
        "my-tenant:timetable-staff-list",
      ] as const;

      renderHook(() => useGlobalSSE());

      fire({ type: "pickup_schedule_changed" });
      vi.advanceTimersByTime(500);
      expect(matchedKeys(attendanceKeys)).toEqual([...attendanceKeys]);
      expect(matchedKeys(unrelatedTimetableKeys)).toEqual([]);

      vi.mocked(mutate).mockClear();
      fire({ type: "student_updated" });
      vi.advanceTimersByTime(500);
      expect(matchedKeys(attendanceKeys)).toEqual([...attendanceKeys]);
      expect(matchedKeys(unrelatedTimetableKeys)).toEqual([]);
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
          (matcher as (key: string) => boolean)("tenant:arrival-data-42")
        );
      });
      expect(arrivalCall).toBeDefined();

      const matcher = arrivalCall![0] as (key: string) => boolean;
      // The Aufsicht arrival rows ride in the aggregate since #2096; the
      // dashboard key is refreshed by the count-dashboard block on the same
      // event, so this list only carries the remaining arrival caches.
      expect(matcher("tenant:arrival-data-42")).toBe(true);
      expect(matcher("tenant:unrelated-key")).toBe(false);

      // Arrival changes reach the OGS page through the broad ogs-students
      // trigger; the retired ogs-dashboard key must no longer be targeted
      // (#2056).
      const dashboardCall = mutateCalls.find((call) => {
        const matcher = call[0];
        return (
          typeof matcher === "function" &&
          (matcher as (key: string) => boolean)("ogs-dashboard")
        );
      });
      expect(dashboardCall).toBeUndefined();
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

    it("handles activity-triggered supervision rejection gracefully", async () => {
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
        expect.objectContaining({ scope: "active_supervision" }),
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

    // The former supervision_visits mutate scope was removed with #2096:
    // per-room visits ride in the aggregated dashboard key, whose rejection
    // path is covered by the dashboard-scope test.

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

    it("dispatches the complete notification payload", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
      const listener = vi.fn();
      window.addEventListener("phoenix:notification", listener);

      try {
        onMessage?.({
          type: "notification",
          active_group_id: "",
          data: {
            title: "Abholzeit geändert",
            body: "Die neue Abholzeit ist verfügbar.",
            deep_link: "/reminders",
            priority: "high",
            notification_type: "pickup_changed",
            notification_data: { reminder_id: "123" },
          },
          timestamp: new Date().toISOString(),
        });

        expect(listener).toHaveBeenCalledOnce();
        const dispatchedEvent = listener.mock.calls[0]?.[0] as CustomEvent;
        expect(dispatchedEvent.detail).toEqual({
          title: "Abholzeit geändert",
          body: "Die neue Abholzeit ist verfügbar.",
          deepLink: "/reminders",
          priority: "high",
          notificationType: "pickup_changed",
          data: { reminder_id: "123" },
        });
      } finally {
        window.removeEventListener("phoenix:notification", listener);
      }
    });

    it("uses null for omitted optional notification fields", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;
      const listener = vi.fn();
      window.addEventListener("phoenix:notification", listener);

      try {
        onMessage?.({
          type: "notification",
          active_group_id: "",
          data: {},
          timestamp: new Date().toISOString(),
        });

        expect(listener).toHaveBeenCalledOnce();
        const dispatchedEvent = listener.mock.calls[0]?.[0] as CustomEvent;
        expect(dispatchedEvent.detail).toEqual({
          title: null,
          body: null,
          deepLink: null,
          priority: null,
          notificationType: null,
          data: null,
        });
      } finally {
        window.removeEventListener("phoenix:notification", listener);
      }
    });
  });

  // #2057: scoped invalidation, jitter, and the request-herd regression guard.
  describe("scoped OGS invalidation (#2057)", () => {
    it("scopes ogs-students invalidation to the event's group_ids", () => {
      renderHook(() => useGlobalSSE());

      fire({
        type: "dashboard_counts_changed",
        data: { group_ids: ["7"] },
      });
      vi.advanceTimersByTime(500);

      const keys = matchedKeys([
        "tenant:ogs-students-7",
        "tenant:ogs-students-8",
        // Exact-boundary matching: gid "7" must not hit group 70.
        "tenant:ogs-students-70",
        "tenant:ogs-dashboard",
        "tenant:staff-dashboard-summary-month",
        "tenant:dashboard-analytics",
      ]);
      expect(keys).toEqual([
        "tenant:ogs-students-7",
        "tenant:dashboard-analytics",
      ]);
    });

    it("falls back to broad ogs-students invalidation when group_ids are absent", () => {
      // Mixed-version safety: an old backend (or a student without an OGS
      // group) sends no group_ids — every group's list must still refresh.
      renderHook(() => useGlobalSSE());

      fire({ type: "dashboard_counts_changed" });
      vi.advanceTimersByTime(500);

      const keys = matchedKeys([
        "tenant:ogs-students-7",
        "tenant:ogs-students-8",
        "tenant:ogs-dashboard",
      ]);
      expect(keys).toEqual(["tenant:ogs-students-7", "tenant:ogs-students-8"]);
    });

    it("keeps the backpressure fallback scoped: student_checkin with group_ids", () => {
      // dashboard_counts_changed is best-effort; a delivered scoped
      // student_checkin may be the only signal — it must invalidate the
      // affected group without re-broadening.
      renderHook(() => useGlobalSSE());

      fire({
        type: "student_checkin",
        active_group_id: "123",
        data: { student_id: "42", group_ids: ["7"] },
      });
      vi.advanceTimersByTime(500);

      const keys = matchedKeys([
        "tenant:ogs-students-7",
        "tenant:ogs-students-8",
        "tenant:ogs-dashboard",
        "tenant:student-detail-42",
      ]);
      expect(keys).toEqual([
        "tenant:ogs-students-7",
        "tenant:student-detail-42",
      ]);
    });

    it("student_checkin without group_ids falls back to broad ogs-students invalidation", () => {
      renderHook(() => useGlobalSSE());

      fire({
        type: "student_checkin",
        active_group_id: "123",
        data: { student_id: "42" },
      });
      vi.advanceTimersByTime(500);

      const keys = matchedKeys([
        "tenant:ogs-students-7",
        "tenant:ogs-students-8",
        "tenant:ogs-dashboard",
      ]);
      expect(keys).toEqual(["tenant:ogs-students-7", "tenant:ogs-students-8"]);
    });

    it("bulk_student_checkout scopes to every carried group id", () => {
      renderHook(() => useGlobalSSE());

      fire({
        type: "bulk_student_checkout",
        active_group_id: "123",
        data: { student_ids: ["42", "77"], group_ids: ["7", "9"] },
      });
      vi.advanceTimersByTime(500);

      const keys = matchedKeys([
        "tenant:ogs-students-7",
        "tenant:ogs-students-8",
        "tenant:ogs-students-9",
        "tenant:ogs-dashboard",
      ]);
      expect(keys).toEqual(["tenant:ogs-students-7", "tenant:ogs-students-9"]);
    });

    it("activity lifecycle events refresh ogs-students broadly", () => {
      // Session lifecycle changes current_location across groups but carries
      // no educational scope. The former ogs-dashboard BFF key is retired
      // (#2056) — the aggregated view lives under ogs-students-{gid}.
      renderHook(() => useGlobalSSE());

      fire({ type: "activity_end", active_group_id: "456" });
      vi.advanceTimersByTime(500);

      const keys = matchedKeys([
        "tenant:ogs-students-7",
        "tenant:ogs-students-8",
        "tenant:ogs-dashboard",
      ]);
      expect(keys).toEqual(["tenant:ogs-students-7", "tenant:ogs-students-8"]);
    });

    it("request-budget guard: one scoped event touches nothing on a tab viewing another group", () => {
      // The herd regression guard for the acceptance criteria: an OGS tab
      // viewing group 8 while group 7's student checks in must refetch NOTHING
      // (previously: 9 Go-API calls via ogs-dashboard + ogs-students-8).
      renderHook(() => useGlobalSSE());

      fire({
        type: "dashboard_counts_changed",
        data: { group_ids: ["7"] },
      });
      fire({
        type: "active_supervision_changed",
        active_group_id: "123",
        data: { reason: "student_moved", student_id: "42" },
      });
      vi.advanceTimersByTime(500);

      // Mounted-key inventory of an OGS tab viewing group 8.
      const keys = matchedKeys([
        "tenant:ogs-dashboard",
        "tenant:ogs-students-8",
      ]);
      expect(keys).toEqual([]);
    });

    it("spreads the flush by the per-burst jitter", () => {
      vi.spyOn(Math, "random").mockReturnValue(0.999);
      renderHook(() => useGlobalSSE());

      fire({ type: "dashboard_counts_changed", data: { group_ids: ["7"] } });

      // Not at the bare debounce...
      vi.advanceTimersByTime(DEBOUNCE_MS);
      expect(mutate).not.toHaveBeenCalled();

      // ...but within DEBOUNCE_MS + FLUSH_JITTER_MS.
      vi.advanceTimersByTime(FLUSH_JITTER_MS);
      expect(mutate).toHaveBeenCalled();
    });

    it("collapses a jittered burst into a single flush", () => {
      vi.spyOn(Math, "random").mockReturnValue(0.5);
      renderHook(() => useGlobalSSE());

      for (let i = 0; i < 5; i++) {
        fire({ type: "dashboard_counts_changed", data: { group_ids: ["7"] } });
        vi.advanceTimersByTime(100);
      }
      expect(mutate).not.toHaveBeenCalled();

      vi.advanceTimersByTime(DEBOUNCE_MS + FLUSH_JITTER_MS);
      const flushes = vi
        .mocked(mutate)
        .mock.calls.filter(
          (call) =>
            typeof call[0] === "function" &&
            (call[0] as (key: string) => boolean)("tenant:ogs-students-7"),
        );
      expect(flushes).toHaveLength(1);
    });

    it("caps a sustained event stream at MAX_FLUSH_WAIT_MS", () => {
      // A morning check-in rush must not starve the flush: the trailing
      // debounce keeps resetting, but the first flush arrives no later than
      // MAX_FLUSH_WAIT_MS after the burst began.
      renderHook(() => useGlobalSSE());

      // Events every 400ms past the cap — each re-arms the debounce, so an
      // uncapped implementation would not have flushed by MAX_FLUSH_WAIT_MS.
      const step = 400;
      const steps = Math.ceil(MAX_FLUSH_WAIT_MS / step) + 1;
      for (let i = 0; i < steps; i++) {
        fire({ type: "dashboard_counts_changed", data: { group_ids: ["7"] } });
        vi.advanceTimersByTime(step);
      }
      // Elapsed > MAX_FLUSH_WAIT_MS: the capped flush has fired.
      expect(mutate).toHaveBeenCalled();
    });
  });

  // #2097: the Kindersuche list rides the same group_ids scope. Its keys carry
  // the page's group filter as a leading "g{id}"/"gall" segment, so a view
  // filtered to another group can be skipped.
  describe("scoped Kindersuche invalidation (#2097)", () => {
    // A tab filtered to group 8, a tab filtered to group 80 (boundary probe),
    // and an unfiltered "Alle Gruppen" tab. Search terms may contain dashes,
    // which is exactly why the scope segment leads the key.
    const searchKeys = [
      "tenant:search-students-g7-Anna-Lena--------",
      "tenant:search-students-g8---------",
      "tenant:search-students-g80---------",
      "tenant:search-students-gall---------",
    ];

    it("scopes the Kindersuche list to the event's group_ids", () => {
      renderHook(() => useGlobalSSE());

      fire({ type: "dashboard_counts_changed", data: { group_ids: ["7"] } });
      vi.advanceTimersByTime(500);

      // The group-7 view refetches; group 8 and the boundary-probe group 80 do
      // not. The unfiltered view renders every child of the school with a live
      // location badge, so it still refetches.
      expect(matchedKeys(searchKeys)).toEqual([
        "tenant:search-students-g7-Anna-Lena--------",
        "tenant:search-students-gall---------",
      ]);
    });

    it("falls back to a broad Kindersuche refresh when group_ids are absent", () => {
      renderHook(() => useGlobalSSE());

      fire({ type: "dashboard_counts_changed" });
      vi.advanceTimersByTime(500);

      expect(matchedKeys(searchKeys)).toEqual(searchKeys);
    });

    it("refreshes only the unfiltered search for an unscoped movement", () => {
      renderHook(() => useGlobalSSE());

      fire({
        type: "active_supervision_changed",
        active_group_id: "123",
        data: { reason: "student_moved" },
      });
      vi.advanceTimersByTime(500);

      expect(matchedKeys(searchKeys)).toEqual([
        "tenant:search-students-gall---------",
      ]);
    });

    it("keeps tenant-wide plan changes broad", () => {
      // arrival/pickup/student_updated carry no group scope and rewrite the
      // times the list rows render.
      renderHook(() => useGlobalSSE());

      fire({ type: "pickup_schedule_changed" });
      vi.advanceTimersByTime(500);

      expect(matchedKeys(searchKeys)).toEqual(searchKeys);
    });

    it("refreshes every Kindersuche view on a group-access change", () => {
      // A handover or leader change rewrites the viewer's own has_full_access
      // for whole groups. It carries no group id and the affected tab is keyed
      // on a different group than the changed one, so this one stays broad.
      // Before #2097 it rode along on any check-in in the school; the scoping
      // removed that crutch, so it needs its own trigger.
      renderHook(() => useGlobalSSE());

      fire({
        type: "group_access_changed",
        data: { source: "group_transfer" },
      });
      vi.advanceTimersByTime(500);

      expect(matchedKeys(searchKeys)).toEqual(searchKeys);
    });

    it("request-budget guard: a foreign group's check-in leaves a filtered Kindersuche tab alone", () => {
      // The acceptance criterion of #2097: a check-in in group 7 (with the
      // tenant-wide active_supervision_changed that always accompanies it)
      // must not refetch a Kindersuche tab filtered to group 8.
      renderHook(() => useGlobalSSE());

      fire({
        type: "student_checkin",
        active_group_id: "123",
        data: { student_id: "42", group_ids: ["7"] },
      });
      fire({
        type: "active_supervision_changed",
        active_group_id: "123",
        data: { reason: "student_moved" },
      });
      vi.advanceTimersByTime(500);

      expect(matchedKeys(["tenant:search-students-g8---------"])).toEqual([]);
    });
  });

  describe("group_access_changed (#2084)", () => {
    it("reconnects once per group-access burst", () => {
      renderHook(() => useGlobalSSE());

      const initialReconnectKey = vi
        .mocked(useSSE)
        .mock.calls.at(-1)?.[1]?.reconnectKey;
      expect(initialReconnectKey).toBeDefined();

      act(() => {
        for (let i = 0; i < 3; i++) {
          fire({
            type: "group_access_changed",
            data: { source: "group_transfer" },
          });
        }
      });

      expect(vi.mocked(useSSE).mock.calls.at(-1)?.[1]?.reconnectKey).toBe(
        initialReconnectKey,
      );

      act(() => vi.advanceTimersByTime(500));

      expect(vi.mocked(useSSE).mock.calls.at(-1)?.[1]?.reconnectKey).toBe(
        `${initialReconnectKey}:1`,
      );
    });

    it("reconnects and refreshes access caches after a Berlin-day boundary", () => {
      const supervisionListener = vi.fn();
      window.addEventListener("phoenix:supervision-stale", supervisionListener);
      const { rerender } = renderHook(() => useGlobalSSE());

      const initialReconnectKey = vi
        .mocked(useSSE)
        .mock.calls.at(-1)?.[1]?.reconnectKey;
      expect(initialReconnectKey).toBeDefined();

      mockMinuteClock.current = new Date("2026-01-01T23:01:00Z");
      act(() => {
        rerender();
      });

      expect(vi.mocked(useSSE).mock.calls.at(-1)?.[1]?.reconnectKey).toBe(
        initialReconnectKey,
      );

      act(() => vi.advanceTimersByTime(500));

      expect(vi.mocked(useSSE).mock.calls.at(-1)?.[1]?.reconnectKey).toBe(
        `${initialReconnectKey}:1`,
      );
      expect(
        matchedKeys([
          "tenant:active-substitutions",
          "tenant:group-handovers",
          "tenant:substitution-groups",
          "tenant:substitution-teachers",
          "tenant:user-context",
        ]),
      ).toEqual([
        "tenant:active-substitutions",
        "tenant:group-handovers",
        "tenant:substitution-groups",
        "tenant:substitution-teachers",
        "tenant:user-context",
      ]);
      expect(supervisionListener).toHaveBeenCalledTimes(1);
      window.removeEventListener(
        "phoenix:supervision-stale",
        supervisionListener,
      );
    });

    it("refreshes EVERY ogs-students key, not just the transferred group", () => {
      // The acceptance criterion: a group handed to this user must appear in
      // their open tab. That tab is keyed on the group they were ALREADY
      // viewing (or "auto" when they had none), so a scoped invalidation would
      // miss exactly the client the event exists for.
      renderHook(() => useGlobalSSE());

      fire({
        type: "group_access_changed",
        data: { source: "group_transfer" },
      });
      vi.advanceTimersByTime(500);

      expect(
        matchedKeys([
          "tenant:ogs-students-7",
          "tenant:ogs-students-8",
          "tenant:ogs-students-auto",
        ]),
      ).toEqual([
        "tenant:ogs-students-7",
        "tenant:ogs-students-8",
        "tenant:ogs-students-auto",
      ]);
    });

    it("refreshes the Vertretungen page and the user-context group list", () => {
      renderHook(() => useGlobalSSE());

      fire({
        type: "group_access_changed",
        data: { source: "substitution_create" },
      });
      vi.advanceTimersByTime(500);

      expect(
        matchedKeys([
          "tenant:active-substitutions",
          "tenant:substitution-groups",
          "tenant:substitution-teachers",
          "tenant:user-context",
        ]),
      ).toEqual([
        "tenant:active-substitutions",
        "tenant:substitution-groups",
        "tenant:substitution-teachers",
        "tenant:user-context",
      ]);
    });

    it("does not drag unrelated cache families into a handover", () => {
      // A handover changes access, not attendance, staff hours or timetable
      // blocks. Refetching those would re-create the request herd #2083 removed.
      renderHook(() => useGlobalSSE());

      fire({
        type: "group_access_changed",
        data: { source: "group_transfer" },
      });
      vi.advanceTimersByTime(500);

      expect(
        matchedKeys([
          "tenant:staff-dashboard-summary-month",
          "tenant:timetable-roster-2026-01",
          "tenant:supervision-visits-3",
          "tenant:student-detail-42",
        ]),
      ).toEqual([]);
    });

    it("announces phoenix:supervision-stale so the sidebar group list refetches", () => {
      // SupervisionContext owns the sidebar's "Meine Gruppen" list in local
      // state behind its own fetch — no SWR key exists to invalidate, and its
      // does not use SWR. Without this announcement a handover stays invisible
      // until another explicit refresh.
      const listener = vi.fn();
      window.addEventListener("phoenix:supervision-stale", listener);
      renderHook(() => useGlobalSSE());

      fire({
        type: "group_access_changed",
        data: { source: "group_transfer" },
      });
      vi.advanceTimersByTime(500);

      expect(listener).toHaveBeenCalledTimes(1);
      window.removeEventListener("phoenix:supervision-stale", listener);
    });

    it("leaves every group-access cache and the sidebar alone on an unrelated event", () => {
      // A check-in must not drag the group list, the Vertretungen page or the
      // sidebar refetch along — that is the herd #2083 removed.
      const listener = vi.fn();
      window.addEventListener("phoenix:supervision-stale", listener);
      renderHook(() => useGlobalSSE());

      fire({ type: "dashboard_counts_changed", data: { group_ids: ["7"] } });
      vi.advanceTimersByTime(500);

      expect(
        matchedKeys([
          "tenant:active-substitutions",
          "tenant:substitution-groups",
          "tenant:substitution-teachers",
          "tenant:user-context",
        ]),
      ).toEqual([]);
      expect(listener).not.toHaveBeenCalled();
      window.removeEventListener("phoenix:supervision-stale", listener);
    });
  });
});
