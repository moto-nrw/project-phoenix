import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import type { UseUnreadCountOptions } from "./use-unread-count";

const {
  mockUseSession,
  mockUseShellAuth,
  mockIsAdmin,
  mockHasPermission,
  mockUseUnreadCount,
  mockListPending,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockUseShellAuth: vi.fn(),
  mockIsAdmin: vi.fn(),
  mockHasPermission: vi.fn(),
  mockUseUnreadCount: vi.fn(),
  mockListPending: vi.fn(),
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

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    listPending: (): ReturnType<typeof mockListPending> => mockListPending(),
  },
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: (): string => "testschool",
}));

vi.mock("./use-unread-count", () => ({
  useUnreadCount: (opts: UseUnreadCountOptions): unknown =>
    mockUseUnreadCount(opts) as unknown,
}));

import { useStaffAbsencesPending } from "./use-staff-absences-pending";

/** Renders the hook and returns the options it passed to useUnreadCount. */
function capturedOptions(): UseUnreadCountOptions {
  renderHook(() => useStaffAbsencesPending());
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

describe("useStaffAbsencesPending", () => {
  it("enables the badge for an admin in the teacher shell", () => {
    mockIsAdmin.mockReturnValue(true);
    expect(capturedOptions().enabled).toBe(true);
  });

  it("enables the badge for a non-admin holding vacation:approve", () => {
    mockHasPermission.mockReturnValue(true);
    const opts = capturedOptions();
    expect(opts.enabled).toBe(true);
    expect(mockHasPermission).toHaveBeenCalledWith(
      { user: { id: "7" } },
      "vacation:approve",
    );
  });

  it("disables the badge for a staffer lacking both admin and vacation:approve", () => {
    expect(capturedOptions().enabled).toBe(false);
  });

  it("disables the badge outside the teacher shell even for an admin", () => {
    mockIsAdmin.mockReturnValue(true);
    mockUseShellAuth.mockReturnValue({ mode: "operator" });
    expect(capturedOptions().enabled).toBe(false);
  });

  it("counts the pending rows via the fetcher and swallows errors as 0", async () => {
    mockIsAdmin.mockReturnValue(true);
    mockListPending.mockResolvedValueOnce([{ id: 1 }, { id: 2 }]);
    const opts = capturedOptions();
    await expect(opts.fetcher()).resolves.toBe(2);
    mockListPending.mockRejectedValueOnce(new Error("403"));
    await expect(opts.fetcher()).resolves.toBe(0);
  });

  it("wires the absence refresh event and focus refetch", () => {
    mockIsAdmin.mockReturnValue(true);
    const opts = capturedOptions();
    expect(opts.eventNames).toEqual(["staff-absences-refresh"]);
    expect(opts.refetchOnFocus).toBe(true);
  });

  it("scopes the cache key by tenant slug and account id", () => {
    mockIsAdmin.mockReturnValue(true);
    expect(capturedOptions().cacheKey).toBe(
      "staff_absences_pending_count:testschool:7",
    );
  });
});
