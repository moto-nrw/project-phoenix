import { afterEach, describe, expect, it, vi } from "vitest";
import { immutableConfig, swrConfig } from "./config";

// SWR types onError's third argument as a fully-resolved PublicConfiguration.
// logSWRError never reads it, so use a loose cast instead of constructing one.
type OnError = NonNullable<typeof swrConfig.onError>;
type ConfigArg = Parameters<OnError>[2];
const stubConfig = {} as ConfigArg;

describe("swrConfig.onError", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("logs the cache key and the message of an Error instance", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    swrConfig.onError?.(
      new Error("network down"),
      "tenant-slug:database-devices-list",
      stubConfig,
    );

    expect(consoleError).toHaveBeenCalledTimes(1);
    expect(consoleError).toHaveBeenCalledWith("swr_fetch_failed", {
      key: "tenant-slug:database-devices-list",
      error: "network down",
    });
  });

  it("falls back to String(err) when a non-Error is thrown", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    swrConfig.onError?.("connection refused", "key-x", stubConfig);

    expect(consoleError).toHaveBeenCalledWith("swr_fetch_failed", {
      key: "key-x",
      error: "connection refused",
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
      key: "key-y",
      error: "[object Object]",
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
      key: "tenant-slug:ogs-students-3",
      error: "Error fetching students: API error: 429",
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
