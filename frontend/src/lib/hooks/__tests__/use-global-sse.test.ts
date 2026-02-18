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
    data: { user: { token: "test-token" } },
  })),
}));

vi.mock("swr", () => ({
  mutate: vi.fn(() => Promise.resolve()),
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
  });

  describe("event handling", () => {
    it("invalidates student caches on student_checkin event", () => {
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
    });

    it("invalidates student caches on student_checkout event", () => {
      renderHook(() => useGlobalSSE());

      const onMessage = vi.mocked(useSSE).mock.calls[0]?.[1]?.onMessage;

      onMessage?.({
        type: "student_checkout",
        active_group_id: "123",
        data: {},
        timestamp: new Date().toISOString(),
      });

      vi.advanceTimersByTime(500);

      expect(mutate).toHaveBeenCalled();
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
        return call[0] === "student-detail-42";
      });
      expect(studentDetailCall).toBeDefined();
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
  });
});
