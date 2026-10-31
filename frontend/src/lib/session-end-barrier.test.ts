import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  beginSessionEnd,
  fetchThroughSessionBarrier,
  isSessionBarrierClosed,
  listenForSessionEnds,
  resetSessionBarrierForTests,
} from "./session-end-barrier";
import { useSessionPollInterval } from "./use-session-poll-interval";

/**
 * #5399: from the start of a logout no answer of the ending session may set
 * its cookie. The barrier aborts the tab's BFF requests whose answer has not
 * arrived (the browser then drops their Set-Cookie), holds new ones and stops
 * the SessionProvider's poll, here and in the other tabs of this origin.
 */

interface Sent {
  url: string;
  signal: AbortSignal | null | undefined;
  answer: (response?: Response) => void;
}

let sent: Sent[] = [];

/** A transport whose answers the test releases; an abort rejects at once. */
const send: typeof fetch = (input, init) =>
  new Promise<Response>((resolve, reject) => {
    const signal = init?.signal;
    if (signal?.aborted) {
      reject(signal.reason);
      return;
    }
    signal?.addEventListener("abort", () => reject(signal.reason), {
      once: true,
    });
    sent.push({
      url: input instanceof Request ? input.url : input.toString(),
      signal,
      answer: (response = new Response("{}")) => resolve(response),
    });
  });

function through(path: string, init?: RequestInit) {
  return fetchThroughSessionBarrier(send, path, init);
}

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

/** Another tab of the same origin. */
function otherTab() {
  const channel = new BroadcastChannel("moto-session-end");
  return {
    post: (type: "begin" | "ended" | "failed") => channel.postMessage(type),
    close: () => channel.close(),
  };
}

beforeEach(() => {
  sent = [];
  resetSessionBarrierForTests();
});
afterEach(() => {
  resetSessionBarrierForTests();
});

describe("session-end barrier", () => {
  it("aborts a running read at logout start and asks again when the logout fails", async () => {
    const result = through("/api/auth/session");
    await flush();
    expect(sent).toHaveLength(1);
    const first = sent[0]!;

    const end = beginSessionEnd();

    expect(first.signal?.aborted).toBe(true);
    await flush();
    // Not sent again while the logout runs.
    expect(sent).toHaveLength(1);

    end.fail();
    await flush();
    expect(sent).toHaveLength(2);
    sent[1]!.answer(Response.json({ ok: true }));
    await expect((await result).json()).resolves.toEqual({ ok: true });
  });

  it("fails an aborted write instead of sending it twice", async () => {
    const result = through("/api/groups", { method: "POST", body: "{}" });
    await flush();

    beginSessionEnd();

    await expect(result).rejects.toMatchObject({ name: "AbortError" });
    expect(sent).toHaveLength(1);
  });

  it("holds new requests but lets the exit's own requests pass", async () => {
    beginSessionEnd();

    const held = through("/api/me/profile");
    for (const [path, method] of [
      ["/api/auth/logout", "POST"],
      ["/api/auth/csrf", "GET"],
      ["/api/parent/auth/logout", "POST"],
      ["/api/auth/staff-preview/end", "POST"],
      ["/api/notifications/push/subscriptions", "DELETE"],
      // Each portal's push cleanup, awaited by the parent and school logout.
      ["/api/parent/me/push/subscriptions", "DELETE"],
      ["/api/school/notifications/push/subscriptions", "DELETE"],
      ["/api/parent/auth/signout", "POST"],
    ] as const) {
      void through(path, { method });
    }
    await flush();

    expect(sent.map(({ url }) => url)).toEqual([
      "/api/auth/logout",
      "/api/auth/csrf",
      "/api/parent/auth/logout",
      "/api/auth/staff-preview/end",
      "/api/notifications/push/subscriptions",
      "/api/parent/me/push/subscriptions",
      "/api/school/notifications/push/subscriptions",
      "/api/parent/auth/signout",
    ]);
    // Everything else waits, also a lookup of another portal's session.
    void through("/api/parent/auth/session");
    void through("/api/parent/me/children");
    await flush();
    expect(sent).toHaveLength(8);
    expect(isSessionBarrierClosed()).toBe(true);
    void held;
  });

  it("leaves other origins and non-API paths alone", async () => {
    beginSessionEnd();

    void through("https://cdn.example.invalid/api/file");
    void through("/home");
    await flush();

    expect(sent).toHaveLength(2);
  });

  it("opens once the signout answer deleted the cookie", async () => {
    beginSessionEnd();
    const held = through("/api/auth/session");

    const signout = through("/api/auth/signout", { method: "POST" });
    await flush();
    sent.at(-1)!.answer(Response.json({ url: "/" }));
    await signout;
    await flush();

    expect(isSessionBarrierClosed()).toBe(false);
    expect(sent.map(({ url }) => url)).toContain("/api/auth/session");
    void held;
  });

  it("stays closed after the signout when the owner navigates itself", async () => {
    const end = beginSessionEnd({ keepClosed: true });
    void through("/api/auth/session");

    const signout = through("/api/auth/signout", { method: "POST" });
    await flush();
    sent.at(-1)!.answer(Response.json({ url: "/" }));
    await signout;
    await flush();

    expect(isSessionBarrierClosed()).toBe(true);
    expect(sent.map(({ url }) => url)).toEqual(["/api/auth/signout"]);
    end.fail();
    expect(isSessionBarrierClosed()).toBe(false);
  });

  it("fails a held request whose caller gave up once the barrier opens", async () => {
    const end = beginSessionEnd();
    const caller = new AbortController();
    const held = through("/api/me/profile", { signal: caller.signal });

    caller.abort();
    end.fail();

    await expect(held).rejects.toMatchObject({ name: "AbortError" });
  });

  describe("another tab of this origin", () => {
    it("closes with the other tab's logout and opens when it reports the end", async () => {
      listenForSessionEnds();
      const running = through("/api/auth/session");
      await flush();
      const tab = otherTab();

      tab.post("begin");
      await vi.waitFor(() => expect(isSessionBarrierClosed()).toBe(true));
      expect(sent[0]!.signal?.aborted).toBe(true);

      tab.post("ended");
      await vi.waitFor(() => expect(isSessionBarrierClosed()).toBe(false));
      // The read goes out again, now without the deleted cookie.
      await vi.waitFor(() => expect(sent).toHaveLength(2));
      sent[1]!.answer();
      await running;
      tab.close();
    });

    it("opens when the other tab's logout fails", async () => {
      listenForSessionEnds();
      const tab = otherTab();

      tab.post("begin");
      await vi.waitFor(() => expect(isSessionBarrierClosed()).toBe(true));
      tab.post("failed");

      await vi.waitFor(() => expect(isSessionBarrierClosed()).toBe(false));
      tab.close();
    });

    it("tells the other tabs when this tab starts and fails a logout", async () => {
      listenForSessionEnds();
      const listener = new BroadcastChannel("moto-session-end");
      const heard: string[] = [];
      listener.onmessage = (event: MessageEvent<string>) =>
        heard.push(event.data);

      const end = beginSessionEnd();
      end.fail();

      await vi.waitFor(() => expect(heard).toEqual(["begin", "failed"]));
      listener.close();
    });
  });
});

describe("useSessionPollInterval", () => {
  it("stops the SessionProvider's poll while a logout runs", () => {
    const { result } = renderHook(() => useSessionPollInterval(240));
    expect(result.current).toBe(240);

    let end!: ReturnType<typeof beginSessionEnd>;
    act(() => {
      end = beginSessionEnd();
    });
    expect(result.current).toBe(0);

    act(() => end.fail());
    expect(result.current).toBe(240);
  });
});
