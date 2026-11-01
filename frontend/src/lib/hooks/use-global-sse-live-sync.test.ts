import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SSEEvent, SSEHookOptions, SSEHookState } from "~/lib/sse-types";

const { session, sse, invalidatedFamilies } = vi.hoisted(() => ({
  session: {
    current: { id: "account-7", tenantId: 1, roles: ["user"] } as {
      id: string;
      tenantId: number;
      roles: string[];
    },
  },
  sse: {
    state: {
      status: "connected",
      isConnected: true,
      error: null,
      reconnectAttempts: 0,
    } as SSEHookState,
    handlers: [] as ((event: SSEEvent) => void)[],
  },
  invalidatedFamilies: [] as string[],
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: session.current, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => ({ tenant: { tenantId: 1 } }),
  useTenantSlugSafe: () => "school-a",
}));
vi.mock("~/lib/hooks/use-sse", () => ({
  useSSE: (_endpoint: string, options: SSEHookOptions) => {
    if (options.onMessage && sse.handlers.at(-1) !== options.onMessage) {
      sse.handlers.push(options.onMessage);
    }
    return sse.state;
  },
}));
vi.mock("~/lib/swr", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/swr")>()),
  useInvalidateFamily: () => (family: string) => {
    invalidatedFamilies.push(family);
    return Promise.resolve([]);
  },
}));
vi.mock("~/lib/swr/mutate", () => ({
  mutate: vi.fn(() => Promise.resolve()),
}));

import { mutate } from "~/lib/swr/mutate";
import {
  LIVE_FALLBACK_INTERVAL_MS,
  LIVE_RECONCILE_INTERVAL_MS,
  useLiveRefreshInterval,
} from "~/lib/swr";
import { resumeSWRSession, suspendSWRSession } from "~/lib/swr/scope";
import { ownSupervisionKeys } from "~/lib/swr/shell-keys";
import {
  MAX_FLUSH_WAIT_MS,
  RECONCILE_MIN_GAP_MS,
  useGlobalSSE,
} from "./use-global-sse";

const OWN = ownSupervisionKeys.family;
// Keys of the scoped cache as SWR's global mutate sees them.
const INVENTORY = [
  "school-a:account-7:active-supervision-dashboard-0-224",
  "school-a:account-7:time-tracking-month-summary-2026-9",
  "school-a:account-7:staff-dashboard-summary-month",
  "school-a:account-7:ogs-students-3",
  "school-a:account-7:timetable-week-2026-09-07",
  "school-a:account-7:time-tracking-config",
];

function matchedKeys(): string[] {
  const matchers = vi
    .mocked(mutate)
    .mock.calls.map((call) => call[0])
    .filter((m): m is (key: unknown) => boolean => typeof m === "function");
  return INVENTORY.filter((key) => matchers.some((m) => m(key)));
}

function event(type: SSEEvent["type"], data: SSEEvent["data"] = {}): SSEEvent {
  return {
    type,
    active_group_id: "",
    data,
    timestamp: new Date().toISOString(),
  };
}

function fire(e: SSEEvent, handler = sse.handlers.at(-1)) {
  act(() => handler?.(e));
}

function setStatus(status: SSEHookState["status"]) {
  sse.state = {
    status,
    isConnected: status === "connected",
    error: null,
    reconnectAttempts: status === "reconnecting" ? 1 : 0,
  };
}

function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    get: () => state,
  });
}

function renderConnected() {
  setStatus("connected");
  const view = renderHook(() => useGlobalSSE());
  vi.mocked(mutate).mockClear();
  invalidatedFamilies.length = 0;
  return view;
}

/** The stream drops and reopens; the hook sees both states. */
function reconnect(view: { rerender: () => void }) {
  setStatus("reconnecting");
  view.rerender();
  setStatus("connected");
  view.rerender();
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.spyOn(Math, "random").mockReturnValue(0);
  vi.mocked(mutate).mockClear();
  invalidatedFamilies.length = 0;
  sse.handlers = [];
  session.current = { id: "account-7", tenantId: 1, roles: ["user"] };
  setVisibility("visible");
  resumeSWRSession();
});

describe("useGlobalSSE: own supervision family (#5017)", () => {
  it.each([
    ["instance_started", {}],
    ["staffing_deviation_changed", {}],
    ["activity_start", {}],
    ["active_supervision_changed", { reason: "supervision_handed_over" }],
    ["group_access_changed", {}],
    ["pickup_schedule_changed", {}],
  ] satisfies [SSEEvent["type"], SSEEvent["data"]][])(
    "%s invalidates the own supervision family",
    (type, data) => {
      renderConnected();
      fire(event(type, data));
      act(() => vi.advanceTimersByTime(MAX_FLUSH_WAIT_MS));
      expect(invalidatedFamilies).toContain(OWN);
    },
  );

  it.each([
    ["dashboard_counts_changed", { group_ids: ["3"] }],
    ["staff_time_tracking_changed", {}],
  ] satisfies [SSEEvent["type"], SSEEvent["data"]][])(
    "%s leaves the own supervision family alone",
    (type, data) => {
      renderConnected();
      fire(event(type, data));
      act(() => vi.advanceTimersByTime(MAX_FLUSH_WAIT_MS));
      // Anker: the event did flush its own caches.
      expect(matchedKeys().length).toBeGreaterThan(0);
      expect(invalidatedFamilies).not.toContain(OWN);
    },
  );

  it("staff_time_tracking_changed refreshes exactly the time-tracking caches", () => {
    renderConnected();
    fire(event("staff_time_tracking_changed"));
    act(() => vi.advanceTimersByTime(MAX_FLUSH_WAIT_MS));
    expect(matchedKeys()).toEqual([
      "school-a:account-7:time-tracking-month-summary-2026-9",
      "school-a:account-7:staff-dashboard-summary-month",
    ]);
  });
});

describe("useGlobalSSE: reconcile after a reconnect (#5017)", () => {
  it("reconciles every SSE-driven cache at once when the stream is back", () => {
    const view = renderConnected();
    reconnect(view);
    // No timer: the reconcile does not wait for the event debounce.
    expect(matchedKeys()).toEqual(INVENTORY.slice(0, 5));
    expect(invalidatedFamilies).toContain(OWN);
  });

  it("does not reconcile on the first connection", () => {
    setStatus("idle");
    const view = renderHook(() => useGlobalSSE());
    setStatus("connected");
    view.rerender();
    act(() => vi.advanceTimersByTime(RECONCILE_MIN_GAP_MS));
    expect(mutate).not.toHaveBeenCalled();
    expect(invalidatedFamilies).toEqual([]);
  });

  it("waits until a hidden tab is visible again", () => {
    const view = renderConnected();
    setVisibility("hidden");
    reconnect(view);
    act(() => vi.advanceTimersByTime(RECONCILE_MIN_GAP_MS));
    expect(mutate).not.toHaveBeenCalled();
    expect(invalidatedFamilies).toEqual([]);

    setVisibility("visible");
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(matchedKeys()).toEqual(INVENTORY.slice(0, 5));
    expect(invalidatedFamilies).toContain(OWN);
  });

  it("reconciles a flapping stream at most once per window", () => {
    const view = renderConnected();
    reconnect(view);
    expect(invalidatedFamilies.filter((f) => f === OWN)).toHaveLength(1);

    reconnect(view);
    reconnect(view);
    act(() => vi.advanceTimersByTime(RECONCILE_MIN_GAP_MS - 1));
    expect(invalidatedFamilies.filter((f) => f === OWN)).toHaveLength(1);

    act(() => vi.advanceTimersByTime(1));
    expect(invalidatedFamilies.filter((f) => f === OWN)).toHaveLength(2);
    act(() => vi.advanceTimersByTime(RECONCILE_MIN_GAP_MS));
    expect(invalidatedFamilies.filter((f) => f === OWN)).toHaveLength(2);
  });

  it("publishes the connected stream, so readers reconcile every five minutes", () => {
    setStatus("connected");
    const view = renderHook(() => {
      useGlobalSSE();
      return useLiveRefreshInterval();
    });
    expect(view.result.current).toBe(LIVE_RECONCILE_INTERVAL_MS);
    setStatus("reconnecting");
    view.rerender();
    expect(view.result.current).toBe(LIVE_FALLBACK_INTERVAL_MS);
  });
});

describe("useGlobalSSE: events of another school or account (#5017)", () => {
  it("ignores an event the stream of the previous account still delivers", () => {
    const view = renderConnected();
    const accountSeven = sse.handlers.at(-1);
    session.current = { ...session.current, id: "account-8" };
    view.rerender();
    // Anker: the new binding has its own handler.
    expect(sse.handlers.at(-1)).not.toBe(accountSeven);

    fire(event("staff_time_tracking_changed"), accountSeven);
    fire(event("instance_started"), accountSeven);
    act(() => vi.advanceTimersByTime(MAX_FLUSH_WAIT_MS));
    expect(mutate).not.toHaveBeenCalled();
    expect(invalidatedFamilies).toEqual([]);

    // The stream of account 8 acts as usual.
    fire(event("instance_started"));
    act(() => vi.advanceTimersByTime(MAX_FLUSH_WAIT_MS));
    expect(invalidatedFamilies).toContain(OWN);
  });

  it("drops a burst collected before the account changed", () => {
    const view = renderConnected();
    fire(event("staff_time_tracking_changed"));
    session.current = { ...session.current, id: "account-8" };
    view.rerender();
    act(() => vi.advanceTimersByTime(MAX_FLUSH_WAIT_MS));
    expect(mutate).not.toHaveBeenCalled();
    expect(invalidatedFamilies).toEqual([]);
  });

  it("ignores events while another tab's session change is being confirmed", () => {
    const view = renderConnected();
    act(() => suspendSWRSession());
    view.rerender();
    fire(event("staff_time_tracking_changed"));
    fire(event("instance_started"));
    act(() => vi.advanceTimersByTime(MAX_FLUSH_WAIT_MS));
    expect(mutate).not.toHaveBeenCalled();
    expect(invalidatedFamilies).toEqual([]);
    act(() => resumeSWRSession());
  });
});
