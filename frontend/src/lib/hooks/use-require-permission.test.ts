import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

const { mockUseSession, mockIsAdmin, mockHasPermission, mockReplace } =
  vi.hoisted(() => ({
    mockUseSession: vi.fn(),
    mockIsAdmin: vi.fn(),
    mockHasPermission: vi.fn(),
    mockReplace: vi.fn(),
  }));

vi.mock("next-auth/react", () => ({
  useSession: (): ReturnType<typeof mockUseSession> => mockUseSession(),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: (...args: unknown[]): boolean => mockIsAdmin(...args) as boolean,
  hasPermission: (...args: unknown[]): boolean =>
    mockHasPermission(...args) as boolean,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: mockReplace }),
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
    expect(mockReplace).not.toHaveBeenCalled();
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
    expect(mockReplace).not.toHaveBeenCalled();
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
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("redirects an authenticated user who lacks the permission to /dashboard", () => {
    mockUseSession.mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    });

    const { result } = renderHook(() => useRequirePermission("users:update"));

    expect(result.current.isReady).toBe(false);
    expect(mockReplace).toHaveBeenCalledWith("/dashboard");
  });

  it("does not redirect while unauthenticated (NextAuth's required:true owns that)", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "unauthenticated",
    });

    const { result } = renderHook(() => useRequirePermission("users:update"));

    expect(result.current.isReady).toBe(false);
    expect(mockReplace).not.toHaveBeenCalled();
  });
});
