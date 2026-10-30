import { act, render } from "@testing-library/react";
import { SessionProvider } from "next-auth/react";
import { afterEach, expect, it, vi } from "vitest";
import { signOutTenant } from "./tenant-session-cookie";

it("does not refresh the real current provider while destination navigation is stalled", async () => {
  const channels = new Set<TestChannel>();
  class TestChannel extends EventTarget {
    constructor(readonly name: string) {
      super();
      channels.add(this);
    }
    postMessage(data: unknown) {
      for (const channel of channels) {
        if (channel !== this && channel.name === this.name) {
          queueMicrotask(() =>
            channel.dispatchEvent(new MessageEvent("message", { data })),
          );
        }
      }
    }
    close() {
      channels.delete(this);
    }
  }
  vi.stubGlobal("BroadcastChannel", TestChannel);
  const originalLocation = window.location;
  Object.defineProperty(window, "location", {
    configurable: true,
    value: { href: "http://school-a.localhost/home" },
  });
  const session = {
    user: { id: "7" },
    expires: new Date(Date.now() + 3_600_000).toISOString(),
  };
  const fetch = vi.fn(async (url: string) =>
    Response.json(
      url.endsWith("/csrf")
        ? { csrfToken: crypto.randomUUID() }
        : url.endsWith("/session")
          ? session
          : { url: "http://school-b.localhost/" },
    ),
  );
  vi.stubGlobal("fetch", fetch);
  try {
    render(
      <SessionProvider session={session} refetchOnWindowFocus={false}>
        <div>Session active</div>
      </SessionProvider>,
    );
    // Prove that another channel really reaches the installed provider.
    const otherTab = new BroadcastChannel("next-auth");
    await act(async () => {
      otherTab.postMessage({ event: "session", data: { trigger: "signout" } });
    });
    expect(fetch.mock.calls.some(([url]) => url.endsWith("/session"))).toBe(
      true,
    );
    fetch.mockClear();
    await act(async () => {
      await signOutTenant(
        { callbackUrl: "http://school-b.localhost/" },
        new AbortController().signal,
      );
    });
    expect(window.location.href).toBe("http://school-b.localhost/");
    expect(
      fetch.mock.calls.filter(([url]) => url.endsWith("/session")),
    ).toHaveLength(0);
    otherTab.close();
  } finally {
    Object.defineProperty(window, "location", {
      configurable: true,
      value: originalLocation,
    });
  }
});

afterEach(() => {
  vi.unstubAllGlobals();
});
