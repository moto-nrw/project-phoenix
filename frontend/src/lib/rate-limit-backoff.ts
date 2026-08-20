const FALLBACK_RETRY_SECONDS = 60;

let blockedUntil = 0;
let restoreFetch: (() => void) | null = null;

function retryAfterSeconds(value: string | null, now: number): number {
  if (value) {
    const seconds = Number(value);
    if (Number.isFinite(seconds) && seconds > 0) return Math.ceil(seconds);
    const date = Date.parse(value);
    if (Number.isFinite(date) && date > now) {
      return Math.max(1, Math.ceil((date - now) / 1000));
    }
  }
  return FALLBACK_RETRY_SECONDS;
}

function guardedApiRequest(input: RequestInfo | URL): boolean {
  if (typeof window === "undefined") return false;
  const raw = input instanceof Request ? input.url : input.toString();
  const url = new URL(raw, window.location.origin);
  return (
    url.origin === window.location.origin &&
    url.pathname.startsWith("/api/") &&
    !url.pathname.startsWith("/api/sse/") &&
    !url.pathname.includes("/auth/")
  );
}

export function remainingRateLimitMs(now = Date.now()): number {
  return Math.max(0, blockedUntil - now);
}

export function isRateLimitError(error: unknown): boolean {
  if (typeof error === "object" && error !== null) {
    const status =
      "status" in error
        ? error.status
        : "httpStatus" in error
          ? error.httpStatus
          : undefined;
    if (status === 429) return true;
  }
  return (
    error instanceof Error &&
    /(?:API error|HTTP|failed)\s*:?\s*\(?429/i.test(error.message)
  );
}

export function installRateLimitFetchGuard(): () => void {
  if (typeof window === "undefined") return () => {};
  if (restoreFetch) return restoreFetch;

  const originalFetch = window.fetch.bind(window);
  const guardedFetch: typeof window.fetch = async (input, init) => {
    if (guardedApiRequest(input) && remainingRateLimitMs() > 0) {
      const seconds = Math.max(1, Math.ceil(remainingRateLimitMs() / 1000));
      return new Response(JSON.stringify({ error: "Bitte kurz warten." }), {
        status: 429,
        headers: {
          "Content-Type": "application/json",
          "Retry-After": seconds.toString(),
        },
      });
    }

    const response = await originalFetch(input, init);
    if (guardedApiRequest(input) && response.status === 429) {
      const now = Date.now();
      const seconds = retryAfterSeconds(
        response.headers.get("Retry-After"),
        now,
      );
      blockedUntil = Math.max(blockedUntil, now + seconds * 1000);
      window.dispatchEvent(
        new CustomEvent("phoenix:rate-limited", { detail: { seconds } }),
      );
    }
    return response;
  };

  window.fetch = guardedFetch;
  restoreFetch = () => {
    if (window.fetch === guardedFetch) window.fetch = originalFetch;
    restoreFetch = null;
  };
  return restoreFetch;
}
