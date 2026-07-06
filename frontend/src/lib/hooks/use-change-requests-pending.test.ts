import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import type { UseUnreadCountOptions } from "./use-unread-count";

const {
  mockUseSession,
  mockUseShellAuth,
  mockIsAdmin,
  mockHasPermission,
  mockUseUnreadCount,
  mockFetchPendingChangeRequestCount,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseShellAuth: vi.fn(),
  mockIsAdmin: vi.fn(),
  mockHasPermission: vi.fn(),
  mockUseUnreadCount: vi.fn(),
  mockFetchPendingChangeRequestCount: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: (): ReturnType<typeof mockUseSession> => mockUseSession(),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: (): ReturnType<typeof mockUseShellAuth> => mockUseShellAuth(),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: (...args: unknown[]): boolean => mockIsAdmin(...args) as boolean,
  hasPermission: (...args: unknown[]): boolean =>
    mockHasPermission(...args) as boolean,
}));

vi.mock("~/lib/change-requests-api", () => ({
  fetchPendingChangeRequestCount: mockFetchPendingChangeRequestCount,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: (): string => "testschool",
}));

vi.mock("./use-unread-count", () => ({
  useUnreadCount: (opts: UseUnreadCountOptions): unknown =>
    mockUseUnreadCount(opts) as unknown,
}));

import { useChangeRequestsPending } from "./use-change-requests-pending";

/** Renders the hook and returns the options it passed to useUnreadCount. */
function capturedOptions(): UseUnreadCountOptions {
  renderHook(() => useChangeRequestsPending());
  expect(mockUseUnreadCount).toHaveBeenCalledTimes(1);
  return mockUseUnreadCount.mock.calls[0]![0] as UseUnreadCountOptions;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockUseUnreadCount.mockReturnValue({ unreadCount: 0, isLoading: false });
  mockIsAdmin.mockReturnValue(false);
  mockHasPermission.mockReturnValue(false);
  mockUseShellAuth.mockReturnValue({ mode: "teacher" });
  mockUseSession.mockReturnValue({
    data: { user: { id: "7" } },
    status: "authenticated",
  });
});

describe("useChangeRequestsPending", () => {
  it("enables the badge for an admin in the teacher shell", () => {
    mockIsAdmin.mockReturnValue(true);
    expect(capturedOptions().enabled).toBe(true);
  });

  it("enables the badge for a non-admin holding users:update", () => {
    mockHasPermission.mockReturnValue(true);
    const opts = capturedOptions();
    expect(opts.enabled).toBe(true);
    expect(mockHasPermission).toHaveBeenCalledWith(
      { user: { id: "7" } },
      "users:update",
    );
  });

  it("disables the badge for a staffer lacking both admin and users:update", () => {
    expect(capturedOptions().enabled).toBe(false);
  });

  it("disables the badge outside the teacher shell even for an admin", () => {
    mockIsAdmin.mockReturnValue(true);
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    expect(capturedOptions().enabled).toBe(false);
  });

  it("disables the badge until the session is authenticated", () => {
    mockIsAdmin.mockReturnValue(true);
    mockUseSession.mockReturnValue({ data: null, status: "loading" });
    expect(capturedOptions().enabled).toBe(false);
  });

  it("wires the pending-count fetcher and the queue's own refresh event", () => {
    mockIsAdmin.mockReturnValue(true);
    const opts = capturedOptions();
    expect(opts.fetcher).toBe(mockFetchPendingChangeRequestCount);
    expect(opts.eventNames).toEqual([
      "messages-unread-refresh",
      "change-requests-refresh",
    ]);
    expect(opts.refetchOnFocus).toBe(true);
  });

  it("scopes the cache key by tenant slug and account id (no cross-tenant leak)", () => {
    mockIsAdmin.mockReturnValue(true);
    expect(capturedOptions().cacheKey).toBe(
      "change_requests_pending_count:testschool:7",
    );
  });
});
