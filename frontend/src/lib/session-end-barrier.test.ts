import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  beginSessionEnd,
  fetchThroughSessionBarrier,
  isSessionBarrierClosed,
  listenForSessionEnds,
  resetSessionBarrierForTests,
} from "./session-end-barrier";
import { useSessionPollInterval } from "~/components/auth/session-account-binding";

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

/** The barrier's timers; message delivery keeps the real event loop. */
const BARRIER_TIMERS = ["setTimeout", "clearTimeout"] as const;

/** BroadcastChannel delivers on the event loop, not on (fake) timers. */
async function flushMessages() {
  for (let turn = 0; turn < 5; turn++)
    await new Promise((resolve) => setImmediate(resolve));
}

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

  it.each([
    "/api/notifications/push/subscriptions",
    "/api/parent/me/push/subscriptions",
    "/api/school/notifications/push/subscriptions",
  ])(
    "aborts running push registration and holds new registration at %s",
    async (path) => {
      const running = through(path, { method: "POST", body: "{}" });
      expect(sent).toHaveLength(1);
      const rejected = expect(running).rejects.toMatchObject({
        name: "AbortError",
      });

      const end = beginSessionEnd();
      await rejected;
      expect(sent[0]!.signal?.aborted).toBe(true);
      const held = fetchThroughSessionBarrier(
        send,
        new Request(new URL(path, location.origin), {
          method: "POST",
          body: "{}",
        }),
      );
      await flush();
      expect(sent).toHaveLength(1);

      end.fail();
      await flush();
      expect(sent).toHaveLength(2);
      sent[1]!.answer();
      await held;
    },
  );

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

  it.each([false, true])(
    "cancels a queued request before reopening (Request signal: %s)",
    async (asRequest) => {
      const end = beginSessionEnd({ keepClosed: true });
      const caller = new AbortController();
      const input = asRequest
        ? new Request(new URL("/api/me/profile", location.origin), {
            signal: caller.signal,
          })
        : "/api/me/profile";
      const held = fetchThroughSessionBarrier(
        send,
        input,
        asRequest ? undefined : { signal: caller.signal },
      );
      const reason = new DOMException("Caller cancelled", "AbortError");
      const rejected = expect(held).rejects.toBe(reason);

      caller.abort(reason);

      await rejected;
      expect(isSessionBarrierClosed()).toBe(true);
      end.fail();
      const active = through("/api/auth/session");
      await flush();
      expect(sent.map(({ url }) => url)).toEqual(["/api/auth/session"]);
      sent[0]!.answer();
      await active;
    },
  );

  it("rejects an already cancelled request while the barrier stays closed", async () => {
    beginSessionEnd({ keepClosed: true });
    const caller = new AbortController();
    caller.abort();

    await expect(
      through("/api/me/profile", { signal: caller.signal }),
    ).rejects.toBe(caller.signal.reason);
    expect(isSessionBarrierClosed()).toBe(true);
  });

  // Final review of #5403: an exit path passes only for the logout attempt
  // that sends it. Started before, it carried no abort signal, and its
  // renewing or ending answer could write the cookie after its deletion.
  it("aborts a push cleanup that started before the logout", async () => {
    const cleanup = through("/api/parent/me/push/subscriptions", {
      method: "DELETE",
    });
    await flush();

    beginSessionEnd();

    expect(sent[0]!.signal?.aborted).toBe(true);
    await expect(cleanup).rejects.toMatchObject({ name: "AbortError" });
  });

  it("aborts a CSRF lookup that started before the logout and asks again", async () => {
    const csrf = through("/api/auth/csrf");
    await flush();

    const end = beginSessionEnd();

    expect(sent[0]!.signal?.aborted).toBe(true);
    end.fail();
    await vi.waitFor(() => expect(sent).toHaveLength(2));
    sent[1]!.answer();
    await csrf;
  });

  it("ends the exit's own requests with a failed attempt", async () => {
    const end = beginSessionEnd();
    const cleanup = through("/api/parent/me/push/subscriptions", {
      method: "DELETE",
    });
    await flush();
    expect(sent[0]!.signal?.aborted).toBe(false);

    end.fail();

    expect(sent[0]!.signal?.aborted).toBe(true);
    await expect(cleanup).rejects.toMatchObject({ name: "AbortError" });
  });

  it("aborts the exit's push cleanup once the signout deleted the cookie", async () => {
    beginSessionEnd({ keepClosed: true });
    const cleanup = through("/api/notifications/push/subscriptions", {
      method: "DELETE",
    });
    const signout = through("/api/auth/signout", { method: "POST" });
    await flush();

    sent[1]!.answer(Response.json({ url: "/" }));
    await signout;

    expect(sent[0]!.signal?.aborted).toBe(true);
    await expect(cleanup).rejects.toMatchObject({ name: "AbortError" });
  });

  it("tells an asking tab that its logout still runs", async () => {
    listenForSessionEnds();
    const listener = new BroadcastChannel("moto-session-end");
    const heard: string[] = [];
    listener.onmessage = (event: MessageEvent<string>) =>
      heard.push(event.data);
    try {
      const end = beginSessionEnd();
      await vi.waitFor(() => expect(heard).toEqual(["begin"]));

      // CSRF or signout hang past the other tab's hold; it asks.
      listener.postMessage("ping");
      await vi.waitFor(() => expect(heard).toEqual(["begin", "begin"]));

      end.fail();
      listener.postMessage("ping");
      await vi.waitFor(() =>
        expect(heard).toEqual(["begin", "begin", "failed"]),
      );
      await flush();
      expect(heard).toHaveLength(3);
    } finally {
      listener.close();
    }
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

    it("stays closed while the other tab's logout still runs", async () => {
      vi.useFakeTimers({ toFake: [...BARRIER_TIMERS] });
      listenForSessionEnds();
      const owner = new BroadcastChannel("moto-session-end");
      // A live owner answers every ping, however long its signout hangs.
      owner.onmessage = (event: MessageEvent<string>) => {
        if (event.data === "ping") owner.postMessage("begin");
      };
      try {
        owner.postMessage("begin");
        await vi.waitFor(() => expect(isSessionBarrierClosed()).toBe(true));
        for (let step = 0; step < 6; step++) {
          await vi.advanceTimersByTimeAsync(10_000);
          await flushMessages();
        }

        expect(isSessionBarrierClosed()).toBe(true);
      } finally {
        owner.close();
      }
    });

    it("aborts what it sent after the hold ran out once the other tab ends", async () => {
      vi.useFakeTimers({ toFake: [...BARRIER_TIMERS] });
      listenForSessionEnds();
      const tab = otherTab();
      try {
        tab.post("begin");
        await vi.waitFor(() => expect(isSessionBarrierClosed()).toBe(true));
        // The other tab went silent: the hold runs out, no answer comes.
        await vi.advanceTimersByTimeAsync(15_000);
        await flushMessages();
        await vi.advanceTimersByTimeAsync(2_000);
        expect(isSessionBarrierClosed()).toBe(false);
        const read = through("/api/auth/session");
        await vi.waitFor(() => expect(sent).toHaveLength(1));

        tab.post("ended");

        await vi.waitFor(() => expect(sent[0]!.signal?.aborted).toBe(true));
        // The read goes out again, now without the deleted cookie.
        await vi.waitFor(() => expect(sent).toHaveLength(2));
        sent[1]!.answer();
        await read;
      } finally {
        tab.close();
      }
    });

    it("keeps local ownership past the other tab's timeout", async () => {
      vi.useFakeTimers();
      listenForSessionEnds();
      const tab = otherTab();
      try {
        tab.post("begin");
        await vi.waitFor(() => expect(isSessionBarrierClosed()).toBe(true));
        const end = beginSessionEnd({ keepClosed: true });

        await vi.advanceTimersByTimeAsync(15_000);

        expect(isSessionBarrierClosed()).toBe(true);
        end.fail();
        expect(isSessionBarrierClosed()).toBe(false);
      } finally {
        tab.close();
      }
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
