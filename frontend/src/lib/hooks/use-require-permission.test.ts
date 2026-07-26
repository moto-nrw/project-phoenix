import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const { mockUseSession, mockIsAdmin, mockHasPermission, mockRedirect } =
  vi.hoisted(() => ({
    mockUseSession: vi.fn(),
    mockIsAdmin: vi.fn(),
    mockHasPermission: vi.fn(),
    mockRedirect: vi.fn(),
  }));

vi.mock("next-auth/react", () => ({
  useSession: (): ReturnType<typeof mockUseSession> => mockUseSession(),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: (...args: unknown[]): boolean => mockIsAdmin(...args) as boolean,
  hasPermission: (...args: unknown[]): boolean =>
    mockHasPermission(...args) as boolean,
}));

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

import { useRequirePermission } from "./use-require-permission";

beforeEach(() => {
  vi.clearAllMocks();
  mockIsAdmin.mockReturnValue(false);
  mockHasPermission.mockReturnValue(false);
});

describe("useRequirePermission", () => {
  it("is not ready and does not redirect while the session is loading", () => {
    mockUseSession.mockReturnValue({ data: null, status: "loading" });

    const { result } = renderHook(() => useRequirePermission("users:update"));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.isReady).toBe(false);
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("is ready and does not redirect when the user holds the permission", () => {
    mockUseSession.mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    });
    mockHasPermission.mockReturnValue(true);

    const { result } = renderHook(() => useRequirePermission("users:update"));

    expect(result.current.isReady).toBe(true);
    expect(result.current.isLoading).toBe(false);
    expect(mockRedirect).not.toHaveBeenCalled();
    expect(mockHasPermission).toHaveBeenCalledWith(
      { user: { id: "1" } },
      "users:update",
    );
  });

  it("lets an admin through without holding the explicit permission", () => {
    mockUseSession.mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    });
    mockIsAdmin.mockReturnValue(true);
    mockHasPermission.mockReturnValue(false);

    const { result } = renderHook(() => useRequirePermission("users:update"));

    expect(result.current.isReady).toBe(true);
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("redirects an authenticated user who lacks the permission to /dashboard", () => {
    mockUseSession.mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    });

    const { result } = renderHook(() => useRequirePermission("users:update"));

    expect(result.current.isReady).toBe(false);
    expect(mockRedirect).toHaveBeenCalledWith("/dashboard");
  });

  it("is ready when the user holds any permission of an array", () => {
    mockUseSession.mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    });
    mockHasPermission.mockImplementation(
      (_session: unknown, permission: unknown) =>
        permission === "display:manage",
    );

    const { result } = renderHook(() =>
      useRequirePermission(["display:read", "display:manage"]),
    );

    expect(result.current.isReady).toBe(true);
    expect(mockRedirect).not.toHaveBeenCalled();
  });

  it("redirects when the user holds none of the array permissions", () => {
    mockUseSession.mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    });

    const { result } = renderHook(() =>
      useRequirePermission(["display:read", "display:manage"]),
    );

    expect(result.current.isReady).toBe(false);
    expect(mockRedirect).toHaveBeenCalledWith("/dashboard");
  });

  it("does not redirect while unauthenticated (NextAuth's required:true owns that)", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "unauthenticated",
    });

    const { result } = renderHook(() => useRequirePermission("users:update"));

    expect(result.current.isReady).toBe(false);
    expect(mockRedirect).not.toHaveBeenCalled();
  });
});
