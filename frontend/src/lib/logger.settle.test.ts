import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.unmock("~/lib/logger");

// The log route answers with the session cookie. A logout settles every
// shipment before it deletes the cookie, so no late answer restores it (#5126).
describe("settling client log shipments before a logout", () => {
  let originalFetch: typeof globalThis.fetch;
  let fetchMock: ReturnType<typeof vi.fn<typeof fetch>>;
  let answers: (() => void)[];

  beforeEach(() => {
    vi.resetModules();
    answers = [];
    originalFetch = globalThis.fetch;
    fetchMock = vi.fn<typeof fetch>(
      () =>
        new Promise<Response>((resolve) => {
          answers.push(() => resolve(new Response(null, { status: 200 })));
        }),
    );
    globalThis.fetch = fetchMock;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  function shippedMessages(call: number): string[] {
    const body = JSON.parse(String(fetchMock.mock.calls[call]?.[1]?.body)) as {
      entries: { msg: string }[];
    };
    return body.entries.map((entry) => entry.msg);
  }

  it("ships every pending batch and waits for all open shipments", async () => {
    const { createLogger, settleClientLogShipping } = await import("./logger");
    const first = createLogger({ component: "First" });
    const second = createLogger({ component: "Second" });
    first.error("already_shipping");
    void first.flush();
    second.error("still_pending");

    let settled = false;
    const settle = settleClientLogShipping().then((release) => {
      settled = true;
      return release;
    });

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(shippedMessages(1)).toEqual(["still_pending"]);
    answers[0]?.();
    await Promise.resolve();
    await Promise.resolve();
    expect(settled).toBe(false);
    answers[1]?.();
    await settle;
    expect(settled).toBe(true);
  });

  it("holds new shipments until released", async () => {
    const { createLogger, settleClientLogShipping } = await import("./logger");
    const logger = createLogger({ component: "Held" });
    const release = await settleClientLogShipping();

    logger.error("written_while_held");
    await logger.flush();
    expect(fetchMock).not.toHaveBeenCalled();

    release();
    void logger.flush();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(shippedMessages(0)).toEqual(["written_while_held"]);
  });

  it("stops waiting for an unanswered shipment after five seconds", async () => {
    vi.useFakeTimers();
    const { createLogger, settleClientLogShipping } = await import("./logger");
    const logger = createLogger({ component: "Hanging" });
    logger.error("never_answered");

    let settled = false;
    void settleClientLogShipping().then(() => {
      settled = true;
    });
    await vi.advanceTimersByTimeAsync(4_999);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    expect(settled).toBe(true);
  });
});
