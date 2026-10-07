import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.unmock("~/lib/logger");

// A 429 from the log route pauses log shipping for every logger instance
// until Retry-After has passed. The dropped batches never reach the network,
// so the log quota cannot keep the user in a rate-limit loop (#3884).
describe("client logger on a log-route 429", () => {
  let originalFetch: typeof globalThis.fetch;
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;

  beforeEach(() => {
    vi.useFakeTimers({ now: new Date("2026-10-07T10:00:00Z") });
    vi.resetModules();
    originalFetch = globalThis.fetch;
    fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        new Response(null, { status: 429, headers: { "Retry-After": "60" } }),
      )
      .mockResolvedValue(new Response(null, { status: 200 }));
    globalThis.fetch = fetchMock;
    vi.spyOn(console, "warn").mockImplementation(() => undefined);
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  async function ship(logger: {
    error(msg: string): void;
    flush(): Promise<void>;
  }) {
    logger.error("rate_limit_probe");
    await logger.flush();
  }

  it("drops batches from all loggers until Retry-After has passed", async () => {
    const { createLogger } = await import("./logger");
    const first = createLogger({ component: "First" });
    const second = createLogger({ component: "Second" });

    await ship(first);
    await ship(first);
    await ship(second);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(59_000);
    await ship(second);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(1_000);
    await ship(second);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    const body = JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body)) as {
      entries: { msg: string; component: string }[];
    };
    expect(body.entries).toEqual([
      expect.objectContaining({ msg: "rate_limit_probe", component: "Second" }),
    ]);
  });
});
