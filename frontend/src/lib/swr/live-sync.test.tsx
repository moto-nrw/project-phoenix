import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  LIVE_FALLBACK_INTERVAL_MS,
  LIVE_RECONCILE_INTERVAL_MS,
  useLiveEventScope,
  useLiveRefreshInterval,
  usePublishLiveChannel,
  useSWRAuth,
} from "./index";
import { resetSWRScope, resumeSWRSession, suspendSWRSession } from "./scope";
import { SWRConfig } from "./testing";

vi.unmock("swr");
const { owner } = vi.hoisted(() => ({
  owner: { current: { id: "account-7", scope: "tenant", tenantId: 42 } },
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: owner.current, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));

function wrapper({ children }: { children: ReactNode }) {
  return (
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      {children}
    </SWRConfig>
  );
}

// SWR reads the visibility on every poll tick; focus revalidation is off.
function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    get: () => state,
  });
}

/**
 * One tab: the global SSE consumer's channel (bound to the session scope)
 * and a reader that polls only for other people's changes.
 */
function renderTab(fetcher: () => Promise<number>, connected: boolean) {
  return renderHook(
    ({ live }: { live: boolean }) => {
      const { scope } = useLiveEventScope();
      usePublishLiveChannel(scope, live);
      const refreshInterval = useLiveRefreshInterval();
      return {
        refreshInterval,
        reader: useSWRAuth("live-sync-reader", fetcher, { refreshInterval }),
      };
    },
    { wrapper, initialProps: { live: connected } },
  );
}

async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

beforeEach(() => {
  vi.useFakeTimers();
  resetSWRScope({ revalidate: false });
  resumeSWRSession();
  owner.current = { id: "account-7", scope: "tenant", tenantId: 42 };
  setVisibility("visible");
});

describe("live reconcile interval (#5017)", () => {
  it("reconciles every five minutes, not every minute, while SSE is connected", async () => {
    // An event the stream lost (lossy buffer) changes the server state; no
    // invalidation arrives for it.
    let serverValue = 1;
    const fetcher = vi.fn(async () => serverValue);
    const tab = renderTab(fetcher, true);
    await advance(0);
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(tab.result.current.reader.data).toBe(1);
    expect(tab.result.current.refreshInterval).toBe(LIVE_RECONCILE_INTERVAL_MS);

    serverValue = 2;
    await advance(LIVE_FALLBACK_INTERVAL_MS);
    expect(fetcher).toHaveBeenCalledTimes(1);

    await advance(LIVE_RECONCILE_INTERVAL_MS - LIVE_FALLBACK_INTERVAL_MS);
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(tab.result.current.reader.data).toBe(2);
  });

  it("keeps the one-minute poll while SSE is down, so no change waits five minutes", async () => {
    let serverValue = 1;
    const fetcher = vi.fn(async () => serverValue);
    const tab = renderTab(fetcher, true);
    await advance(0);
    expect(fetcher).toHaveBeenCalledTimes(1);

    // The stream drops: the reader falls back to the minute poll at once.
    tab.rerender({ live: false });
    expect(tab.result.current.refreshInterval).toBe(LIVE_FALLBACK_INTERVAL_MS);
    serverValue = 2;
    await advance(LIVE_FALLBACK_INTERVAL_MS);
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(tab.result.current.reader.data).toBe(2);
    await advance(LIVE_FALLBACK_INTERVAL_MS);
    expect(fetcher).toHaveBeenCalledTimes(3);
  });

  it("polls every minute when the tab never had a stream", async () => {
    const fetcher = vi.fn(async () => 1);
    const tab = renderTab(fetcher, false);
    await advance(0);
    expect(tab.result.current.refreshInterval).toBe(LIVE_FALLBACK_INTERVAL_MS);
    await advance(LIVE_FALLBACK_INTERVAL_MS);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("does not reconcile in a hidden tab, and does on the first tick once visible", async () => {
    const fetcher = vi.fn(async () => 1);
    renderTab(fetcher, true);
    await advance(0);
    expect(fetcher).toHaveBeenCalledTimes(1);

    setVisibility("hidden");
    await advance(3 * LIVE_RECONCILE_INTERVAL_MS);
    expect(fetcher).toHaveBeenCalledTimes(1);

    setVisibility("visible");
    await advance(LIVE_RECONCILE_INTERVAL_MS);
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("counts only a stream of the reader's own school and account", async () => {
    // The channel stays bound to account 7 while the reader already reads
    // for account 8: account 7's stream says nothing about account 8.
    const channel = renderHook(
      () => {
        const { scope } = useLiveEventScope();
        usePublishLiveChannel(scope, true);
      },
      { wrapper },
    );
    const reader = renderHook(() => useLiveRefreshInterval(), { wrapper });
    expect(reader.result.current).toBe(LIVE_RECONCILE_INTERVAL_MS);

    owner.current = { ...owner.current, id: "account-8" };
    reader.rerender();
    expect(reader.result.current).toBe(LIVE_FALLBACK_INTERVAL_MS);
    channel.unmount();
  });
});

describe("live event scope (#5017)", () => {
  it("accepts an event only for the binding's school and account", () => {
    const tab = renderHook(() => useLiveEventScope(), { wrapper });
    const bound = tab.result.current.scope;
    expect(bound).toBe("school-a:account-7:");
    expect(tab.result.current.accepts(bound)).toBe(true);

    // Another tab binds the browser to another account: until this tab's
    // provider confirms, no event may act.
    act(() => suspendSWRSession());
    tab.rerender();
    expect(tab.result.current.scope).toBe(bound);
    expect(tab.result.current.accepts(bound)).toBe(false);
    act(() => resumeSWRSession());

    // The confirmed session names account 8: events of the stream bound to
    // account 7 no longer count, the new binding does.
    owner.current = { ...owner.current, id: "account-8" };
    tab.rerender();
    expect(tab.result.current.scope).toBe("school-a:account-8:");
    expect(tab.result.current.accepts(bound)).toBe(false);
    expect(tab.result.current.accepts(tab.result.current.scope)).toBe(true);
    expect(tab.result.current.accepts(null)).toBe(false);
  });
});
