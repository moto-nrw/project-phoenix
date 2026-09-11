/**
 * Tests for useSettingsCacheBridge: invalidates the SETTINGS_SCHEMA_SWR_KEY
 * SWR cache when either signal fires:
 *   1. same-origin BroadcastChannel ping (notifySettingsChanged)
 *   2. phoenix:tenant-settings-stale window event (cross-origin SSE)
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";

const mockMutate = vi.fn();
const mockUseSession = vi.fn();

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

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
    mockUseSession.mockReturnValue({
      data: { user: { id: "account-7" } },
      status: "authenticated",
    });
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

  it("revalidates only the active account's effective request access", () => {
    renderHook(() => useSettingsCacheBridge());

    subscribers[0]!();

    expect(mockMutate).toHaveBeenCalledWith(
      "test-tenant:change-request-access:account-7",
    );
    expect(mockMutate).not.toHaveBeenCalledWith(
      "test-tenant:change-request-access:account-8",
    );
    expect(
      mockMutate.mock.calls.some(([key]) => typeof key === "function"),
    ).toBe(false);
  });

  it("switches access invalidation to the current account", () => {
    const { rerender } = renderHook(() => useSettingsCacheBridge());

    mockUseSession.mockReturnValue({
      data: { user: { id: "account-8" } },
      status: "authenticated",
    });
    rerender();
    subscribers.at(-1)!();

    expect(mockMutate).toHaveBeenCalledWith(
      "test-tenant:change-request-access:account-8",
    );
    expect(mockMutate).not.toHaveBeenCalledWith(
      "test-tenant:change-request-access:account-7",
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
