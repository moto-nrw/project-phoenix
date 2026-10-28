import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionEndDetail } from "./session-end-barrier";

vi.unmock("~/lib/session-end-barrier");
vi.mock("./api-url", () => ({ resolveApiUrl: async (path: string) => path }));

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
    // Like the browser, an aborted signal rejects with its reason.
    window.fetch = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = input instanceof Request ? input.url : input.toString();
      sent.push(url);
      return new Promise<Response>((resolve, reject) => {
        answers.set(url, (status = 200) =>
          resolve(new Response(null, { status })),
        );
        const signal = init?.signal;
        if (signal?.aborted) reject(signal.reason);
        signal?.addEventListener("abort", () => reject(signal.reason));
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

  it("honors init and Request cancellation without counting cancelled writes", async () => {
    const barrier = await loadBarrier();
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    const alreadyAborted = new AbortController();
    alreadyAborted.abort();
    await expect(
      window.fetch("/api/groups", {
        method: "POST",
        signal: alreadyAborted.signal,
      }),
    ).rejects.toBe(alreadyAborted.signal.reason);
    for (const useRequest of [false, true]) {
      const controller = new AbortController();
      const options = { method: "POST", signal: controller.signal };
      const request = useRequest
        ? window.fetch(
            new Request(new URL("/api/groups", window.location.href), options),
          )
        : window.fetch("/api/groups", options);
      controller.abort();
      await expect(request).rejects.toBe(controller.signal.reason);
    }
    cancel();
    expect(sent).toEqual([]);
    expect(phases.at(-1)).toEqual({ ending: false, discardedWrites: 0 });
  });

  it("holds background logs without reporting them as discarded user changes", async () => {
    const barrier = await loadBarrier();
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    const logs = window.fetch("/api/logs?batch=1", { method: "POST" });
    cancel();
    await expect(logs).rejects.toMatchObject({ name: "AbortError" });
    expect(sent).toEqual([]);
    expect(phases.at(-1)).toEqual({ ending: false, discardedWrites: 0 });
  });

  it("retains discarded identity through the student update service", async () => {
    const barrier = await loadBarrier();
    const { studentService } = await import("./api");
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    const update = studentService.updateStudent("1", {});
    await flush();
    cancel();
    const error = await update.catch((cause: unknown) => cause);
    expect(error).toBeInstanceOf(Error);
    expect(barrier.discardedBySessionEnd(error)).toBe(true);
    expect(sent).toEqual([]);
    expect(phases.at(-1)).toEqual({ ending: false, discardedWrites: 1 });
  });

  it("recognises a discarded student list request through both service wrappers", async () => {
    const barrier = await loadBarrier();
    const { studentService } = await import("./api");
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    const students = studentService.getStudents();
    await flush();
    cancel();
    const error = await students.catch((cause: unknown) => cause);
    expect(error).toBeInstanceOf(Error);
    expect(error).toMatchObject({ cause: { cause: { name: "AbortError" } } });
    expect(barrier.discardedBySessionEnd(error)).toBe(true);
    expect(sent).toEqual([]);
  });

  it("stops at cyclic and non-object causes without marking unrelated errors", async () => {
    const barrier = await loadBarrier();
    const first = new Error("first");
    const second = new Error("second", { cause: first });
    first.cause = second;
    expect(barrier.discardedBySessionEnd(first)).toBe(false);
    expect(
      barrier.discardedBySessionEnd(new Error("plain", { cause: "offline" })),
    ).toBe(false);
    expect(barrier.discardedBySessionEnd(null)).toBe(false);
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

  it("aborts a hanging sign-out after ten seconds, resets and reports the logout as unavailable", async () => {
    vi.useFakeTimers();
    const barrier = await loadBarrier();
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    const write = window.fetch("/api/groups", { method: "POST" });
    write.catch(() => undefined);
    const signOut = window.fetch("/api/auth/signout", { method: "POST" });
    signOut.catch(() => undefined);

    await vi.advanceTimersByTimeAsync(9_999);
    expect(barrier.isSessionEnding()).toBe(true);
    await vi.advanceTimersByTimeAsync(1);

    const failure = await signOut.catch((error: unknown) => error);
    expect(failure).toMatchObject({ code: "general.unavailable", status: 503 });
    // The person sees this failure; it is not a silently discarded request.
    expect(barrier.discardedBySessionEnd(failure)).toBe(false);
    await expect(write).rejects.toMatchObject({ name: "AbortError" });
    expect(barrier.isSessionEnding()).toBe(false);
    expect(phases.at(-1)).toEqual({ ending: false, discardedWrites: 1 });
    // Requests flow again.
    void window.fetch("/api/me/profile");
    expect(sent).toContain("/api/me/profile");
    cancel();
  });

  it("refuses this logout's late sign-out after a hanging csrf request, until the caller cancels", async () => {
    vi.useFakeTimers();
    const barrier = await loadBarrier();
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    // Auth.js swallows a failed csrf lookup and posts the sign-out anyway.
    const csrf = window.fetch("/api/auth/csrf");
    csrf.catch(() => undefined);
    await vi.advanceTimersByTimeAsync(10_000);
    await expect(csrf).rejects.toMatchObject({ code: "general.unavailable" });

    await expect(
      window.fetch("/api/auth/signout", { method: "POST" }),
    ).rejects.toMatchObject({ code: "general.unavailable" });
    expect(sent).toEqual(["/api/auth/csrf"]);

    // The caller's failed logout cancels; a retry's sign-out goes out.
    cancel();
    void window.fetch("/api/auth/signout", { method: "POST" });
    expect(sent).toEqual(["/api/auth/csrf", "/api/auth/signout"]);
  });

  it("aborts a session lookup still hanging after the cookie deletion", async () => {
    vi.useFakeTimers();
    const barrier = await loadBarrier();
    await signOutAndHold(barrier);
    // signOut({ redirect: false }) refetches the session before it returns.
    const session = window.fetch("/api/auth/session");
    session.catch(() => undefined);

    await vi.advanceTimersByTimeAsync(10_000);

    await expect(session).rejects.toMatchObject({
      code: "general.unavailable",
    });
    expect(barrier.isSessionEnding()).toBe(false);
  });

  // Invariants (#5126): every transition acts only on its own generation, and
  // the barrier opens exactly once per generation.
  const openings = () => phases.filter((detail) => !detail.ending).length;

  it("ignores a stale cancel of an earlier generation", async () => {
    const barrier = await loadBarrier();
    const cancelFirst = await barrier.holdRequestsUntilSessionEnds();
    const cancelSecond = await barrier.holdRequestsUntilSessionEnds();
    expect(openings()).toBe(1);

    cancelFirst();
    expect(barrier.isSessionEnding()).toBe(true);
    expect(openings()).toBe(1);

    cancelSecond();
    expect(barrier.isSessionEnding()).toBe(false);
    expect(openings()).toBe(2);
  });

  it("ignores a late sign-out answer of an earlier generation", async () => {
    const barrier = await loadBarrier();
    const cancelFirst = await barrier.holdRequestsUntilSessionEnds();
    const lateSignOut = window.fetch("/api/auth/signout", { method: "POST" });
    cancelFirst();
    const holding = barrier.holdRequestsUntilSessionEnds();
    // The first generation's sign-out answers while the second one ends. Its
    // cancel aborted it, so it rejects; either way it must not touch the second.
    answers.get("/api/auth/signout")?.();
    await lateSignOut.catch(() => undefined);
    const cancelSecond = await holding;
    await flush();

    // Still ending: a read is held, not sent.
    window.fetch("/api/me/profile").catch(() => undefined);
    expect(sent).not.toContain("/api/me/profile");
    cancelSecond();
    expect(openings()).toBe(2);
  });

  it("opens once when the deadline and a late sign-out answer meet", async () => {
    vi.useFakeTimers();
    const barrier = await loadBarrier();
    const cancel = await barrier.holdRequestsUntilSessionEnds();
    const signOut = window.fetch("/api/auth/signout", { method: "POST" });
    signOut.catch(() => undefined);

    await vi.advanceTimersByTimeAsync(10_000);
    answers.get("/api/auth/signout")?.();
    await flush();
    cancel();
    await vi.advanceTimersByTimeAsync(20_000);

    await expect(signOut).rejects.toMatchObject({
      code: "general.unavailable",
    });
    expect(barrier.isSessionEnding()).toBe(false);
    expect(openings()).toBe(1);
  });

  it("does not let a navigation timer of an earlier generation open the current one", async () => {
    vi.useFakeTimers();
    const barrier = await loadBarrier();
    await signOutAndHold(barrier);
    await vi.advanceTimersByTimeAsync(5_000);
    const cancelSecond = await barrier.holdRequestsUntilSessionEnds();
    expect(openings()).toBe(1);

    // The first generation's ten-second navigation watch has passed.
    await vi.advanceTimersByTimeAsync(6_000);
    expect(barrier.isSessionEnding()).toBe(true);
    expect(openings()).toBe(1);
    cancelSecond();
    expect(openings()).toBe(2);
  });

  it("opens once when the cache restores the page and the logout fails late", async () => {
    const barrier = await loadBarrier();
    const { cancel } = await signOutAndHold(barrier);
    const pageshow = new Event("pageshow");
    Object.defineProperty(pageshow, "persisted", { value: true });
    window.dispatchEvent(pageshow);
    cancel();
    expect(openings()).toBe(1);
  });

  it("opens once when a new sign-in succeeds and the logout fails late", async () => {
    const barrier = await loadBarrier();
    const { cancel } = await signOutAndHold(barrier);
    void window.fetch("/api/auth/callback/credentials", { method: "POST" });
    answers.get("/api/auth/callback/credentials")?.();
    await flush();
    cancel();
    expect(openings()).toBe(1);
  });

  // Every way out of a session end aborts what still hangs and frees the one
  // logout of the tab, so a new logout can start (#5126).
  describe("every exit with a hanging request frees the logout", () => {
    async function flightModule() {
      return import("./session-end-flight");
    }

    /** A logout like the real ones: hold, sign out, then (left) refetch. */
    async function hangingLogout(
      barrier: Awaited<ReturnType<typeof loadBarrier>>,
      stage: "sign-out" | "after-deletion",
    ) {
      const { endSessionOnce } = await flightModule();
      let cancelled = false;
      const logout = endSessionOnce(async () => {
        const cancel = await barrier.holdRequestsUntilSessionEnds();
        try {
          const signOut = window.fetch("/api/auth/signout", { method: "POST" });
          if (stage === "after-deletion") {
            answers.get("/api/auth/signout")?.();
            await signOut;
            // signOut({ redirect: false }) refetches the session; it hangs.
            await window.fetch("/api/auth/session");
          } else {
            await signOut;
          }
        } catch (error) {
          cancelled = true;
          cancel();
          throw error;
        }
      });
      logout.catch(() => undefined);
      await flush();
      await flush();
      return { logout, wasCancelled: () => cancelled };
    }

    async function expectNewLogoutStarts() {
      const { endSessionOnce } = await flightModule();
      const next = vi.fn(() => Promise.resolve());
      await endSessionOnce(next);
      expect(next).toHaveBeenCalledOnce();
    }

    it("on a failed logout", async () => {
      const barrier = await loadBarrier();
      const { endSessionOnce } = await flightModule();
      let csrf!: Promise<Response>;
      const failure = new Error("sign-out failed");
      const logout = endSessionOnce(async () => {
        const cancel = await barrier.holdRequestsUntilSessionEnds();
        // The sign-out's csrf lookup hangs while the logout fails.
        csrf = window.fetch("/api/auth/csrf");
        csrf.catch(() => undefined);
        cancel();
        throw failure;
      });
      await expect(logout).rejects.toBe(failure);
      await expect(csrf).rejects.toMatchObject({
        code: "general.unavailable",
      });
      expect(barrier.isSessionEnding()).toBe(false);
      await expectNewLogoutStarts();
    });

    it("on the deadline", async () => {
      vi.useFakeTimers();
      const barrier = await loadBarrier();
      const { logout, wasCancelled } = await hangingLogout(barrier, "sign-out");
      await vi.advanceTimersByTimeAsync(10_000);
      await expect(logout).rejects.toMatchObject({
        code: "general.unavailable",
      });
      expect(wasCancelled()).toBe(true);
      expect(barrier.isSessionEnding()).toBe(false);
      await expectNewLogoutStarts();
    });

    it("on a bfcache restore", async () => {
      const barrier = await loadBarrier();
      const { logout } = await hangingLogout(barrier, "sign-out");
      const pageshow = new Event("pageshow");
      Object.defineProperty(pageshow, "persisted", { value: true });
      window.dispatchEvent(pageshow);
      await expect(logout).rejects.toMatchObject({
        code: "general.unavailable",
      });
      expect(barrier.isSessionEnding()).toBe(false);
      await expectNewLogoutStarts();
    });

    it("on an aborted navigation", async () => {
      const navigation = new EventTarget();
      Object.defineProperty(window, "navigation", {
        configurable: true,
        value: navigation,
      });
      const barrier = await loadBarrier();
      const { logout } = await hangingLogout(barrier, "after-deletion");
      navigation.dispatchEvent(new Event("navigateerror"));
      await expect(logout).rejects.toMatchObject({
        code: "general.unavailable",
      });
      expect(barrier.isSessionEnding()).toBe(false);
      await expectNewLogoutStarts();
    });

    it("when no page change follows", async () => {
      vi.useFakeTimers();
      const barrier = await loadBarrier();
      const { logout } = await hangingLogout(barrier, "after-deletion");
      await vi.advanceTimersByTimeAsync(10_000);
      await expect(logout).rejects.toMatchObject({
        code: "general.unavailable",
      });
      await expectNewLogoutStarts();
    });

    it("on a new sign-in", async () => {
      const barrier = await loadBarrier();
      const { logout } = await hangingLogout(barrier, "after-deletion");
      void window.fetch("/api/auth/callback/credentials", { method: "POST" });
      answers.get("/api/auth/callback/credentials")?.();
      await expect(logout).rejects.toMatchObject({
        code: "general.unavailable",
      });
      expect(barrier.isSessionEnding()).toBe(false);
      await expectNewLogoutStarts();
    });

    it("frees the slot even if the logout itself never settles", async () => {
      const barrier = await loadBarrier();
      const { endSessionOnce } = await flightModule();
      // A logout stuck in its own code, after the barrier ended.
      void endSessionOnce(async () => {
        await barrier.holdRequestsUntilSessionEnds();
        await new Promise(() => undefined);
      });
      await flush();
      const pageshow = new Event("pageshow");
      Object.defineProperty(pageshow, "persisted", { value: true });
      window.dispatchEvent(pageshow);
      await expectNewLogoutStarts();
    });
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
