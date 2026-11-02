import { act, cleanup, render, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { SessionProvider, useSession } from "next-auth/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { apiErrorFromResponse } from "~/lib/api-error";
import { getCachedSession, sessionFetch } from "~/lib/session-cache";
import { setTestClock, TEST_CLOCK_INSTANT } from "~/test/clock";
import { mockSessionData } from "~/test/mocks/next-auth";
import { SessionCachePrimer } from "./providers";

/**
 * The real next-auth SessionProvider and the tenant primer (#4989). The
 * provider's session stays in the cache until its next poll (#5130); when the
 * provider learns the sign-in is gone, also from another tab, no request of
 * this tab may still pass as signed in.
 */

const fetchMock = vi.fn<typeof fetch>();
let sessionAnswer: Session | null = null;
let apiAnswer: () => Response = () => new Response(null, { status: 200 });

function sessionRequests(): number {
  return fetchMock.mock.calls.filter(([url]) =>
    String(url).endsWith("/session"),
  ).length;
}

function apiRequests(): number {
  return fetchMock.mock.calls.filter(([url]) => String(url) === "/api/test")
    .length;
}

/** What another tab of the same browser sends after a sign-in or logout. */
function otherTabChangesSession() {
  const channel = new BroadcastChannel("next-auth");
  channel.postMessage({ event: "session", data: { trigger: "signout" } });
  channel.close();
}

/** The school this tab's provider reports, as TenantGuard reads it. */
function ProviderTenant() {
  const { data } = useSession();
  return <output>{String(data?.user?.tenantId ?? "")}</output>;
}

function renderTab(session: Session) {
  return render(
    <SessionProvider
      session={session}
      refetchInterval={0}
      refetchOnWindowFocus={false}
    >
      <SessionCachePrimer />
      <ProviderTenant />
    </SessionProvider>,
  );
}

beforeEach(() => {
  sessionAnswer = null;
  apiAnswer = () => new Response(null, { status: 200 });
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url) => {
    if (String(url).endsWith("/session")) {
      // Auth.js answers `null` without a session.
      return Response.json(sessionAnswer);
    }
    return apiAnswer();
  });
  vi.stubGlobal("fetch", fetchMock);
});

/**
 * Unmounts the tab and lets broadcasts still on their way arrive while no
 * provider listens; otherwise a test's announcement lands in the next one.
 */
async function drainBroadcasts() {
  cleanup();
  const listener = new BroadcastChannel("next-auth");
  const sender = new BroadcastChannel("next-auth");
  await new Promise<void>((resolve) => {
    listener.onmessage = (event: MessageEvent<{ event?: string }>) => {
      if (event.data.event === "drained") resolve();
    };
    sender.postMessage({ event: "drained" });
  });
  listener.close();
  sender.close();
}

afterEach(async () => {
  await drainBroadcasts();
  vi.unstubAllGlobals();
});

describe("SessionCachePrimer with the real SessionProvider", () => {
  it("serves the hydrated session without asking the session route", async () => {
    renderTab(mockSessionData({ user: { id: "a" } }));

    expect((await getCachedSession())?.user?.id).toBe("a");
    expect(sessionRequests()).toBe(0);
  });

  it("stops passing as signed in once another tab logged out", async () => {
    renderTab(mockSessionData({ user: { id: "a" } }));
    expect((await getCachedSession())?.user?.id).toBe("a");

    sessionAnswer = null;
    act(() => otherTabChangesSession());
    await waitFor(() => expect(sessionRequests()).toBe(1));

    await waitFor(async () => expect(await getCachedSession()).toBeNull());
    await expect(sessionFetch("/api/test")).rejects.toMatchObject({
      status: 401,
      code: "general.permission",
    });
    expect(apiRequests()).toBe(0);
  });

  it("never serves the previous account after another tab signed in", async () => {
    renderTab(mockSessionData({ user: { id: "a", tenantId: 1 } }));
    expect((await getCachedSession())?.user?.id).toBe("a");

    sessionAnswer = mockSessionData({ user: { id: "b", tenantId: 2 } });
    act(() => otherTabChangesSession());

    await waitFor(async () =>
      expect((await getCachedSession())?.user?.id).toBe("b"),
    );
    // Only the provider's own request; the cache took its answer.
    expect(sessionRequests()).toBe(1);
  });

  // Review of #5116, mandatory test of #5130: a school switch on another
  // school's subdomain replaces the domain-wide cookie. BroadcastChannel and
  // storage events stay within one origin, so this tab's provider keeps
  // reporting the old school. The primed session now holds until the
  // provider's next poll; the BFF refuses the first request with the other
  // school's cookie, and that refusal must drop the old school from the
  // cache and the provider at once.
  it("drops the old school with the first request refused after a switch on another subdomain", async () => {
    const view = renderTab(mockSessionData({ user: { id: "a", tenantId: 1 } }));
    expect((await getCachedSession())?.user?.tenantId).toBe(1);

    sessionAnswer = mockSessionData({ user: { id: "a", tenantId: 2 } });
    apiAnswer = () =>
      Response.json(
        { status: 409, code: "identity.session_tenant_mismatch" },
        { status: 409 },
      );
    const response = await sessionFetch("/api/test");
    const error = await apiErrorFromResponse(response, "Request refused");
    expect(error.code).toBe("identity.session_tenant_mismatch");

    expect((await getCachedSession())?.user?.tenantId).toBe(2);
    await waitFor(() =>
      expect(view.getByRole("status")).toHaveTextContent("2"),
    );
  });

  // #5130: the lookup announces a changed session, so the provider and with
  // it TenantGuard follow the school switch on the next page change, not
  // only at the provider's next poll.
  it("tells the provider about a school switch on another subdomain", async () => {
    const view = renderTab(mockSessionData({ user: { id: "a", tenantId: 1 } }));
    expect(view.getByRole("status")).toHaveTextContent("1");

    sessionAnswer = mockSessionData({ user: { id: "a", tenantId: 2 } });
    setTestClock(new Date(TEST_CLOCK_INSTANT.getTime() + 270_001));
    expect((await getCachedSession())?.user?.tenantId).toBe(2);

    await waitFor(() =>
      expect(view.getByRole("status")).toHaveTextContent("2"),
    );
    // The cache's lookup and the provider's own.
    expect(sessionRequests()).toBe(2);
  });

  // #5130: getSession() broadcasts by default, and this tab's provider
  // answered every lookup after the window with a second request.
  it("asks the session route once when the session did not change", async () => {
    renderTab(mockSessionData({ user: { id: "a", tenantId: 1 } }));
    expect((await getCachedSession())?.user?.tenantId).toBe(1);

    sessionAnswer = mockSessionData({ user: { id: "a", tenantId: 1 } });
    setTestClock(new Date(TEST_CLOCK_INSTANT.getTime() + 270_001));
    expect((await getCachedSession())?.user?.tenantId).toBe(1);
    expect(sessionRequests()).toBe(1);

    // Anker: a later broadcast reaches the provider, which asks once. An echo
    // of the lookup, delivered before it, would have made it three.
    act(() => otherTabChangesSession());
    await waitFor(() => expect(sessionRequests()).toBeGreaterThanOrEqual(2));
    expect(sessionRequests()).toBe(2);
  });
});
