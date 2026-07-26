import { readFileSync } from "node:fs";
import path from "node:path";
import { runInNewContext } from "node:vm";
import { describe, expect, it, vi } from "vitest";

type PushEvent = {
  data: { json: () => Record<string, string> };
  waitUntil: (promise: Promise<unknown>) => void;
};

describe("web push service worker", () => {
  it("keeps distinct notifications visible when they share an event type", async () => {
    const listeners = new Map<string, (event: PushEvent) => void>();
    const showNotification = vi.fn(
      async (_title: string, _options: Record<string, unknown>) => undefined,
    );
    const worker = {
      addEventListener: (
        type: string,
        listener: (event: PushEvent) => void,
      ) => {
        listeners.set(type, listener);
      },
      clients: {
        claim: vi.fn(async () => undefined),
        matchAll: vi.fn(async () => []),
        openWindow: vi.fn(async () => undefined),
      },
      registration: { showNotification },
      skipWaiting: vi.fn(async () => undefined),
    };

    const source = readFileSync(
      path.join(process.cwd(), "public/sw.js"),
      "utf8",
    );
    runInNewContext(source, { self: worker });

    const push = listeners.get("push");
    expect(push).toBeDefined();

    const pending: Promise<unknown>[] = [];
    for (const title of ["Erste Mitteilung", "Zweite Mitteilung"]) {
      push?.({
        data: {
          json: () => ({
            title,
            body: "Neue Mitteilung",
            deepLink: "/parents/announcements",
            type: "parent_announcement",
          }),
        },
        waitUntil: (promise) => pending.push(promise),
      });
    }
    await Promise.all(pending);

    expect(showNotification).toHaveBeenCalledTimes(2);
    for (const [, options] of showNotification.mock.calls) {
      expect(options).not.toHaveProperty("tag");
    }
  });
});
