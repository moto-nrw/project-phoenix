/**
 * Tests for useSettingsCacheBridge: invalidates the SETTINGS_SCHEMA_SWR_KEY
 * SWR cache when either signal fires:
 *   1. same-origin BroadcastChannel ping (notifySettingsChanged)
 *   2. phoenix:tenant-settings-stale window event (cross-origin SSE)
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockMutate = vi.fn();

vi.mock("swr", () => ({
  mutate: (...args: unknown[]) => mockMutate(...args),
}));

const subscribers: Array<() => void> = [];
const mockUnsubscribe = vi.fn();
vi.mock("~/lib/settings-broadcast", () => ({
  subscribeSettingsChanged: (handler: () => void) => {
    subscribers.push(handler);
    return mockUnsubscribe;
  },
}));

import { useSettingsCacheBridge } from "../use-settings-cache-bridge";

describe("useSettingsCacheBridge", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    subscribers.length = 0;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("invalidates the schema SWR cache on a BroadcastChannel notification", () => {
    renderHook(() => useSettingsCacheBridge());

    expect(subscribers).toHaveLength(1);
    subscribers[0]!();

    expect(mockMutate).toHaveBeenCalledWith("test-tenant:settings-schema");
  });

  it("revalidates effective request access when tenant settings change", () => {
    renderHook(() => useSettingsCacheBridge());

    subscribers[0]!();

    expect(mockMutate).toHaveBeenCalledWith(
      "test-tenant:change-request-access",
    );
  });

  it("invalidates the schema SWR cache on phoenix:tenant-settings-stale", () => {
    renderHook(() => useSettingsCacheBridge());

    window.dispatchEvent(new Event("phoenix:tenant-settings-stale"));

    expect(mockMutate).toHaveBeenCalledWith("test-tenant:settings-schema");
  });

  it("unsubscribes the broadcast handler on unmount", () => {
    const { unmount } = renderHook(() => useSettingsCacheBridge());

    unmount();

    expect(mockUnsubscribe).toHaveBeenCalled();
  });

  it("removes the window event listener on unmount", () => {
    const { unmount } = renderHook(() => useSettingsCacheBridge());

    unmount();

    // After unmount, dispatching the window event must NOT trigger mutate.
    mockMutate.mockClear();
    window.dispatchEvent(new Event("phoenix:tenant-settings-stale"));

    expect(mockMutate).not.toHaveBeenCalled();
  });
});
