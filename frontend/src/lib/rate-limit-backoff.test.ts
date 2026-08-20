import { afterEach, describe, expect, it, vi } from "vitest";

let restoreGuard: (() => void) | undefined;

async function loadGuard(backendFetch: typeof window.fetch) {
  vi.resetModules();
  const rateLimit = await import("./rate-limit-backoff");
  window.fetch = backendFetch;
  restoreGuard = rateLimit.installRateLimitFetchGuard();
  return rateLimit;
}

describe("rate-limit fetch guard", () => {
  afterEach(() => {
    restoreGuard?.();
    restoreGuard = undefined;
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("honors Retry-After and makes no network request during the lock", async () => {
    const backendFetch = vi.fn().mockResolvedValue(
      new Response("rate limited", {
        status: 429,
        headers: { "Retry-After": "17" },
      }),
    );
    const notice = vi.fn();
    window.addEventListener("phoenix:rate-limited", notice);
    await loadGuard(backendFetch);

    const first = await fetch("/api/active-supervision-dashboard");
    const blocked = await fetch("/api/me/groups/supervised");

    expect(first.status).toBe(429);
    expect(blocked.status).toBe(429);
    expect(blocked.headers.get("Retry-After")).toBe("17");
    expect(backendFetch).toHaveBeenCalledTimes(1);
    expect(notice).toHaveBeenCalledTimes(1);

    window.removeEventListener("phoenix:rate-limited", notice);
  });

  it("honors an HTTP-date Retry-After value", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2040-01-01T00:00:00Z"));
    const backendFetch = vi.fn().mockResolvedValue(
      new Response("rate limited", {
        status: 429,
        headers: { "Retry-After": "Sun, 01 Jan 2040 00:00:10 GMT" },
      }),
    );
    await loadGuard(backendFetch);

    await fetch("/api/active-supervision-dashboard");
    const blocked = await fetch("/api/me/groups/supervised");

    expect(blocked.headers.get("Retry-After")).toBe("10");
    expect(backendFetch).toHaveBeenCalledTimes(1);
  });

  it("falls back to sixty seconds for an invalid Retry-After value", async () => {
    const backendFetch = vi.fn().mockResolvedValue(
      new Response("rate limited", {
        status: 429,
        headers: { "Retry-After": "not-a-date" },
      }),
    );
    await loadGuard(backendFetch);

    await fetch("/api/active-supervision-dashboard");
    const blocked = await fetch("/api/me/groups/supervised");

    expect(blocked.headers.get("Retry-After")).toBe("60");
  });

  it("does not block SSE, auth, or cross-origin requests", async () => {
    const backendFetch = vi
      .fn()
      .mockResolvedValueOnce(
        new Response("rate limited", {
          status: 429,
          headers: { "Retry-After": "17" },
        }),
      )
      .mockResolvedValue(new Response(null));
    const rateLimit = await loadGuard(backendFetch);

    expect(rateLimit.installRateLimitFetchGuard()).toBe(restoreGuard);
    await fetch("/api/active-supervision-dashboard");
    await fetch("/api/sse/events");
    await fetch("/api/auth/session");
    await fetch("https://example.com/api/students");
    const blocked = await fetch(
      new Request(`${window.location.origin}/api/students`),
    );

    expect(backendFetch).toHaveBeenCalledTimes(4);
    expect(blocked.status).toBe(429);
  });

  it("recognizes structured and message-based 429 errors", async () => {
    const rateLimit = await loadGuard(vi.fn());

    expect(rateLimit.isRateLimitError({ status: 429 })).toBe(true);
    expect(rateLimit.isRateLimitError({ httpStatus: 429 })).toBe(true);
    expect(rateLimit.isRateLimitError({ response: { status: 429 } })).toBe(
      true,
    );
    expect(rateLimit.isRateLimitError(new Error("HTTP failed (429)"))).toBe(
      true,
    );
    expect(rateLimit.isRateLimitError(new Error("HTTP failed (500)"))).toBe(
      false,
    );
    expect(
      rateLimit.isRateLimitError(
        new Error("HTTP 500 for /students/429: failed validation"),
      ),
    ).toBe(false);
    expect(rateLimit.isRateLimitError("429")).toBe(false);
  });
});
