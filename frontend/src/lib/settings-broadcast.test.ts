/**
 * Tests for settings-broadcast.ts: same-origin BroadcastChannel ping
 * that lets the cache bridge react to changes from another tab on the
 * same origin.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  notifySettingsChanged,
  subscribeSettingsChanged,
} from "./settings-broadcast";

describe("settings-broadcast", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("subscribeSettingsChanged invokes the handler when notify fires", async () => {
    // happy-doom polyfills BroadcastChannel; messages are async, so wait a
    // microtask after notify before asserting.
    const handler = vi.fn();
    const unsubscribe = subscribeSettingsChanged(handler);

    notifySettingsChanged();

    // Yield to the BroadcastChannel delivery microtask.
    await new Promise((r) => setTimeout(r, 0));
    expect(handler).toHaveBeenCalledTimes(1);

    unsubscribe();
  });

  it("returns a noop unsubscribe when BroadcastChannel is unavailable", () => {
    // Some test runtimes lack BroadcastChannel; the helper must still
    // return a callable so callers don't crash on cleanup.
    const original = globalThis.BroadcastChannel;
    // @ts-expect-error — deliberately removing for the test
    globalThis.BroadcastChannel = undefined;
    try {
      const unsubscribe = subscribeSettingsChanged(() => {});
      expect(typeof unsubscribe).toBe("function");
      // No-op unsubscribe must not throw.
      expect(() => unsubscribe()).not.toThrow();
    } finally {
      globalThis.BroadcastChannel = original;
    }
  });

  it("notifySettingsChanged is a noop when BroadcastChannel is unavailable", () => {
    const original = globalThis.BroadcastChannel;
    // @ts-expect-error — deliberately removing for the test
    globalThis.BroadcastChannel = undefined;
    try {
      // Must not throw — callers fire-and-forget.
      expect(() => notifySettingsChanged()).not.toThrow();
    } finally {
      globalThis.BroadcastChannel = original;
    }
  });

  it("unsubscribe stops the handler from firing on later notifies", async () => {
    const handler = vi.fn();
    const unsubscribe = subscribeSettingsChanged(handler);

    notifySettingsChanged();
    await new Promise((r) => setTimeout(r, 0));
    expect(handler).toHaveBeenCalledTimes(1);

    unsubscribe();

    notifySettingsChanged();
    await new Promise((r) => setTimeout(r, 0));
    // Still 1 — closed channel does not deliver.
    expect(handler).toHaveBeenCalledTimes(1);
  });
});
