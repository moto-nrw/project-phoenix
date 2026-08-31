import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";

// Use vi.hoisted for mock values referenced in vi.mock
const { mockUseSession, mockSignOut, mockClearSessionCache } = vi.hoisted(
  () => ({
    mockUseSession: vi.fn(),
    mockSignOut: vi.fn(),
    mockClearSessionCache: vi.fn(),
  }),
);

const mockProfile = {
  firstName: "John",
  lastName: "Doe",
  avatar: "/avatar.jpg",
};

interface MockSessionReturn {
  data: {
    user?: {
      name?: string;
      email?: string;
      roles?: string[];
    };
    error?: string;
  } | null;
  status: "authenticated" | "loading" | "unauthenticated";
}

vi.mock("next-auth/react", () => ({
  useSession: (): MockSessionReturn => mockUseSession() as MockSessionReturn,
  signOut: mockSignOut,
}));

vi.mock("~/lib/profile-context", () => ({
  useProfile: () => ({ profile: mockProfile }),
}));

vi.mock("~/lib/operator-url", () => ({
  operatorAbsoluteUrl: (path: string) => path,
  operatorPath: (path: string) => path,
}));

vi.mock("~/lib/session-cache", () => ({
  clearSessionCache: mockClearSessionCache,
}));

import {
  TeacherShellProvider,
  OperatorShellProvider,
  useShellAuth,
} from "./shell-auth-context";

describe("TeacherShellProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSignOut.mockResolvedValue(undefined);
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <TeacherShellProvider>{children}</TeacherShellProvider>
  );

  it("provides authenticated teacher data", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "John Doe",
          email: "john@example.com",
          roles: ["teacher", "admin"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.user).toEqual({
      name: "John Doe",
      email: "john@example.com",
      roles: ["teacher", "admin"],
    });
    expect(result.current.status).toBe("authenticated");
    expect(result.current.isSessionExpired).toBe(false);
    expect(result.current.mode).toBe("teacher");
    // Betreuungskräfte (auch mit Doppelrolle) haben den Tagesplan als Home
    // (#2383) — dieselbe Priorität wie der Login-Redirect.
    expect(result.current.homeUrl).toBe("/betreuungsplan/tag");
    expect(result.current.profileUrl).toBe("/profile");
  });

  it("keeps /dashboard as home for admin-only accounts (#2383)", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Admin Only",
          email: "admin@example.com",
          roles: ["admin"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.homeUrl).toBe("/dashboard");
  });

  it("provides profile data from context", () => {
    mockUseSession.mockReturnValue({
      data: { user: { name: "Test", email: "test@example.com", roles: [] } },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.profile).toEqual({
      firstName: "John",
      lastName: "Doe",
      avatar: "/avatar.jpg",
    });
  });

  it("handles loading state", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.status).toBe("loading");
    expect(result.current.user).toBeNull();
  });

  it("handles unauthenticated state", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "unauthenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.status).toBe("unauthenticated");
    expect(result.current.user).toBeNull();
  });

  it("uses fallback name when user name is empty", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "   ",
          email: "user@example.com",
          roles: [],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.user?.name).toBe("Benutzer");
  });

  it("uses fallback name when user name is undefined", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: undefined,
          email: "user@example.com",
          roles: [],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.user?.name).toBe("Benutzer");
  });

  it("detects expired session", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: { name: "User", email: "user@example.com", roles: [] },
        error: "RefreshTokenExpired",
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.isSessionExpired).toBe(true);
  });

  it("calls signOut with redirect:false and navigates manually on logout", async () => {
    const mockFetch = vi.fn().mockResolvedValue(new Response(null));
    vi.stubGlobal("fetch", mockFetch);
    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });

    try {
      const { result } = renderHook(() => useShellAuth(), { wrapper });

      await result.current.logout();

      expect(mockFetch).toHaveBeenCalledWith("/api/auth/logout", {
        method: "POST",
        signal: expect.any(AbortSignal) as AbortSignal,
      });
      expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("fires backend logout and clears session cache on teacher logout", async () => {
    const mockFetch = vi.fn().mockResolvedValue(new Response(null));
    vi.stubGlobal("fetch", mockFetch);

    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    await result.current.logout();

    expect(mockFetch).toHaveBeenCalledWith("/api/auth/logout", {
      method: "POST",
      signal: expect.any(AbortSignal) as AbortSignal,
    });
    expect(mockClearSessionCache).toHaveBeenCalled();

    vi.unstubAllGlobals();
  });

  it("handles backend logout failure gracefully", async () => {
    const mockFetch = vi.fn().mockRejectedValue(new Error("network error"));
    vi.stubGlobal("fetch", mockFetch);

    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    // Should not throw even when fetch fails
    await result.current.logout();

    expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    expect(mockClearSessionCache).toHaveBeenCalled();

    vi.unstubAllGlobals();
  });

  it("clears the Auth.js cookie only after the response-aware backend logout finishes", async () => {
    let finishBackendLogout: ((response: Response) => void) | undefined;
    const backendLogout = new Promise<Response>((resolve) => {
      finishBackendLogout = resolve;
    });
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(backendLogout));
    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });
    const logout = result.current.logout();

    await Promise.resolve();
    expect(mockSignOut).not.toHaveBeenCalled();

    finishBackendLogout?.(new Response(null));
    await logout;

    expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    vi.unstubAllGlobals();
  });

  it("provides empty roles array when not specified", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "User",
          email: "user@example.com",
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.user?.roles).toEqual([]);
  });

  it("provides empty email when not specified", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "User",
          roles: [],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.user?.email).toBe("");
  });
});

describe("OperatorShellProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSignOut.mockResolvedValue(undefined);
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <OperatorShellProvider>{children}</OperatorShellProvider>
  );

  it("provides authenticated operator data", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Admin Operator",
          email: "admin@example.com",
          roles: ["operator"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.user).toEqual({
      name: "Admin Operator",
      email: "admin@example.com",
      roles: ["operator"],
    });
    expect(result.current.status).toBe("authenticated");
    expect(result.current.mode).toBe("operator");
    expect(result.current.homeUrl).toBe("/operator/organizations");
    expect(result.current.profileUrl).toBe("/operator/settings");
  });

  it("splits display name into first and last name", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "John Paul Jones",
          email: "john@example.com",
          roles: ["operator"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.profile).toEqual({
      firstName: "John",
      lastName: "Paul Jones",
    });
  });

  it("handles single word display name", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Admin",
          email: "admin@example.com",
          roles: ["operator"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.profile).toEqual({
      firstName: "Admin",
      lastName: undefined,
    });
  });

  it("handles loading state", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.status).toBe("loading");
    expect(result.current.user).toBeNull();
  });

  it("handles unauthenticated state", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "unauthenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.status).toBe("unauthenticated");
    expect(result.current.user).toBeNull();
  });

  it("reports session as not expired when no error", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Operator",
          email: "op@example.com",
          roles: ["operator"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.isSessionExpired).toBe(false);
  });

  it("detects expired session", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Operator",
          email: "op@example.com",
          roles: ["operator"],
        },
        error: "RefreshTokenExpired",
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.isSessionExpired).toBe(true);
  });

  it("uses fallback name when user name is empty", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "   ",
          email: "op@example.com",
          roles: ["operator"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.user?.name).toBe("Operator");
  });

  it("provides default roles when not specified", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Op",
          email: "op@example.com",
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.user?.roles).toEqual(["operator"]);
  });

  it("calls signOut with operator login callbackUrl", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Operator",
          email: "op@example.com",
          roles: ["operator"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    await result.current.logout();

    expect(mockSignOut).toHaveBeenCalledWith({
      callbackUrl: "/operator/login",
    });
  });

  it("clears session cache on operator logout", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Operator",
          email: "op@example.com",
          roles: ["operator"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    await result.current.logout();

    expect(mockClearSessionCache).toHaveBeenCalled();
  });
});

describe("useShellAuth hook", () => {
  it("throws error when used outside provider", () => {
    expect(() => {
      renderHook(() => useShellAuth());
    }).toThrow("useShellAuth must be used within a ShellAuthProvider");
  });
});
