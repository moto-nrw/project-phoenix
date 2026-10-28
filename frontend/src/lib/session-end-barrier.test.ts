import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.unmock("~/lib/session-end-barrier");

// Every authenticated BFF answer carries the session cookie again. A logout
// holds the tab's requests and waits for those in flight, so no answer lands
// after the cookie deletion and restores the ended session (#5126).
describe("session end barrier", () => {
  let originalFetch: typeof window.fetch;
  let answers: Map<string, () => void>;
  let sent: string[];

  beforeEach(() => {
    vi.resetModules();
    originalFetch = window.fetch;
    answers = new Map();
    sent = [];
    window.fetch = vi.fn((input: RequestInfo | URL) => {
      const url = input instanceof Request ? input.url : input.toString();
      sent.push(url);
      return new Promise<Response>((resolve) => {
        answers.set(url, () => resolve(new Response(null, { status: 200 })));
      });
    });
  });

  afterEach(() => {
    window.fetch = originalFetch;
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

  it("waits for open requests and holds new ones until sign-out deleted the cookie", async () => {
    const { holdRequestsUntilSessionEnds, isSessionEnding } =
      await loadBarrier();
    void window.fetch("/api/announcements/unread");

    let holding = false;
    const hold = holdRequestsUntilSessionEnds().then((release) => {
      holding = true;
      return release;
    });
    await flush();
    expect(holding).toBe(false);
    expect(isSessionEnding()).toBe(true);

    void window.fetch("/api/auth/session");
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

  it("sends the held requests and reopens Server-Sent Events on release", async () => {
    const { holdRequestsUntilSessionEnds, isSessionEnding, SESSION_END_EVENT } =
      await loadBarrier();
    const phases: boolean[] = [];
    const listener = (event: Event) =>
      phases.push((event as CustomEvent<{ ending: boolean }>).detail.ending);
    window.addEventListener(SESSION_END_EVENT, listener);

    const release = await holdRequestsUntilSessionEnds();
    const held = window.fetch("/api/me/profile");
    expect(sent).toEqual([]);

    release();
    expect(isSessionEnding()).toBe(false);
    expect(sent).toEqual(["/api/me/profile"]);
    answers.get("/api/me/profile")?.();
    await expect(held).resolves.toMatchObject({ status: 200 });
    expect(phases).toEqual([true, false]);
    window.removeEventListener(SESSION_END_EVENT, listener);
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
