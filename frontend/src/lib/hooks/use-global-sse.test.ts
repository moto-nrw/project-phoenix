import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

// ---------------------------------------------------------------------------
// Mocks (vi.hoisted so they are available inside vi.mock factories)
// ---------------------------------------------------------------------------

const { mockUseSession, mockUseSSE, mockMutate, mockCache, capturedOptions } =
  vi.hoisted(() => {
    const capturedOptions: {
      onMessage?: (event: unknown) => void;
      enabled?: boolean;
      reconnectKey?: string | number;
    } = {};

    type SSEState = {
      isConnected: boolean;
      error: null;
      reconnectAttempts: number;
      status: "idle" | "connected" | "reconnecting" | "failed";
    };
    return {
      mockUseSession: vi.fn(),
      mockUseSSE: vi.fn((_: string, opts: typeof capturedOptions): SSEState => {
        Object.assign(capturedOptions, opts);
        return {
          isConnected: false,
          error: null,
          reconnectAttempts: 0,
          status: "idle",
        };
      }),
      mockMutate: vi.fn().mockResolvedValue(undefined),
      // Shared SWR cache the reminders-gating logic reads. Tests seed a
      // "tenant-slug:reminders" entry to model the feature being on/off.
      mockCache: new Map<string, { data?: { enabled?: boolean } }>(),
      capturedOptions,
    };
  });

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

// The reminders revalidation path scopes to the active tenant's SWR key, so the
// hook needs a slug. Provide a stable one for the "tenant-slug:reminders" key.
vi.mock("~/components/tenant/tenant-provider", () => ({
  useTenantSlugSafe: () => "tenant-slug",
}));

vi.mock("~/lib/hooks/use-sse", () => ({
  useSSE: mockUseSSE,
}));

// Override the global SWR mock from setup.ts with a version that exposes our spy
vi.mock("swr", () => ({
  default: vi.fn(() => ({
    data: undefined,
    error: undefined,
    isLoading: true,
    isValidating: false,
    mutate: vi.fn(),
  })),
  mutate: mockMutate,
  useSWRConfig: vi.fn(() => ({ mutate: mockMutate, cache: mockCache })),
}));

vi.mock("~/lib/swr/room-derived-caches", () => ({
  ROOM_LIST_CACHE_KEYS: ["rooms-list"],
}));

import { useGlobalSSE } from "./use-global-sse";
import type { SSEEvent } from "~/lib/sse-types";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeEvent(
  type: SSEEvent["type"],
  data: SSEEvent["data"] = {},
  active_group_id = "",
): SSEEvent {
  return {
    type,
    active_group_id,
    data,
    timestamp: new Date().toISOString(),
  };
}

function authenticatedSession(tenantId = "t1") {
  return {
    status: "authenticated" as const,
    data: {
      user: {
        token: "tok",
        roles: ["user"],
        tenantId,
        id: "u1",
      },
    },
  };
}

function fireSSE(event: SSEEvent) {
  act(() => {
    capturedOptions.onMessage?.(event);
  });
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.clearAllMocks();
  mockCache.clear();
  mockUseSession.mockReturnValue(authenticatedSession());
  // Re-wire the useSSE mock after clearAllMocks
  mockUseSSE.mockImplementation(
    (_url: string, opts: typeof capturedOptions) => {
      Object.assign(capturedOptions, opts);
      return {
        isConnected: false,
        error: null,
        reconnectAttempts: 0,
        status: "idle" as const,
      };
    },
  );
});

afterEach(() => {
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// Authentication gate
// ---------------------------------------------------------------------------

describe("useGlobalSSE — authentication gate", () => {
  it("does not enable SSE when session is unauthenticated", () => {
    mockUseSession.mockReturnValue({
      status: "unauthenticated",
      data: null,
    });

    renderHook(() => useGlobalSSE());

    expect(capturedOptions.enabled).toBe(false);
  });

  it("does not enable SSE when the user has no staff or admin role", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { token: "tok", roles: [], tenantId: "t1", id: "u1" } },
    });

    renderHook(() => useGlobalSSE());

    expect(capturedOptions.enabled).toBe(false);
  });

  it("enables SSE for staff users", () => {
    mockUseSession.mockReturnValue(authenticatedSession());

    renderHook(() => useGlobalSSE());

    expect(capturedOptions.enabled).toBe(true);
  });

  it("enables SSE for admin-only users", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: {
        user: { token: "tok", roles: ["admin"], tenantId: "t1", id: "u1" },
      },
    });

    renderHook(() => useGlobalSSE());

    expect(capturedOptions.enabled).toBe(true);
  });

  it("passes the tenantId as reconnectKey", () => {
    renderHook(() => useGlobalSSE());
    expect(capturedOptions.reconnectKey).toBe("t1");
  });
});

// ---------------------------------------------------------------------------
// parent_message / parent_message_read events → window event fan-out
// ---------------------------------------------------------------------------

describe("useGlobalSSE — parent_message dispatch", () => {
  it("dispatches messages-unread-refresh on parent_message", () => {
    renderHook(() => useGlobalSSE());

    const listener = vi.fn();
    window.addEventListener("messages-unread-refresh", listener);

    fireSSE(makeEvent("parent_message", { thread_id: "t1", student_id: "s1" }));

    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener("messages-unread-refresh", listener);
  });

  it("dispatches messages-activity with thread_id and student_id on parent_message", () => {
    renderHook(() => useGlobalSSE());

    let detail: { threadId: unknown; studentId: unknown } | null = null;
    const listener = (e: Event) => {
      detail = (e as CustomEvent).detail as typeof detail;
    };
    window.addEventListener("messages-activity", listener);

    fireSSE(
      makeEvent("parent_message", { thread_id: "t42", student_id: "s99" }),
    );

    expect(detail).toMatchObject({ threadId: "t42", studentId: "s99" });
    window.removeEventListener("messages-activity", listener);
  });

  it("dispatches null ids in messages-activity when event data has no thread/student", () => {
    renderHook(() => useGlobalSSE());

    let detail: { threadId: unknown; studentId: unknown } | null = null;
    const listener = (e: Event) => {
      detail = (e as CustomEvent).detail as typeof detail;
    };
    window.addEventListener("messages-activity", listener);

    fireSSE(makeEvent("parent_message", {}));

    expect(detail).toMatchObject({ threadId: null, studentId: null });
    window.removeEventListener("messages-activity", listener);
  });

  it("dispatches messages-unread-refresh on parent_message_read", () => {
    renderHook(() => useGlobalSSE());

    const listener = vi.fn();
    window.addEventListener("messages-unread-refresh", listener);

    fireSSE(makeEvent("parent_message_read", { thread_id: "t1" }));

    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener("messages-unread-refresh", listener);
  });

  it("dispatches messages-activity on parent_message_read", () => {
    renderHook(() => useGlobalSSE());

    const listener = vi.fn();
    window.addEventListener("messages-activity", listener);

    fireSSE(makeEvent("parent_message_read", { thread_id: "t1" }));

    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener("messages-activity", listener);
  });
});

// ---------------------------------------------------------------------------
// tenant_settings_changed → phoenix:tenant-settings-stale
// ---------------------------------------------------------------------------

describe("useGlobalSSE — tenant_settings_changed", () => {
  it("dispatches phoenix:tenant-settings-stale on tenant_settings_changed", () => {
    renderHook(() => useGlobalSSE());

    const listener = vi.fn();
    window.addEventListener("phoenix:tenant-settings-stale", listener);

    fireSSE(makeEvent("tenant_settings_changed", { source: "operator" }));

    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener("phoenix:tenant-settings-stale", listener);
  });

  it("includes source in the event detail", () => {
    renderHook(() => useGlobalSSE());

    let captured: Record<string, unknown> | null = null;
    const listener = (e: Event) => {
      captured = (e as CustomEvent<Record<string, unknown>>).detail;
    };
    window.addEventListener("phoenix:tenant-settings-stale", listener);

    fireSSE(makeEvent("tenant_settings_changed", { source: "operator" }));

    expect(captured?.["source"]).toBe("operator");
    window.removeEventListener("phoenix:tenant-settings-stale", listener);
  });
});

// ---------------------------------------------------------------------------
// SWR invalidation via debounced flush
// ---------------------------------------------------------------------------

describe("useGlobalSSE — SWR invalidation debounce", () => {
  it("calls mutate after 500 ms debounce on student_checkin", async () => {
    renderHook(() => useGlobalSSE());

    fireSSE(makeEvent("student_checkin", { student_id: "s1" }, "grp1"));

    expect(mockMutate).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(mockMutate).toHaveBeenCalled();
  });

  it("collapses a burst of events into one flush", async () => {
    renderHook(() => useGlobalSSE());

    fireSSE(makeEvent("student_checkin", { student_id: "s1" }, "grp1"));
    fireSSE(makeEvent("student_checkin", { student_id: "s2" }, "grp1"));
    fireSSE(makeEvent("student_checkin", { student_id: "s3" }, "grp1"));

    act(() => {
      vi.advanceTimersByTime(500);
    });

    // Should have called mutate, but not once per event
    const beforeNextFlush = mockMutate.mock.calls.length;
    expect(beforeNextFlush).toBeGreaterThan(0);
    expect(beforeNextFlush).toBeLessThan(9); // definitely not 3 per event × 3 events
  });

  it("calls mutate on dashboard_counts_changed after debounce", () => {
    renderHook(() => useGlobalSSE());

    fireSSE(makeEvent("dashboard_counts_changed"));

    expect(mockMutate).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(mockMutate).toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Reminders revalidation (issue #1457)
// ---------------------------------------------------------------------------

const REMINDERS_KEY = "tenant-slug:reminders";

// True when mutate() was called with the exact reminders key. The reminders
// path now targets the active tenant's key directly (not a substring matcher),
// so a cross-tenant cache in another tab is never revalidated.
function mutateCalledWithRemindersKey(): boolean {
  return mockMutate.mock.calls.some(([arg]) => arg === REMINDERS_KEY);
}

// Model the reminders feature being switched on for this tenant: the bell's
// SWR entry exists with enabled=true.
function seedRemindersEnabled(enabled: boolean) {
  mockCache.set(REMINDERS_KEY, { data: { enabled } });
}

describe("useGlobalSSE — reminders revalidation", () => {
  it("revalidates the reminders cache on student_checkout when the feature is enabled", () => {
    seedRemindersEnabled(true);
    renderHook(() => useGlobalSSE());

    fireSSE(makeEvent("student_checkout", { student_id: "s1" }, "grp1"));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(mutateCalledWithRemindersKey()).toBe(true);
  });

  it("revalidates the reminders cache on instance_overdue when the feature is enabled", () => {
    seedRemindersEnabled(true);
    renderHook(() => useGlobalSSE());

    fireSSE(makeEvent("instance_overdue"));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(mutateCalledWithRemindersKey()).toBe(true);
  });

  it("does NOT revalidate reminders when the feature is disabled", () => {
    // The default: all reminder types off. The bell still creates a cache entry
    // with enabled=false; a check-in burst must not revalidate it.
    seedRemindersEnabled(false);
    renderHook(() => useGlobalSSE());

    fireSSE(makeEvent("student_checkout", { student_id: "s1" }, "grp1"));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(mutateCalledWithRemindersKey()).toBe(false);
  });

  it("does not revalidate reminders for tenant_settings_changed", () => {
    seedRemindersEnabled(true);
    renderHook(() => useGlobalSSE());

    fireSSE(makeEvent("tenant_settings_changed", { source: "operator" }));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(mutateCalledWithRemindersKey()).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Return value
// ---------------------------------------------------------------------------

describe("useGlobalSSE — return value", () => {
  it("returns the SSEHookState from useSSE", () => {
    mockUseSSE.mockImplementation(
      (_url: string, opts: typeof capturedOptions) => {
        Object.assign(capturedOptions, opts);
        return {
          isConnected: true,
          error: null,
          reconnectAttempts: 2,
          status: "connected" as const,
        };
      },
    );

    const { result } = renderHook(() => useGlobalSSE());

    expect(result.current.isConnected).toBe(true);
    expect(result.current.status).toBe("connected");
    expect(result.current.reconnectAttempts).toBe(2);
  });

  it("connects to /api/sse/events", () => {
    renderHook(() => useGlobalSSE());

    expect(mockUseSSE).toHaveBeenCalledWith(
      "/api/sse/events",
      expect.objectContaining({ onMessage: expect.any(Function) }),
    );
  });
});
