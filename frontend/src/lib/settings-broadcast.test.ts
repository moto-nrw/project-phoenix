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

type BroadcastHandler = ((event: MessageEvent) => void) | null;

class FakeBroadcastChannel {
  private static channels = new Map<string, Set<FakeBroadcastChannel>>();

  onmessage: BroadcastHandler = null;

  constructor(private readonly name: string) {
    const channels = FakeBroadcastChannel.channels.get(name) ?? new Set();
    channels.add(this);
    FakeBroadcastChannel.channels.set(name, channels);
  }

  postMessage(data: unknown) {
    const channels = FakeBroadcastChannel.channels.get(this.name) ?? new Set();
    for (const channel of channels) {
      if (channel !== this) {
        channel.onmessage?.({ data } as MessageEvent);
      }
    }
  }

  close() {
    FakeBroadcastChannel.channels.get(this.name)?.delete(this);
  }

  static reset() {
    FakeBroadcastChannel.channels.clear();
  }
}

describe("settings-broadcast", () => {
  const originalBroadcastChannel = globalThis.BroadcastChannel;

  beforeEach(() => {
    vi.restoreAllMocks();
    FakeBroadcastChannel.reset();
    globalThis.BroadcastChannel =
      FakeBroadcastChannel as unknown as typeof BroadcastChannel;
  });

  afterEach(() => {
    vi.restoreAllMocks();
    FakeBroadcastChannel.reset();
    globalThis.BroadcastChannel = originalBroadcastChannel;
  });

  it("subscribeSettingsChanged invokes the handler when notify fires", () => {
    const handler = vi.fn();
    const unsubscribe = subscribeSettingsChanged(handler);

    notifySettingsChanged();

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

  it("unsubscribe stops the handler from firing on later notifies", () => {
    const handler = vi.fn();
    const unsubscribe = subscribeSettingsChanged(handler);

    notifySettingsChanged();
    expect(handler).toHaveBeenCalledTimes(1);

    unsubscribe();

    notifySettingsChanged();
    // Still 1 — closed channel does not deliver.
    expect(handler).toHaveBeenCalledTimes(1);
  });
});
