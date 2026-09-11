import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";

import type { UseUnreadCountOptions } from "./use-unread-count";

const {
  mockUseSession,
  mockUseShellAuth,
  mockHasPermission,
  mockUseUnreadCount,
  mockUseTenantSlugSafe,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseShellAuth: vi.fn(),
  mockHasPermission: vi.fn(),
  mockUseUnreadCount: vi.fn(),
  mockUseTenantSlugSafe: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: (): ReturnType<typeof mockUseSession> => mockUseSession(),
}));

vi.mock("~/lib/auth-utils", () => ({
  hasPermission: (...args: unknown[]): boolean =>
    mockHasPermission(...args) as boolean,
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: (): ReturnType<typeof mockUseShellAuth> => mockUseShellAuth(),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: (): ReturnType<typeof mockUseTenantSlugSafe> =>
    mockUseTenantSlugSafe(),
}));

vi.mock("./use-unread-count", () => ({
  useUnreadCount: (opts: UseUnreadCountOptions): unknown =>
    mockUseUnreadCount(opts) as unknown,
}));

import { useStaffNoticesPending } from "./use-staff-notices-pending";

function capturedOptions(): UseUnreadCountOptions {
  renderHook(() => useStaffNoticesPending());
  expect(mockUseUnreadCount).toHaveBeenCalledTimes(1);
  return mockUseUnreadCount.mock.calls[0]![0] as UseUnreadCountOptions;
}

beforeEach(() => {
  vi.clearAllMocks();
  mockUseUnreadCount.mockReturnValue({ unreadCount: 0, isLoading: false });
  mockUseShellAuth.mockReturnValue({ mode: "teacher" });
  mockUseTenantSlugSafe.mockReturnValue("testschool");
  mockUseSession.mockReturnValue({
    data: { user: { id: "7" } },
    status: "authenticated",
  });
  mockHasPermission.mockReturnValue(true);
});

describe("useStaffNoticesPending", () => {
  it("enables the badge for teacher-mode users with users:read", () => {
    const opts = capturedOptions();

    expect(opts.enabled).toBe(true);
    expect(mockHasPermission).toHaveBeenCalledWith(
      { user: { id: "7" } },
      "users:read",
    );
  });

  it("does not request notices without users:read", () => {
    mockHasPermission.mockReturnValue(false);

    expect(capturedOptions().enabled).toBe(false);
  });

  it("disables the badge outside teacher mode", () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });

    expect(capturedOptions().enabled).toBe(false);
  });
});
