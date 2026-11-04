import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";
import {
  beginSessionEnd,
  fetchThroughSessionBarrier,
  isSessionBarrierClosed,
  resetSessionBarrierForTests,
} from "./session-end-barrier";
import { useSessionPollInterval } from "~/components/auth/session-account-binding";

/**
 * #5399: from the start of a logout no answer of the ending session may set
 * its cookie. The server guarantees it (session-cookie-writes); the barrier
 * of the logging-out document aborts its BFF requests whose answer has not
 * arrived, holds new ones and stops the SessionProvider's poll.
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
  // An answered request keeps running: its cookies landed before the
  // logout went on, and aborting it broke the signout's own body (the tenant
  // relogin then never navigated) and every SSE stream.
  it("leaves requests alone whose answer already arrived", async () => {
    const stream = through("/api/sse/events");
    await flush();
    sent[0]!.answer();
    await stream;

    beginSessionEnd();

    expect(sent[0]!.signal?.aborted).toBe(false);
  });

  it("keeps the signout answer readable after it deleted the cookie", async () => {
    beginSessionEnd({ keepClosed: true });
    const signout = through("/api/auth/signout", { method: "POST" });
    await flush();

    sent[0]!.answer(Response.json({ url: "/" }));
    await signout;

    expect(sent[0]!.signal?.aborted).toBe(false);
  });

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
