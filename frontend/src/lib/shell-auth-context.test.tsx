import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";

// Use vi.hoisted for mock values referenced in vi.mock
const { mockUseSession, mockSignOut } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockSignOut: vi.fn(),
}));

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
    expect(result.current.homeUrl).toBe("/dashboard");
    expect(result.current.settingsUrl).toBe("/settings");
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

  it("calls signOut with callbackUrl on logout", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    await result.current.logout();

    expect(mockSignOut).toHaveBeenCalledWith({ callbackUrl: "/" });
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
    expect(result.current.homeUrl).toBe("/operator/suggestions");
    expect(result.current.settingsUrl).toBe("/operator/settings");
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

  it("never reports session as expired", () => {
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
});

describe("useShellAuth hook", () => {
  it("throws error when used outside provider", () => {
    expect(() => {
      renderHook(() => useShellAuth());
    }).toThrow("useShellAuth must be used within a ShellAuthProvider");
  });
});
