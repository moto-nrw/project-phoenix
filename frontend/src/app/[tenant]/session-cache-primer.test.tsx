import { act, render, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { SessionProvider } from "next-auth/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { getCachedSession, sessionFetch } from "~/lib/session-cache";
import { mockSessionData } from "~/test/mocks/next-auth";
import { SessionCachePrimer } from "./providers";

/**
 * The real next-auth SessionProvider and the tenant primer (#4989). The
 * provider's session stays in the cache until its next poll; when the
 * provider learns the sign-in is gone, also from another tab, no request of
 * this tab may still pass as signed in.
 */

const fetchMock = vi.fn<typeof fetch>();
let sessionAnswer: Session | null = null;

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

function renderTab(session: Session) {
  return render(
    <SessionProvider
      session={session}
      refetchInterval={0}
      refetchOnWindowFocus={false}
    >
      <SessionCachePrimer />
    </SessionProvider>,
  );
}

beforeEach(() => {
  sessionAnswer = null;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async (url) => {
    if (String(url).endsWith("/session")) {
      // Auth.js answers `null` without a session.
      return Response.json(sessionAnswer);
    }
    return new Response(null, { status: 200 });
  });
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
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
});
