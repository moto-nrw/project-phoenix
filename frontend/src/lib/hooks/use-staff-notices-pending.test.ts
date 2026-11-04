import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";

import type { StaffNotice } from "~/lib/staff-notices-api";

const { mockUseSession, mockUseShellAuth, mockHasPermission, mockUseSWRAuth } =
  vi.hoisted(() => ({
    mockUseSession: vi.fn(),
    mockUseShellAuth: vi.fn(),
    mockHasPermission: vi.fn(),
    mockUseSWRAuth: vi.fn(),
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

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (...args: unknown[]): unknown =>
    mockUseSWRAuth(...args) as unknown,
}));

import { useStaffNoticesPending } from "./use-staff-notices-pending";

function notice(overrides: Partial<StaffNotice>): StaffNotice {
  return {
    id: "1",
    title: "Hinweis",
    body: "",
    priority: "info",
    audience: "all",
    valid_from: "2026-09-09",
    weekdays: [],
    week_pattern: 0,
    requires_acknowledgement: false,
    active: true,
    ...overrides,
  };
}

/** The key the badge asks SWR for; null when it asks nothing. */
function requestedKey(): unknown {
  renderHook(() => useStaffNoticesPending());
  expect(mockUseSWRAuth).toHaveBeenCalled();
  return mockUseSWRAuth.mock.calls.at(-1)?.[0];
}

beforeEach(() => {
  vi.clearAllMocks();
  mockUseSWRAuth.mockReturnValue({
    data: undefined,
    isLoading: false,
    mutate: vi.fn(),
  });
  mockUseShellAuth.mockReturnValue({ mode: "teacher" });
  mockUseSession.mockReturnValue({
    data: { user: { id: "7" } },
    status: "authenticated",
  });
  mockHasPermission.mockReturnValue(true);
});

describe("useStaffNoticesPending", () => {
  it("reads the shared entry of today's notices for teacher-mode users with users:read", () => {
    expect(requestedKey()).toBe("staff-notices-today:7");
    expect(mockHasPermission).toHaveBeenCalledWith(
      { user: { id: "7" } },
      "users:read",
    );
    // A page change shows the entry the shell seeded; focus reloads it.
    expect(mockUseSWRAuth.mock.calls.at(-1)?.[2]).toMatchObject({
      revalidateIfStale: false,
      revalidateOnFocus: true,
    });
  });

  it("does not request notices without users:read", () => {
    mockHasPermission.mockReturnValue(false);

    expect(requestedKey()).toBeNull();
  });

  it("disables the badge outside teacher mode", () => {
    mockUseShellAuth.mockReturnValue({ mode: "operator" });

    expect(requestedKey()).toBeNull();
  });

  it("counts only notices whose acknowledgement is required and missing", () => {
    mockUseSWRAuth.mockReturnValue({
      data: [
        notice({ id: "1", requires_acknowledgement: true }),
        notice({
          id: "2",
          requires_acknowledgement: true,
          acknowledged_at: "2026-09-09T08:00:00Z",
        }),
        notice({ id: "3", requires_acknowledgement: false }),
      ],
      isLoading: false,
      mutate: vi.fn(),
    });

    const { result } = renderHook(() => useStaffNoticesPending());

    expect(result.current.unreadCount).toBe(1);
  });
});
