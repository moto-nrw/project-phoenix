import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "../api-error";
import { installRateLimitFetchGuard } from "../rate-limit-backoff";
import { immutableConfig, swrConfig } from "./config";

// SWR types onError's third argument as a fully-resolved PublicConfiguration.
// logSWRError never reads it, so use a loose cast instead of constructing one.
type OnError = NonNullable<typeof swrConfig.onError>;
type ConfigArg = Parameters<OnError>[2];
const stubConfig = {} as ConfigArg;
type OnErrorRetry = NonNullable<typeof swrConfig.onErrorRetry>;
type RetryOptions = Parameters<OnErrorRetry>[4];

describe("swrConfig.onError", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  // #2512: the entry carries identifiers only. The cache key can hold the
  // Kindersuche search term and the message the backend detail, so neither
  // reaches Loki; the request ID joins the entry to the server's request line.
  it("logs code, class, status and request ID of an API error", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    swrConfig.onError?.(
      new ApiError("student Anna Schmidt not visible", 403, {
        code: "general.permission",
        instance: "req-7",
      }),
      "tenant-slug:search-students-gall-Anna Schmidt",
      stubConfig,
    );

    expect(consoleError).toHaveBeenCalledTimes(1);
    expect(consoleError).toHaveBeenCalledWith("swr_fetch_failed", {
      error_class: "permission",
      error_code: "general.permission",
      status: 403,
      request_id: "req-7",
      error_name: "ApiError",
    });
    expect(JSON.stringify(consoleError.mock.calls)).not.toContain("Anna");
  });

  it("classes a dropped connection as unavailable without its text", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    swrConfig.onError?.(
      new TypeError("Failed to fetch"),
      "tenant-slug:database-devices-list",
      stubConfig,
    );

    expect(consoleError).toHaveBeenCalledWith("swr_fetch_failed", {
      error_class: "unavailable",
      error_name: "TypeError",
    });
  });

  it("logs a thrown non-Error as a crash", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    swrConfig.onError?.("connection refused", "key-x", stubConfig);

    expect(consoleError).toHaveBeenCalledWith("swr_fetch_failed", {
      error_class: "crash",
    });
  });

  it("does not throw when the error has no message field", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    expect(() =>
      swrConfig.onError?.({ status: 500 }, "key-y", stubConfig),
    ).not.toThrow();

    expect(consoleError).toHaveBeenCalledWith("swr_fetch_failed", {
      error_class: "crash",
    });
  });

  it("logs 429 errors as rate-limit warnings", () => {
    const consoleWarn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    swrConfig.onError?.(
      new Error("Error fetching students: API error: 429"),
      "tenant-slug:ogs-students-3",
      stubConfig,
    );

    expect(consoleWarn).toHaveBeenCalledWith("swr_fetch_rate_limited", {
      error_class: "unavailable",
      status: 429,
      error_name: "Error",
      rate_limited: true,
    });
    expect(consoleError).not.toHaveBeenCalled();
  });
});

describe("swrConfig defaults", () => {
  it("disables focus revalidation and keeps reconnect revalidation on", () => {
    expect(swrConfig.revalidateOnFocus).toBe(false);
    expect(swrConfig.revalidateOnReconnect).toBe(true);
  });

  it("retries failed requests three times with a one-second backoff", () => {
    expect(swrConfig.errorRetryCount).toBe(3);
    expect(swrConfig.errorRetryInterval).toBe(1000);
  });

  it("dedupes within five seconds to absorb SSE bursts", () => {
    expect(swrConfig.dedupingInterval).toBe(5000);
  });

  it("preserves previous data while revalidating", () => {
    expect(swrConfig.keepPreviousData).toBe(true);
  });
});

describe("swrConfig.onErrorRetry", () => {
  let restoreGuard: (() => void) | undefined;

  afterEach(() => {
    restoreGuard?.();
    restoreGuard = undefined;
    vi.useRealTimers();
  });

  it("waits for the browser rate-limit window before revalidating", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2040-01-01T00:00:00Z"));
    window.fetch = vi.fn().mockResolvedValue(
      new Response("rate limited", {
        status: 429,
        headers: { "Retry-After": "2" },
      }),
    );
    restoreGuard = installRateLimitFetchGuard();
    await fetch("/api/active-supervision-dashboard");
    const revalidate = vi.fn();

    swrConfig.onErrorRetry?.({ status: 429 }, "key", stubConfig, revalidate, {
      retryCount: 1,
    } as RetryOptions);
    vi.advanceTimersByTime(2049);
    expect(revalidate).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);

    expect(revalidate).toHaveBeenCalledWith({ retryCount: 1 });
  });

  it("stops rate-limit retries at the configured limit", () => {
    vi.useFakeTimers();
    const revalidate = vi.fn();

    swrConfig.onErrorRetry?.({ status: 429 }, "key", stubConfig, revalidate, {
      retryCount: 3,
    } as RetryOptions);
    vi.runAllTimers();

    expect(revalidate).not.toHaveBeenCalled();
  });

  it("backs off ordinary retries and stops after the configured limit", () => {
    vi.useFakeTimers();
    const revalidate = vi.fn();

    swrConfig.onErrorRetry?.(
      new Error("network down"),
      "key",
      stubConfig,
      revalidate,
      { retryCount: 2 } as RetryOptions,
    );
    vi.advanceTimersByTime(1999);
    expect(revalidate).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(revalidate).toHaveBeenCalledWith({ retryCount: 2 });

    swrConfig.onErrorRetry?.(
      new Error("network down"),
      "key",
      stubConfig,
      revalidate,
      { retryCount: 3 } as RetryOptions,
    );
    vi.runAllTimers();
    expect(revalidate).toHaveBeenCalledTimes(1);
  });
});

describe("immutableConfig", () => {
  it("inherits the shared onError sink from swrConfig", () => {
    expect(immutableConfig.onError).toBe(swrConfig.onError);
  });

  it("turns off all revalidation triggers for static data", () => {
    expect(immutableConfig.revalidateIfStale).toBe(false);
    expect(immutableConfig.revalidateOnFocus).toBe(false);
    expect(immutableConfig.revalidateOnReconnect).toBe(false);
  });

  it("uses a five-minute dedupe window for truly static data", () => {
    expect(immutableConfig.dedupingInterval).toBe(5 * 60 * 1000);
  });
});
