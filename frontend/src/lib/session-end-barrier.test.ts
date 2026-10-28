import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionEndDetail } from "./session-end-barrier";

vi.unmock("~/lib/session-end-barrier");

// Every authenticated BFF answer carries the session cookie again. A logout
// holds the tab's requests and waits for those in flight, so no answer lands
// after the cookie deletion and restores the ended session (#5126). When the
// tab stays after all, the barrier resets and never sends a held write.
describe("session end barrier", () => {
  let originalFetch: typeof window.fetch;
  let originalNavigation: PropertyDescriptor | undefined;
  let answers: Map<string, (status?: number) => void>;
  let sent: string[];
  let phases: SessionEndDetail[];
  const onSessionEnd = (event: Event) =>
    phases.push((event as CustomEvent<SessionEndDetail>).detail);

  beforeEach(() => {
    vi.resetModules();
    originalFetch = window.fetch;
    originalNavigation = Object.getOwnPropertyDescriptor(window, "navigation");
    answers = new Map();
    sent = [];
    phases = [];
    window.fetch = vi.fn((input: RequestInfo | URL) => {
      const url = input instanceof Request ? input.url : input.toString();
      sent.push(url);
      return new Promise<Response>((resolve) => {
        answers.set(url, (status = 200) =>
          resolve(new Response(null, { status })),
        );
      });
    });
    window.addEventListener("phoenix:session-end", onSessionEnd);
  });

  afterEach(() => {
    window.removeEventListener("phoenix:session-end", onSessionEnd);
    window.fetch = originalFetch;
    if (originalNavigation)
      Object.defineProperty(window, "navigation", originalNavigation);
    else delete (window as { navigation?: unknown }).navigation;
    vi.useRealTimers();
  });

  async function loadBarrier() {
    const barrier = await import("./session-end-barrier");
    barrier.installSessionEndBarrier();
    return barrier;
  }

  async function flush() {
    for (let i = 0; i < 5; i++) await Promise.resolve();
  }

  /** Holds, sends the sign-out and lets its answer delete the cookie. */
  async function signOutAndHold(
    barrier: Awaited<ReturnType<typeof loadBarrier>>,
  ) {
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    const write = window.fetch("/api/groups", { method: "POST" });
    const read = window.fetch("/api/me/profile");
    // Not every test awaits both; mark them handled.
    write.catch(() => undefined);
    read.catch(() => undefined);
    void window.fetch("/api/auth/signout", { method: "POST" });
    answers.get("/api/auth/signout")?.();
    await flush();
    return { cancel, write, read };
  }

  it("waits for open requests and holds new ones until sign-out deleted the cookie", async () => {
    const { holdRequestsUntilSessionEnds, isSessionEnding } =
      await loadBarrier();
    void window.fetch("/api/announcements/unread");

    let holding = false;
    const hold = holdRequestsUntilSessionEnds().then((cancel) => {
      holding = true;
      return cancel;
    });
    await flush();
    expect(holding).toBe(false);
    expect(isSessionEnding()).toBe(true);

    window.fetch("/api/auth/session").catch(() => undefined);
    expect(sent).toEqual(["/api/announcements/unread"]);

    answers.get("/api/announcements/unread")?.();
    await hold;
    expect(holding).toBe(true);

    // Auth.js' sign-out requests and other origins pass.
    void window.fetch("/api/auth/csrf");
    void window.fetch("/api/auth/signout", { method: "POST" });
    void window.fetch("https://monitoring.invalid/ingest");
    expect(sent).toEqual([
      "/api/announcements/unread",
      "/api/auth/csrf",
      "/api/auth/signout",
      "https://monitoring.invalid/ingest",
    ]);

    // After the deletion new requests leave without the cookie; held ones stay.
    answers.get("/api/auth/signout")?.();
    await flush();
    void window.fetch("/api/auth/session?after");
    expect(sent).toContain("/api/auth/session?after");
    expect(sent).not.toContain("/api/auth/session");
  });

  it("aborts the held requests on a failed logout, reports discarded writes and reopens", async () => {
    const barrier = await loadBarrier();
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    const write = window.fetch("/api/groups", { method: "POST" });
    const read = window.fetch("/api/me/profile");
    read.catch(() => undefined);

    cancel();

    const writeError = await write.catch((error: unknown) => error);
    expect(writeError).toMatchObject({ name: "AbortError" });
    expect(barrier.discardedBySessionEnd(writeError)).toBe(true);
    await expect(read).rejects.toMatchObject({ name: "AbortError" });
    expect(sent).toEqual([]);
    expect(barrier.isSessionEnding()).toBe(false);
    expect(phases).toEqual([
      { ending: true, discardedWrites: 0 },
      { ending: false, discardedWrites: 1 },
    ]);
    // A later request goes out normally.
    void window.fetch("/api/me/profile");
    expect(sent).toEqual(["/api/me/profile"]);
  });

  it("recognises a discarded request behind an error that wraps it", async () => {
    const barrier = await loadBarrier();
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    const write = window.fetch("/api/groups", { method: "PUT" });
    cancel();
    const cause = await write.catch((error: unknown) => error);
    const { unavailableApiError } = await import("./api-error");
    expect(barrier.discardedBySessionEnd(unavailableApiError(cause))).toBe(
      true,
    );
    expect(
      barrier.discardedBySessionEnd(
        new DOMException("superseded", "AbortError"),
      ),
    ).toBe(false);
  });

  it("resets when the navigation after sign-out is aborted", async () => {
    const navigation = new EventTarget();
    Object.defineProperty(window, "navigation", {
      configurable: true,
      value: navigation,
    });
    const barrier = await loadBarrier();
    const { write } = await signOutAndHold(barrier);
    expect(barrier.isSessionEnding()).toBe(true);

    navigation.dispatchEvent(new Event("navigateerror"));

    await expect(write).rejects.toMatchObject({ name: "AbortError" });
    expect(barrier.isSessionEnding()).toBe(false);
    expect(phases.at(-1)).toEqual({ ending: false, discardedWrites: 1 });
  });

  it("resets when no page change follows the sign-out within ten seconds", async () => {
    vi.useFakeTimers();
    const barrier = await loadBarrier();
    const { read } = await signOutAndHold(barrier);

    await vi.advanceTimersByTimeAsync(9_999);
    expect(barrier.isSessionEnding()).toBe(true);
    await vi.advanceTimersByTimeAsync(1);

    await expect(read).rejects.toMatchObject({ name: "AbortError" });
    expect(barrier.isSessionEnding()).toBe(false);
  });

  it("keeps holding while the page leaves and resets when the cache restores it", async () => {
    vi.useFakeTimers();
    const barrier = await loadBarrier();
    const { write } = await signOutAndHold(barrier);

    window.dispatchEvent(new Event("pagehide"));
    await vi.advanceTimersByTimeAsync(11_000);
    expect(barrier.isSessionEnding()).toBe(true);

    const pageshow = new Event("pageshow");
    Object.defineProperty(pageshow, "persisted", { value: true });
    window.dispatchEvent(pageshow);
    await expect(write).rejects.toMatchObject({ name: "AbortError" });
    expect(barrier.isSessionEnding()).toBe(false);
  });

  it("resets when a new sign-in in this document succeeds", async () => {
    const barrier = await loadBarrier();
    const { write } = await signOutAndHold(barrier);

    void window.fetch("/api/auth/callback/credentials", { method: "POST" });
    answers.get("/api/auth/callback/credentials")?.();
    await flush();

    await expect(write).rejects.toMatchObject({ name: "AbortError" });
    expect(barrier.isSessionEnding()).toBe(false);
    expect(phases.at(-1)).toEqual({ ending: false, discardedWrites: 1 });
  });

  it("stops waiting for an unanswered request after five seconds", async () => {
    vi.useFakeTimers();
    const { holdRequestsUntilSessionEnds } = await loadBarrier();
    void window.fetch("/api/sse/events");

    let holding = false;
    void holdRequestsUntilSessionEnds().then(() => {
      holding = true;
    });
    await vi.advanceTimersByTimeAsync(4_999);
    expect(holding).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(holding).toBe(true);
  });
});
