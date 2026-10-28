import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";

// Use vi.hoisted for mock values referenced in vi.mock
const {
  mockShowLogoutError,
  mockUseSession,
  mockSignOut,
  mockClearSessionCache,
  mockEndStaffPreview,
  mockSchoolPortalLoginUrl,
} = vi.hoisted(() => ({
  mockShowLogoutError: vi.fn(),
  mockUseSession: vi.fn(),
  mockSignOut: vi.fn(),
  mockClearSessionCache: vi.fn(),
  mockEndStaffPreview: vi.fn(),
  mockSchoolPortalLoginUrl: vi.fn(),
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

vi.mock("~/contexts/ToastContext", () => ({
  useApiErrorDisplay: () => ({ show: mockShowLogoutError }),
}));

vi.mock("~/lib/profile-context", () => ({
  useProfile: () => ({ profile: mockProfile }),
}));

vi.mock("~/lib/operator-url", () => ({
  operatorAbsoluteUrl: (path: string) => path,
  operatorPath: (path: string) => path,
}));

vi.mock("~/lib/school-url", () => ({
  schoolAbsoluteUrl: (path: string) => path,
  schoolPath: (path: string) => path,
  schoolPortalLoginUrl: mockSchoolPortalLoginUrl,
}));

vi.mock("~/lib/session-cache", () => ({
  clearSessionCache: mockClearSessionCache,
}));

vi.mock("~/lib/staff-preview-api", () => ({
  performEndStaffPreview: mockEndStaffPreview,
}));

vi.mock("~/lib/swr", () => ({
  mutate: vi.fn(),
}));

vi.mock("~/lib/parent-url", () => ({
  parentAbsoluteUrl: (path: string) => path,
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/caterer-url", () => ({
  catererAbsoluteUrl: (path: string) => path,
  catererPath: (path: string) => path,
}));

import {
  TeacherShellProvider,
  OperatorShellProvider,
  ParentShellProvider,
  SchoolShellProvider,
  CatererShellProvider,
  useShellAuth,
} from "./shell-auth-context";
import { endSessionOnce, sessionEndFailureClaimed } from "./session-end-flight";
import { sidebarMemoryKey } from "./sidebar-memory";
import { holdRequestsUntilSessionEnds } from "~/lib/session-end-barrier";

describe("TeacherShellProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSignOut.mockResolvedValue(undefined);
    mockSchoolPortalLoginUrl.mockReturnValue(
      "https://schule.example.test/login",
    );
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
          permissions: ["schedules:read"],
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
    // Das Logo führt für jede Rolle auf die Startseite (#2180).
    expect(result.current.homeUrl).toBe("/home");
    expect(result.current.profileUrl).toBe("/profile");
  });

  it("keeps the start page as home for admin-only accounts (#2180)", () => {
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

    expect(result.current.homeUrl).toBe("/home");
  });

  it("keeps the start page as home for caregivers without schedules:read (#2180)", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Ohne Recht",
          email: "user@example.com",
          roles: ["user"],
          permissions: [],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.homeUrl).toBe("/home");
  });

  it("hands existing school-only sessions to the school portal", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Lehrkraft",
          email: "lehrkraft@example.com",
          roles: ["lehrkraft"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.homeUrl).toBe("/school/login");
    expect(mockSchoolPortalLoginUrl).not.toHaveBeenCalled();
  });

  it("keeps the school-only hand-off safe during server rendering", () => {
    mockSchoolPortalLoginUrl.mockImplementation(() => {
      throw new Error("schoolPortalLoginUrl() is client-only.");
    });
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Lehrkraft",
          email: "lehrkraft@example.com",
          roles: ["lehrkraft"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.homeUrl).toBe("/school/login");
    expect(mockSchoolPortalLoginUrl).not.toHaveBeenCalled();
  });

  it("keeps dual-role lehrkraft accounts in the staff portal", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Lehrkraft mit Betreuung",
          email: "lehrkraft@example.com",
          roles: ["lehrkraft", "user"],
        },
      },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });

    expect(result.current.homeUrl).toBe("/home");
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

  // Abmelden aus einer laufenden Vorschau (#2893): die Vorschau endet damit
  // genauso wie per Klick — und muss deshalb auch im Protokoll enden, bevor
  // der Logout die Admin-Familie widerruft.
  it("ends a running staff preview before logging out", async () => {
    const mockFetch = vi.fn().mockResolvedValue(new Response(null));
    vi.stubGlobal("fetch", mockFetch);
    mockEndStaffPreview.mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Erika Beispiel",
          email: "erika@example.com",
          roles: [],
          isPreview: true,
        },
      },
      status: "authenticated",
    });

    try {
      const { result } = renderHook(() => useShellAuth(), { wrapper });

      await result.current.logout();

      // Das Vorschau-Token liegt nur im Sitzungs-Cookie (#5005): das Ende
      // muss im Protokoll stehen, bevor der Logout die Sitzung löscht.
      expect(mockEndStaffPreview).toHaveBeenCalledOnce();
      expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
      expect(mockEndStaffPreview.mock.invocationCallOrder[0]).toBeLessThan(
        mockSignOut.mock.invocationCallOrder[0]!,
      );
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("does not end a preview on a regular logout", async () => {
    const mockFetch = vi.fn().mockResolvedValue(new Response(null));
    vi.stubGlobal("fetch", mockFetch);
    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });

    try {
      const { result } = renderHook(() => useShellAuth(), { wrapper });
      await result.current.logout();
      expect(mockEndStaffPreview).not.toHaveBeenCalled();
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("logs out even when ending the preview fails", async () => {
    const mockFetch = vi.fn().mockResolvedValue(new Response(null));
    vi.stubGlobal("fetch", mockFetch);
    mockEndStaffPreview.mockRejectedValue(new Error("audit endpoint down"));
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Erika Beispiel",
          email: "erika@example.com",
          roles: [],
          isPreview: true,
        },
      },
      status: "authenticated",
    });

    try {
      const { result } = renderHook(() => useShellAuth(), { wrapper });
      await result.current.logout();
      expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    } finally {
      vi.unstubAllGlobals();
    }
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

  it("clears the Auth.js cookie only after the tab's open requests are answered (#5126)", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null)));
    let settled: (() => void) | undefined;
    vi.mocked(holdRequestsUntilSessionEnds).mockReturnValueOnce(
      new Promise((resolve) => {
        settled = () => resolve(() => undefined);
      }),
    );
    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });

    const { result } = renderHook(() => useShellAuth(), { wrapper });
    const logout = result.current.logout();

    await vi.waitFor(() =>
      expect(holdRequestsUntilSessionEnds).toHaveBeenCalled(),
    );
    expect(mockSignOut).not.toHaveBeenCalled();

    settled?.();
    await logout;

    expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    vi.unstubAllGlobals();
  });

  it("reports a failed sign-out and releases the barrier for a retry (#5126)", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null)));
    const cancelSessionEnd = vi.fn();
    vi.mocked(holdRequestsUntilSessionEnds).mockResolvedValueOnce(
      cancelSessionEnd,
    );
    const failure = new Error("Sign out failed");
    mockSignOut.mockRejectedValueOnce(failure);
    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });

    try {
      const { result } = renderHook(() => useShellAuth(), { wrapper });
      await expect(result.current.logout()).rejects.toBe(failure);

      expect(cancelSessionEnd).toHaveBeenCalledOnce();
      expect(mockShowLogoutError).toHaveBeenCalledWith(failure, {
        object: "die Abmeldung",
        retry: expect.any(Function),
      });
      // The logout modal closes for a failure the shell already reported.
      expect(sessionEndFailureClaimed(failure)).toBe(true);
      expect(sessionEndFailureClaimed(new Error("other"))).toBe(false);
      expect(sessionStorage.getItem("deliberateLogout")).toBeNull();

      // The toast's retry runs the whole logout again.
      const [, options] = mockShowLogoutError.mock.calls[0] as [
        unknown,
        { retry: () => void },
      ];
      mockSignOut.mockResolvedValueOnce(undefined);
      options.retry();
      await vi.waitFor(() => expect(mockSignOut).toHaveBeenCalledTimes(2));
    } finally {
      vi.unstubAllGlobals();
    }
  });

  // One logout per tab, one message per failure (#5126).
  it("starts one logout for a double click and shows its failure once", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null));
    vi.stubGlobal("fetch", fetchMock);
    let failSignOut!: (error: Error) => void;
    mockSignOut.mockReturnValueOnce(
      new Promise((_resolve, reject) => (failSignOut = reject)),
    );
    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });

    try {
      const { result } = renderHook(() => useShellAuth(), { wrapper });
      const first = result.current.logout();
      const second = result.current.logout();
      await vi.waitFor(() => expect(mockSignOut).toHaveBeenCalledOnce());
      // A retry while it still runs joins it too.
      const retry = result.current.logout();

      const failure = new Error("sign-out failed");
      failSignOut(failure);
      await expect(first).rejects.toBe(failure);
      await expect(second).rejects.toBe(failure);
      await expect(retry).rejects.toBe(failure);

      expect(mockSignOut).toHaveBeenCalledOnce();
      expect(
        fetchMock.mock.calls.filter(([url]) => url === "/api/auth/logout"),
      ).toHaveLength(1);
      expect(holdRequestsUntilSessionEnds).toHaveBeenCalledOnce();
      expect(mockShowLogoutError).toHaveBeenCalledOnce();
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("joins a running school switch instead of starting a second logout", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null));
    vi.stubGlobal("fetch", fetchMock);
    mockUseSession.mockReturnValue({
      data: { user: { name: "User", email: "user@example.com", roles: [] } },
      status: "authenticated",
    });
    let failSwitch!: (error: Error) => void;
    const schoolSwitch = endSessionOnce(
      () => new Promise<void>((_resolve, reject) => (failSwitch = reject)),
    );

    try {
      const { result } = renderHook(() => useShellAuth(), { wrapper });
      const menu = result.current.logout();
      const failure = new Error("switch logout failed");
      failSwitch(failure);
      await expect(schoolSwitch).rejects.toBe(failure);
      await expect(menu).rejects.toBe(failure);

      expect(fetchMock).not.toHaveBeenCalled();
      expect(mockSignOut).not.toHaveBeenCalled();
      // The switch had no caller claiming it here, so the menu shows it once.
      expect(mockShowLogoutError).toHaveBeenCalledOnce();
    } finally {
      vi.unstubAllGlobals();
    }
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

describe("ParentShellProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // A parent token carries no names; its session name is the username,
    // which is the account's address (#3891).
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "sabine.schneider@demo-7.example",
          email: "sabine.schneider@demo-7.example",
          roles: ["guardian"],
        },
      },
      status: "authenticated",
    });
  });

  it("names the parent after the guardian profile, not the session", () => {
    const { result } = renderHook(() => useShellAuth(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <ParentShellProvider
          accountName={{ firstName: "Florian", lastName: "Besuch" }}
        >
          {children}
        </ParentShellProvider>
      ),
    });

    expect(result.current.user?.name).toBe("Florian Besuch");
    expect(result.current.profile).toEqual({
      firstName: "Florian",
      lastName: "Besuch",
    });
  });

  it("never shows the address as the name while the profile is missing", () => {
    const { result } = renderHook(() => useShellAuth(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <ParentShellProvider>{children}</ParentShellProvider>
      ),
    });

    expect(result.current.user?.name).toBe("Eltern");
    expect(result.current.profile).toBeNull();
  });

  it("keeps a real session name when the profile has none", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          name: "Karin Klein",
          email: "karin@example.com",
          roles: ["guardian"],
        },
      },
      status: "authenticated",
    });
    const { result } = renderHook(() => useShellAuth(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <ParentShellProvider accountName={{}}>{children}</ParentShellProvider>
      ),
    });

    expect(result.current.user?.name).toBe("Karin Klein");
  });
});

describe("useShellAuth hook", () => {
  it("throws error when used outside provider", () => {
    expect(() => {
      renderHook(() => useShellAuth());
    }).toThrow("useShellAuth must be used within a ShellAuthProvider");
  });
});

describe("logout leaves no personal entries behind (#5000)", () => {
  const LAST_ROOM_NAME = sidebarMemoryKey("sidebar-last-room-name", "1:7");
  const PERSONAL_LOCAL = [
    LAST_ROOM_NAME,
    "messages_unread_count:schule:7",
    "student-search:last-filters:tenant-1:7",
    "moto:personal-storage-owner",
  ];
  const PERSONAL_SESSION = [
    "timetable-reopenable-instance",
    "parent-first-steps-opened:7",
  ];
  let leftAtSignOut: string[] = [];

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    sessionStorage.clear();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null)));
    // The check runs when the cookie goes: nothing may outlive the session.
    mockSignOut.mockImplementation(() => {
      leftAtSignOut = [
        ...PERSONAL_LOCAL.filter((key) => localStorage.getItem(key) !== null),
        ...PERSONAL_SESSION.filter(
          (key) => sessionStorage.getItem(key) !== null,
        ),
      ];
      return Promise.resolve(undefined);
    });
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "7",
          tenantId: 1,
          name: "Mia Lehrerin",
          email: "mia@example.com",
          roles: [],
        },
      },
      status: "authenticated",
    });
    for (const key of PERSONAL_LOCAL) localStorage.setItem(key, "x");
    for (const key of PERSONAL_SESSION) sessionStorage.setItem(key, "x");
    localStorage.setItem("sidebar-collapsed", "true");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it.each([
    ["OGS", TeacherShellProvider],
    ["Betreiber", OperatorShellProvider],
    ["Schule", SchoolShellProvider],
    ["Caterer", CatererShellProvider],
    ["Eltern", ParentShellProvider],
  ] as const)("in the %s portal", async (_portal, Provider) => {
    leftAtSignOut = ["signOut was not called"];
    const { result } = renderHook(() => useShellAuth(), {
      wrapper: ({ children }: { children: ReactNode }) => (
        <Provider>{children}</Provider>
      ),
    });

    await result.current.logout();

    expect(leftAtSignOut).toEqual([]);
    // Device settings without data of a person stay.
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");
  });
});
