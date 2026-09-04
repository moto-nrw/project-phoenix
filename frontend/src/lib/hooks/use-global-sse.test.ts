import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

// ---------------------------------------------------------------------------
// Mocks (vi.hoisted so they are available inside vi.mock factories)
// ---------------------------------------------------------------------------

const { mockUseSession, mockUseSSE, mockMutate, capturedOptions } = vi.hoisted(
  () => {
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
      capturedOptions,
    };
  },
);

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
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
  // Pin the per-burst flush jitter (#2057) to 0 so the existing
  // advanceTimersByTime(500) timing assertions stay deterministic. Jitter
  // bounds get their own explicit tests.
  vi.spyOn(Math, "random").mockReturnValue(0);
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
  vi.restoreAllMocks();
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

  it.each(["admin:*", "*:*"])(
    "enables SSE for custom roles with %s permission",
    (permission) => {
      mockUseSession.mockReturnValue({
        status: "authenticated",
        data: {
          user: {
            token: "tok",
            roles: ["custom-admin"],
            permissions: [permission],
            tenantId: "t1",
            id: "u1",
          },
        },
      });

      renderHook(() => useGlobalSSE());

      expect(capturedOptions.enabled).toBe(true);
    },
  );

  it("does not enable SSE for custom roles without an admin wildcard", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: {
        user: {
          token: "tok",
          roles: ["custom-admin"],
          permissions: ["admin:read"],
          tenantId: "t1",
          id: "u1",
        },
      },
    });

    renderHook(() => useGlobalSSE());

    expect(capturedOptions.enabled).toBe(false);
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

describe("useGlobalSSE — change_requests_changed", () => {
  it("re-dispatches change-requests-refresh so an open review queue updates live", () => {
    renderHook(() => useGlobalSSE());

    const listener = vi.fn();
    window.addEventListener("change-requests-refresh", listener);

    fireSSE(
      makeEvent("change_requests_changed", { source: "excused_request" }),
    );

    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener("change-requests-refresh", listener);
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

  it("refreshes room-move source visits on attendance changes", () => {
    renderHook(() => useGlobalSSE());

    fireSSE(makeEvent("student_checkin", { student_id: "s1" }, "grp1"));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    const matchers = mockMutate.mock.calls
      .map(([matcher]) => matcher)
      .filter(
        (matcher): matcher is (key: unknown) => boolean =>
          typeof matcher === "function",
      );
    expect(
      matchers.some((matcher) => matcher("t1:room-bulk-source-visits-1-2")),
    ).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// Reminders revalidation (issue #1457)
// ---------------------------------------------------------------------------

// useGlobalSSE does not own the tenant-prefixed "{slug}:reminders" SWR key.
// Instead of mutating the wrong key it dispatches a window event; useReminders()
// owns the correctly scoped key and enabled gate, so it performs the actual
// revalidation.
describe("useGlobalSSE — reminders revalidation", () => {
  function listenForRemindersStale() {
    const listener = vi.fn();
    window.addEventListener("phoenix:reminders-stale", listener);
    return {
      listener,
      cleanup: () =>
        window.removeEventListener("phoenix:reminders-stale", listener),
    };
  }

  it("dispatches phoenix:reminders-stale on student_checkout after debounce", () => {
    renderHook(() => useGlobalSSE());
    const { listener, cleanup } = listenForRemindersStale();

    fireSSE(makeEvent("student_checkout", { student_id: "s1" }, "grp1"));
    expect(listener).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(listener).toHaveBeenCalledTimes(1);
    cleanup();
  });

  it("dispatches phoenix:reminders-stale on instance_overdue", () => {
    renderHook(() => useGlobalSSE());
    const { listener, cleanup } = listenForRemindersStale();

    fireSSE(makeEvent("instance_overdue"));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(listener).toHaveBeenCalledTimes(1);
    cleanup();
  });

  it("dispatches phoenix:reminders-stale on student_updated", () => {
    // Reminder rows carry student names/classes and are filtered by readable
    // scope, so a student edit must refresh them — not wait for the 60s poll.
    renderHook(() => useGlobalSSE());
    const { listener, cleanup } = listenForRemindersStale();

    fireSSE(makeEvent("student_updated", { student_id: "s1" }));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(listener).toHaveBeenCalledTimes(1);
    cleanup();
  });

  it("does not dispatch phoenix:reminders-stale for tenant_settings_changed", () => {
    renderHook(() => useGlobalSSE());
    const { listener, cleanup } = listenForRemindersStale();

    fireSSE(makeEvent("tenant_settings_changed", { source: "operator" }));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(listener).not.toHaveBeenCalled();
    cleanup();
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

// ---------------------------------------------------------------------------
// Laufgemeinschaft announcements
// ---------------------------------------------------------------------------

describe("useGlobalSSE — companion announcements", () => {
  it("announces companion changes only for student_companions_changed", async () => {
    const { subscribeStudentCompanionsChanged } =
      await import("~/lib/student-companion-api");
    renderHook(() => useGlobalSSE());
    const listener = vi.fn();
    const unsubscribe = subscribeStudentCompanionsChanged(listener);

    try {
      // A rename, a photo upload, a sick flag — all arrive as student_updated
      // and none of them touches the links. An open "läuft mit" form reacts to
      // the announcement by discarding or blocking its draft, so reacting here
      // would cost the user their work for an unrelated edit.
      fireSSE(makeEvent("student_updated", { student_id: "s1" }));
      act(() => {
        vi.advanceTimersByTime(500);
      });
      expect(listener).not.toHaveBeenCalled();

      // The dedicated event is the one the backend emits when a write could
      // actually have changed the links.
      fireSSE(makeEvent("student_companions_changed", { student_id: "s1" }));
      act(() => {
        vi.advanceTimersByTime(500);
      });
      expect(listener).toHaveBeenCalledTimes(1);
    } finally {
      unsubscribe();
    }
  });

  it("announces a display refresh for student_updated, so linked names stay honest", async () => {
    const { subscribeStudentCompanionDisplayChanged } =
      await import("~/lib/student-companion-api");
    renderHook(() => useGlobalSSE());
    const listener = vi.fn();
    const unsubscribe = subscribeStudentCompanionDisplayChanged(listener);

    try {
      // Renaming or reassigning a linked child leaves every edge untouched, so
      // no companion event follows — but the entry another child's card shows
      // for them is now wrong, and after a group change possibly no longer
      // visible to this viewer at all. The separate bus is what lets read-only
      // cards refetch for it while forms keep their drafts.
      fireSSE(makeEvent("student_updated", { student_id: "s1" }));
      act(() => {
        vi.advanceTimersByTime(500);
      });
      expect(listener).toHaveBeenCalledTimes(1);
    } finally {
      unsubscribe();
    }
  });

  it("revalidates the Kindersuche list, which carries its grouping in-band", () => {
    renderHook(() => useGlobalSSE());

    // Deleting a linked child announces ONLY this event — no student_updated,
    // no checkin — and the Kindersuche reads companion_student_ids out of its
    // own search-students-* response instead of fetching the links. Without the
    // invalidation an open "Nach Laufgemeinschaft" view keeps the deleted child.
    fireSSE(makeEvent("student_companions_changed", { student_id: "s1" }));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    const matchers = mockMutate.mock.calls
      .map(([matcher]) => matcher)
      .filter(
        (matcher): matcher is (key: unknown) => boolean =>
          typeof matcher === "function",
      );
    // Tenant-prefixed by useSWRAuth, hence the prefix in the probe key.
    expect(
      matchers.some((matcher) => matcher("t1:search-students-gall-all-")),
    ).toBe(true);
  });

  it("invalidates the per-child Betreuungsplan on attendance, arrival, timetable, and student-update events", () => {
    renderHook(() => useGlobalSSE());

    // The Betreuungsplan day/week view (care-plan-*) must stay live: a
    // check-in/out changes attendance, an arrival change moves the anchor, an
    // instance lifecycle event changes the blocks, and a student edit changes
    // header data. Without invalidation an open tab shows stale data.
    fireSSE(makeEvent("student_checkin", { student_id: "s1" }, "grp1"));
    fireSSE(makeEvent("student_updated", { student_id: "s1" }));
    fireSSE(makeEvent("arrival_schedule_changed", { student_id: "s1" }));
    fireSSE(makeEvent("instance_completed", { instance_id: "i1" }, "grp1"));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    const matchers = mockMutate.mock.calls
      .map(([matcher]) => matcher)
      .filter(
        (matcher): matcher is (key: unknown) => boolean =>
          typeof matcher === "function",
      );
    const dayKey = "t1:care-plan-day-s1-2026-07-22";
    const weekKey = "t1:care-plan-week-s1-2026-07-20-2026-07-24";
    // Probe every matcher against both keys so each modified block's clauses run.
    expect(
      matchers.filter((matcher) => matcher(dayKey)).length,
    ).toBeGreaterThan(0);
    expect(
      matchers.filter((matcher) => matcher(weekKey)).length,
    ).toBeGreaterThan(0);
  });

  it("refreshes pickup-derived caches broadly and id-lessly on pickup_schedule_changed (GDPR)", () => {
    renderHook(() => useGlobalSSE());

    // A staff Gehzeit write emits ONLY this event — no student_updated, no
    // checkin. It is TENANT-WIDE, so it deliberately carries NO student id: an
    // id in the raw SSE stream would leak pickup activity to staff outside
    // gdpr.student_data_scope=group_supervisors_only. Invalidation is therefore
    // broad across every open detail/care-plan/header cache, exactly like
    // arrival, and each refetch is server-access-filtered. The mock backend
    // sends no id; assert the client refreshes those caches for ANY open child
    // (s1 and s2 alike) plus the student lists.
    fireSSE(makeEvent("pickup_schedule_changed", {}));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    const matchers = mockMutate.mock.calls
      .map(([matcher]) => matcher)
      .filter(
        (matcher): matcher is (key: unknown) => boolean =>
          typeof matcher === "function",
      );
    // Tenant-prefixed by useSWRAuth, hence the prefix in every probe key.
    for (const key of [
      "t1:care-plan-day-s1-2026-07-22",
      "t1:care-plan-week-s1-2026-07-20-2026-07-24",
      "t1:pickup-data-s1",
      "t1:student-detail-s1",
      "t1:pickup-data-s2",
      "t1:student-detail-s2",
      "t1:search-students-gall-all-",
    ]) {
      expect(
        matchers.some((matcher) => matcher(key)),
        `pickup_schedule_changed must invalidate ${key}`,
      ).toBe(true);
    }
  });

  it("refreshes the detail header's pickup/arrival slots on student_updated", () => {
    renderHook(() => useGlobalSSE());

    // A parent care-exception submit or delete rewrites that day's pickup AND
    // arrival override but announces only student_updated; an approved care
    // request rewrites the weekly pickup plan under an arrival-named event.
    // Both header caches disable focus revalidation, so this is their only
    // live path for those writes.
    fireSSE(makeEvent("student_updated", { student_id: "s1" }));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    const matchers = mockMutate.mock.calls
      .map(([matcher]) => matcher)
      .filter(
        (matcher): matcher is (key: unknown) => boolean =>
          typeof matcher === "function",
      );
    for (const key of ["t1:pickup-data-s1", "t1:arrival-data-s1"]) {
      expect(
        matchers.some((matcher) => matcher(key)),
        `student_updated must invalidate ${key}`,
      ).toBe(true);
    }
  });

  // The two tests below pin the Aktuelle-Aufsicht live times. Since #2096
  // the aggregated active-supervision-dashboard response carries the pickup
  // and arrival times (like the OGS view's ogs-students-{gid}, #2056), and
  // the key disables focus revalidation, so an SSE invalidation is its ONLY
  // pickup-driven live update path.
  const AUFSICHT_DASHBOARD_KEY = "t1:active-supervision-dashboard-0";

  // Fires one event, flushes the burst debounce, and asserts that some matcher
  // handed to mutate() claims each key.
  function expectEventInvalidates(
    event: Parameters<typeof fireSSE>[0],
    keys: readonly string[],
  ) {
    renderHook(() => useGlobalSSE());
    fireSSE(event);
    act(() => {
      vi.advanceTimersByTime(500);
    });

    const matchers = mockMutate.mock.calls
      .map(([matcher]) => matcher)
      .filter(
        (matcher): matcher is (key: unknown) => boolean =>
          typeof matcher === "function",
      );
    for (const key of keys) {
      expect(
        matchers.some((matcher) => matcher(key)),
        `${event.type} must invalidate ${key}`,
      ).toBe(true);
    }
  }

  it("refreshes the Aufsicht aggregate on pickup_schedule_changed", () => {
    // A staff Gehzeit write (weekly plan or date exception) emits only this
    // event — see api/students/pickup_schedule_handlers.go. The supervising
    // staff member watching the room is exactly who must see the new time.
    expectEventInvalidates(makeEvent("pickup_schedule_changed", {}), [
      AUFSICHT_DASHBOARD_KEY,
    ]);
  });

  it("refreshes the Aufsicht aggregate on student_updated", () => {
    // A parent care exception (submit AND delete) rewrites that day's pickup
    // and arrival override under student_updated alone, and an approved care
    // request rewrites the weekly pickup plan while announcing only
    // student_updated + arrival_schedule_changed. Both writes can target a
    // child already standing in the room; the aggregate carries the times.
    expectEventInvalidates(makeEvent("student_updated", { student_id: "7" }), [
      AUFSICHT_DASHBOARD_KEY,
    ]);
  });

  it("refreshes the Aufsicht aggregate on arrival_schedule_changed", () => {
    expectEventInvalidates(makeEvent("arrival_schedule_changed", {}), [
      AUFSICHT_DASHBOARD_KEY,
    ]);
  });

  it("does not revalidate another child whose id merely starts with the event's", () => {
    renderHook(() => useGlobalSSE());

    // Ids are plain integers, so "1" is a prefix of "10", "11", "100". Matching
    // on includes() alone made one child's check-in refetch every such
    // sibling's detail and care-plan caches.
    fireSSE(makeEvent("student_checkin", { student_id: "1" }, "grp1"));
    act(() => {
      vi.advanceTimersByTime(500);
    });

    const matchers = mockMutate.mock.calls
      .map(([matcher]) => matcher)
      .filter(
        (matcher): matcher is (key: unknown) => boolean =>
          typeof matcher === "function",
      );
    const matchesAny = (key: string) =>
      matchers.some((matcher) => matcher(key));

    // The event's own child still revalidates, terminal id and segment id alike.
    expect(matchesAny("t1:student-detail-1")).toBe(true);
    expect(matchesAny("t1:care-plan-day-1-2026-07-22")).toBe(true);
    expect(matchesAny("t1:care-plan-week-1-2026-07-20-2026-07-24")).toBe(true);
    // Different children that share the id prefix must be left alone.
    expect(matchesAny("t1:student-detail-10")).toBe(false);
    expect(matchesAny("t1:care-plan-day-10-2026-07-22")).toBe(false);
    expect(matchesAny("t1:care-plan-week-100-2026-07-20-2026-07-24")).toBe(
      false,
    );
  });

  it("marks reminders stale on pickup_schedule_changed", () => {
    // "Abholung in 10 Min" / "überfällig" rows come from the effective pickup
    // time, so a Gehzeit edit adds, drops or re-times them. The bell and
    // /reminders own their tenant-prefixed SWR key under TenantProvider, so the
    // hook announces staleness via this window event instead of mutating.
    const listener = vi.fn();
    window.addEventListener("phoenix:reminders-stale", listener);

    try {
      renderHook(() => useGlobalSSE());
      fireSSE(makeEvent("pickup_schedule_changed", { student_id: "s1" }));
      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(listener).toHaveBeenCalledTimes(1);
    } finally {
      window.removeEventListener("phoenix:reminders-stale", listener);
    }
  });

  it("announces phoenix:care-schedule-stale on pickup, arrival and student_updated", () => {
    // The Betreuungszeiten editor keeps arrival/pickup in local state (not SWR)
    // and stays force-mounted, so SWR invalidation cannot reach it. Each of
    // these events changes the data it renders — a staff pickup/arrival write,
    // or a parent care-exception that surfaces only as student_updated — so all
    // three must announce staleness on the window event it listens for.
    for (const evt of [
      "pickup_schedule_changed",
      "arrival_schedule_changed",
      "student_updated",
    ] as const) {
      const listener = vi.fn();
      window.addEventListener("phoenix:care-schedule-stale", listener);
      try {
        const { unmount } = renderHook(() => useGlobalSSE());
        fireSSE(makeEvent(evt, { student_id: "s1" }));
        act(() => {
          vi.advanceTimersByTime(500);
        });
        expect(
          listener,
          `${evt} must announce care-schedule staleness`,
        ).toHaveBeenCalledTimes(1);
        unmount();
      } finally {
        window.removeEventListener("phoenix:care-schedule-stale", listener);
      }
    }
  });

  it("does not announce phoenix:care-schedule-stale for an unrelated event", () => {
    const listener = vi.fn();
    window.addEventListener("phoenix:care-schedule-stale", listener);
    try {
      renderHook(() => useGlobalSSE());
      fireSSE(makeEvent("tenant_settings_changed", {}));
      act(() => {
        vi.advanceTimersByTime(500);
      });
      expect(listener).not.toHaveBeenCalled();
    } finally {
      window.removeEventListener("phoenix:care-schedule-stale", listener);
    }
  });
});
