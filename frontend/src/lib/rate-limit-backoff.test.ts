import { afterEach, describe, expect, it, vi } from "vitest";
import { installRateLimitFetchGuard } from "./rate-limit-backoff";

describe("rate-limit fetch guard", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("honors Retry-After and makes no network request during the lock", async () => {
    const backendFetch = vi.fn().mockResolvedValue(
      new Response("rate limited", {
        status: 429,
        headers: { "Retry-After": "17" },
      }),
    );
    window.fetch = backendFetch;
    const notice = vi.fn();
    window.addEventListener("phoenix:rate-limited", notice);
    const restore = installRateLimitFetchGuard();

    const first = await fetch("/api/active-supervision-dashboard");
    const blocked = await fetch("/api/me/groups/supervised");

    expect(first.status).toBe(429);
    expect(blocked.status).toBe(429);
    expect(blocked.headers.get("Retry-After")).toBe("17");
    expect(backendFetch).toHaveBeenCalledTimes(1);
    expect(notice).toHaveBeenCalledTimes(1);

    restore();
    window.removeEventListener("phoenix:rate-limited", notice);
  });
});
