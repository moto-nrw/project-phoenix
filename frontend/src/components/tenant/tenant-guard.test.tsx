import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { useState } from "react";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { ApiError, reportSessionTenantMismatch } from "~/lib/api-error";
import { installSessionTenantMismatchObserver } from "~/lib/session-tenant-mismatch-observer";
import { catalogText } from "~/test/error-catalog-text";
import { runTenantSessionEnd } from "~/lib/tenant-session-end";

// ============================================================================
// Mocks
// ============================================================================

const { mockUseSession, mockSignIn, mockSignOut } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockSignIn: vi.fn(),
  mockSignOut: vi.fn(),
}));

const { mockUsePathname } = vi.hoisted(() => ({
  mockUsePathname: vi.fn<() => string | null>(),
}));

vi.mock("next/navigation", () => ({
  usePathname: mockUsePathname,
}));

const { mockUseTenant } = vi.hoisted(() => ({
  mockUseTenant: vi.fn(),
}));

const { mockPerformTenantSwitch } = vi.hoisted(() => ({
  mockPerformTenantSwitch: vi.fn(),
}));

const { mockMutate } = vi.hoisted(() => ({
  mockMutate: vi.fn(),
}));

const { mockClearSessionCache } = vi.hoisted(() => ({
  mockClearSessionCache: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
  signIn: mockSignIn,
  signOut: mockSignOut,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: mockUseTenant,
  useNFCEnabled: vi.fn(() => true),
}));

vi.mock("~/lib/tenant-api", () => ({
  performTenantSwitch: mockPerformTenantSwitch,
}));

// "Neu anmelden" ist eine Aktion ohne Formular: ein Fehler dabei kommt als
// Toast (#2517). Der Grund eines gescheiterten Wechsels läuft über den echten
// Ladefehler-Weg.
const showActionError = vi.hoisted(() => vi.fn());
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useApiErrorDisplay: () => ({ show: showActionError }),
}));

vi.mock("~/lib/swr", () => ({
  mutate: mockMutate,
}));

const mockCheckSessionAfterLogoutElsewhere = vi.hoisted(() => vi.fn());
vi.mock("~/lib/session-cache", () => ({
  clearSessionCache: mockClearSessionCache,
  checkSessionAfterLogoutElsewhere: mockCheckSessionAfterLogoutElsewhere,
}));

import { TenantGuard } from "./tenant-guard";

// ============================================================================
// Test Data
// ============================================================================

const tenantA = {
  tenantId: 1,
  slug: "school-a",
  name: "School A",
  subdomain: "school-a",
  organizationId: 10,
  organizationName: "Org Alpha",
  settings: {},
};

const tenantB = {
  tenantId: 2,
  slug: "school-b",
  name: "School B",
  subdomain: "school-b",
  organizationId: 10,
  organizationName: "Org Alpha",
  settings: {},
};

const tenantC = {
  tenantId: 3,
  slug: "school-c",
  name: "School C",
  subdomain: "school-c",
  organizationId: 10,
  organizationName: "Org Alpha",
  settings: {},
};

// ============================================================================
// Tests
// ============================================================================

describe("TenantGuard", () => {
  let originalLocation: Location;

  beforeEach(() => {
    vi.clearAllMocks();
    mockUsePathname.mockReturnValue("/");
    mockMutate.mockResolvedValue(undefined);
    mockSignIn.mockResolvedValue({ ok: true });
    mockSignOut.mockResolvedValue(undefined);
    sessionStorage.clear();

    // Mock window.location for redirect assertions
    originalLocation = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...originalLocation, href: "", pathname: "/", assign: vi.fn() },
    });
  });

  afterEach(() => {
    Object.defineProperty(window, "location", {
      writable: true,
      value: originalLocation,
    });
    sessionStorage.clear();
  });

  it("renders children when session tenant matches URL tenant", () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-a",
      tenant: tenantA,
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    expect(screen.getByText("Protected Content")).toBeInTheDocument();
  });

  it("checks the session once another tab logged this account out", () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({ tenantSlug: "school-a", tenant: tenantA });
    mockCheckSessionAfterLogoutElsewhere.mockReset();
    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );
    const ownerChange = (newValue: string | null) =>
      window.dispatchEvent(
        new StorageEvent("storage", {
          key: "moto:personal-storage-owner",
          newValue,
          storageArea: localStorage,
        }),
      );

    // A new owner (another account, or this account bound to another
    // school) is no logout of this account; only a removed owner is.
    ownerChange("1:7");
    expect(mockCheckSessionAfterLogoutElsewhere).not.toHaveBeenCalled();
    ownerChange(null);
    expect(mockCheckSessionAfterLogoutElsewhere).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Protected Content")).toBeInTheDocument();
  });

  it("renders children transparently when session is loading", () => {
    mockUseSession.mockReturnValue({
      data: null,
      status: "loading",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-a",
      tenant: tenantA,
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    // Children render during loading to avoid layout flicker;
    // individual pages handle their own loading states
    expect(screen.getByText("Protected Content")).toBeInTheDocument();
  });

  it("shows switching state when mismatch detected", () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    // Prevent the switch from resolving immediately
    mockPerformTenantSwitch.mockReturnValue(new Promise(vi.fn()));

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();
    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
  });

  it("calls performTenantSwitch + update on mismatch", async () => {
    const mockUpdate = vi.fn().mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: mockUpdate,
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    mockPerformTenantSwitch.mockResolvedValue({
      access_token: "new-access",
      refresh_token: "new-refresh",
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
        "school-b",
        mockMutate,
      );
    });

    expect(mockUpdate).toHaveBeenCalled();
  });

  // #5130: while the cookie held the other school, the BFF refused this
  // tab's requests; after the switch back, mounted views load again.
  it("revalidates the mounted views after the switch went through", async () => {
    const mockUpdate = vi.fn().mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: mockUpdate,
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    mockPerformTenantSwitch.mockResolvedValue({
      access_token: "new-access",
      refresh_token: "new-refresh",
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
    await waitFor(() =>
      expect(mockMutate).toHaveBeenCalledWith(expect.any(Function)),
    );
    const [matcher] = mockMutate.mock.calls.at(-1) ?? [];
    expect((matcher as (key: unknown) => boolean)("any-key")).toBe(true);
  });

  // Regression test for #1975: the auto-switch must send the subdomain,
  // not the slug column — the two can legitimately differ.
  it("auto-switches with the subdomain when slug differs (#1975)", async () => {
    const mockUpdate = vi.fn().mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: mockUpdate,
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: { ...tenantB, slug: "ogs-school-b", subdomain: "school-b" },
    });
    mockPerformTenantSwitch.mockResolvedValue({
      access_token: "new-access",
      refresh_token: "new-refresh",
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
        "school-b",
        mockMutate,
      );
    });

    expect(mockUpdate).toHaveBeenCalled();
  });

  it("stops automatic switching and offers explicit target login when MFA proof is missing", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({ tenantSlug: "school-b", tenant: tenantB });
    mockPerformTenantSwitch.mockRejectedValue(
      new ApiError("second factor required", 401, {
        code: "identity.mfa_session_required",
      }),
    );
    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );
    expect(
      await screen.findByRole("button", { name: "Zur Schulanmeldung" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("School B");
    expect(mockPerformTenantSwitch).toHaveBeenCalledTimes(1);
    expect(mockSignOut).not.toHaveBeenCalled();
    expect(window.location.assign).not.toHaveBeenCalled();
  });

  it("signs out when switchTenant reports access denied", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    mockPerformTenantSwitch.mockRejectedValue(
      new ApiError("account does not have access to this tenant", 401, {
        code: "identity.tenant_access_denied",
      }),
    );

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalledWith({ callbackUrl: "/" });
    });
  });

  it("offers recovery when automatic sign-out after access denial fails", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    mockPerformTenantSwitch.mockRejectedValue(
      new ApiError("account does not have access to this tenant", 401, {
        code: "identity.tenant_access_denied",
      }),
    );
    mockSignOut.mockRejectedValue(new Error("sign-out failed"));

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    expect(
      await screen.findByRole("button", { name: "Erneut versuchen" }),
    ).toBeInTheDocument();
    expect(mockSignOut).toHaveBeenCalledWith({ callbackUrl: "/" });
    // The 401 names the missing access instead of jumping to the login.
    expect(
      await screen.findByText(
        catalogText(
          "identity.tenant_access_denied",
          "das Wechseln zu School B",
        ),
      ),
    ).toBeInTheDocument();
  });

  it.each(["/demo", "/school-a/demo"])(
    "leaves the demo entry page %s alone, whatever session arrives",
    async (pathname) => {
      mockUsePathname.mockReturnValue(pathname);
      mockUseSession.mockReturnValue({
        data: {
          user: { tenantId: undefined, scope: "platform", token: "operator" },
        },
        status: "authenticated",
        update: vi.fn(),
      });
      mockUseTenant.mockReturnValue({
        tenantSlug: "school-a",
        tenant: tenantA,
      });

      render(
        <TenantGuard>
          <div data-testid="child" />
        </TenantGuard>,
      );

      expect(screen.getByTestId("child")).toBeInTheDocument();
      await Promise.resolve();
      expect(mockSignOut).not.toHaveBeenCalled();
      expect(mockPerformTenantSwitch).not.toHaveBeenCalled();
    },
  );

  it("signs out operator session on tenant subdomain and blocks render", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          tenantId: undefined,
          scope: "platform",
          token: "operator-token",
        },
      },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-a",
      tenant: tenantA,
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    // Children must NOT render — operator session is blocked
    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
    expect(
      screen.getByText("Operator-Sitzung wird beendet..."),
    ).toBeInTheDocument();

    // Should clear caches, sign out, and redirect to tenant login
    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    });
    expect(mockMutate).toHaveBeenCalled();
    expect(mockClearSessionCache).toHaveBeenCalled();
    expect(window.location.assign).toHaveBeenCalledWith("/");

    // Should NOT attempt tenant switching
    expect(mockPerformTenantSwitch).not.toHaveBeenCalled();
  });

  it("redirects to tenant login even when signOut fails", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          tenantId: undefined,
          scope: "platform",
          token: "operator-token",
        },
      },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-a",
      tenant: tenantA,
    });
    mockSignOut.mockRejectedValue(new Error("network down"));

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    // Anker: die Guard zeigt den Abmeldehinweis der Operator-Sitzung.
    expect(
      screen.getByText("Operator-Sitzung wird beendet..."),
    ).toBeInTheDocument();
    // Children must NOT render
    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();

    // Even though signOut fails, redirect must still fire (finally block)
    await waitFor(() => {
      expect(window.location.assign).toHaveBeenCalledWith("/");
    });
  });

  it("redirects to tenant login even when cache clear fails", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          tenantId: undefined,
          scope: "platform",
          token: "operator-token",
        },
      },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-a",
      tenant: tenantA,
    });
    mockMutate.mockRejectedValue(new Error("cache error"));

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    // signOut should still be called despite cache clear failure
    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    });

    // Redirect must fire regardless
    expect(window.location.assign).toHaveBeenCalledWith("/");
  });

  it("skips check when session tenantId is undefined and scope is not platform", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          tenantId: undefined,
          scope: undefined,
          token: "access-token",
        },
      },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-a",
      tenant: tenantA,
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    // Should render children without attempting a switch
    expect(screen.getByText("Protected Content")).toBeInTheDocument();
    expect(mockPerformTenantSwitch).not.toHaveBeenCalled();
  });

  it("does not sign out on transient switch failures", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    mockPerformTenantSwitch.mockRejectedValue(new Error("network down"));

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
        "school-b",
        mockMutate,
      );
    });

    expect(mockSignOut).not.toHaveBeenCalled();
  });

  it("retries after a remount instead of persisting a failed slug marker", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    mockPerformTenantSwitch.mockRejectedValue(new Error("temporary failure"));

    const { unmount } = render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
        "school-b",
        mockMutate,
      );
    });

    expect(mockPerformTenantSwitch).toHaveBeenCalledTimes(1);

    unmount();
    mockPerformTenantSwitch.mockRejectedValueOnce(
      new Error("temporary failure"),
    );

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockPerformTenantSwitch).toHaveBeenCalledTimes(2);
    });

    expect(mockSignOut).not.toHaveBeenCalled();
  });

  it("renders children when tenant context is null", () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1, token: "access-token" } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-a",
      tenant: null,
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    // tenant is null — guard skips check, renders children
    expect(screen.getByText("Protected Content")).toBeInTheDocument();
    expect(mockPerformTenantSwitch).not.toHaveBeenCalled();
  });

  // The browser session carries no token (#5005); the session callback
  // marks a session without one with an error.
  it("signs out invalid authenticated session with no token", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          tenantId: 1,
          roles: [],
        },
        error: "RefreshTokenError",
      },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-a",
      tenant: tenantA,
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
    expect(screen.getByText("Sitzung wird erneuert...")).toBeInTheDocument();

    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    });
    expect(mockMutate).toHaveBeenCalled();
    expect(mockClearSessionCache).toHaveBeenCalled();
    expect(window.location.assign).toHaveBeenCalledWith(
      "/?error=SessionExpired",
    );
    expect(mockPerformTenantSwitch).not.toHaveBeenCalled();
  });

  it.each(["authenticated", "unauthenticated"])(
    "leaves an ended %s session to the active explicit logout",
    async (status) => {
      let finish!: () => void;
      const flight = runTenantSessionEnd(
        () =>
          new Promise<void>((resolve) => {
            finish = resolve;
          }),
      );
      await Promise.resolve();
      try {
        mockUseSession.mockReturnValue({
          data:
            status === "authenticated"
              ? { user: { tenantId: 1 }, error: "RefreshTokenError" }
              : null,
          status,
          update: vi.fn(),
        });
        mockUseTenant.mockReturnValue({
          tenantSlug: "school-a",
          tenant: tenantA,
        });
        await act(async () => {
          render(
            <TenantGuard>
              <div>Protected Content</div>
            </TenantGuard>,
          );
        });
        // Anker: the guard is mounted and protects the ended session.
        expect(
          screen.getByText("Sitzung wird erneuert..."),
        ).toBeInTheDocument();
        expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
        expect(mockSignOut).not.toHaveBeenCalled();
        expect(window.location.assign).not.toHaveBeenCalled();
      } finally {
        finish();
        await flight;
      }
    },
  );

  it("signs out authenticated session with refresh error", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          tenantId: 1,
          token: "access-token",
          roles: [],
        },
        error: "RefreshTokenError",
      },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-a",
      tenant: tenantA,
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
    expect(screen.getByText("Sitzung wird erneuert...")).toBeInTheDocument();

    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    });
    expect(window.location.assign).toHaveBeenCalledWith(
      "/?error=SessionExpired",
    );
  });

  // #3375: a switch that "succeeds" can still leave the old school in the
  // session cookie. The guard used to try once and then show
  // "Schule wird gewechselt..." forever.
  describe("when the mismatch survives the switch (#3375)", () => {
    function mismatchedSession() {
      mockUseSession.mockReturnValue({
        data: { user: { tenantId: 1, token: "access-token" } },
        status: "authenticated",
        update: vi.fn().mockResolvedValue(undefined),
      });
      mockUseTenant.mockReturnValue({
        tenantSlug: "school-b",
        tenant: tenantB,
      });
    }

    function renderGuard() {
      return render(
        <TenantGuard>
          <div>Protected Content</div>
        </TenantGuard>,
      );
    }

    it("repeats the switch a bounded number of times, then offers a way out", async () => {
      mismatchedSession();
      mockPerformTenantSwitch.mockResolvedValue({
        access_token: "new-access",
        refresh_token: "new-refresh",
      });

      renderGuard();

      expect(
        await screen.findByText(/Der Wechsel zu School B hat nicht geklappt/),
      ).toBeInTheDocument();
      expect(mockPerformTenantSwitch).toHaveBeenCalledTimes(3);
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
      expect(mockSignOut).not.toHaveBeenCalled();
    });

    it("starts a new retry budget when the URL target changes", async () => {
      mismatchedSession();
      mockPerformTenantSwitch.mockImplementation((subdomain: string) => {
        if (subdomain === "school-c") return new Promise(vi.fn());
        return Promise.resolve({
          access_token: "new-access",
          refresh_token: "new-refresh",
        });
      });

      const { rerender } = renderGuard();

      await screen.findByText(/Der Wechsel zu School B hat nicht geklappt/);
      expect(mockPerformTenantSwitch).toHaveBeenCalledTimes(3);

      mockUseTenant.mockReturnValue({
        tenantSlug: "school-c",
        tenant: tenantC,
      });
      rerender(
        <TenantGuard>
          <div>Protected Content</div>
        </TenantGuard>,
      );

      await waitFor(() => {
        expect(mockPerformTenantSwitch).toHaveBeenLastCalledWith(
          "school-c",
          mockMutate,
        );
      });
      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();
    });

    it("shows the way out after a failed switch instead of an endless wait", async () => {
      mismatchedSession();
      mockPerformTenantSwitch.mockRejectedValue(new Error("network down"));

      renderGuard();

      expect(
        await screen.findByRole("button", { name: "Erneut versuchen" }),
      ).toBeInTheDocument();
      expect(mockPerformTenantSwitch).toHaveBeenCalledTimes(1);
      expect(
        screen.queryByText("Schule wird gewechselt..."),
      ).not.toBeInTheDocument();
    });

    it("tries again on request", async () => {
      mismatchedSession();
      mockPerformTenantSwitch.mockRejectedValue(new Error("network down"));

      renderGuard();

      fireEvent.click(
        await screen.findByRole("button", { name: "Erneut versuchen" }),
      );

      await waitFor(() => {
        expect(mockPerformTenantSwitch).toHaveBeenCalledTimes(2);
      });
    });

    it("signs out on request", async () => {
      mismatchedSession();
      mockPerformTenantSwitch.mockRejectedValue(new Error("network down"));

      renderGuard();

      fireEvent.click(
        await screen.findByRole("button", { name: "Neu anmelden" }),
      );

      expect(mockSignOut).toHaveBeenCalledWith({ callbackUrl: "/" });
    });

    it("keeps the recovery actions when sign-out fails", async () => {
      mismatchedSession();
      mockPerformTenantSwitch.mockRejectedValue(new Error("network down"));
      mockSignOut.mockRejectedValue(new Error("sign-out failed"));

      renderGuard();

      fireEvent.click(
        await screen.findByRole("button", { name: "Neu anmelden" }),
      );

      await waitFor(() => {
        expect(mockSignOut).toHaveBeenCalledWith({ callbackUrl: "/" });
      });
      expect(
        screen.getByRole("button", { name: "Neu anmelden" }),
      ).toBeInTheDocument();
      expect(showActionError).toHaveBeenCalledWith(expect.any(Error), {
        object: "die Abmeldung",
      });
    });

    it("names the reason of a failed switch request with its catalog text", async () => {
      mismatchedSession();
      mockPerformTenantSwitch.mockRejectedValue(
        new ApiError("boom", 503, { code: "general.unavailable" }),
      );

      renderGuard();

      expect(
        await screen.findByText(
          catalogText("general.unavailable", "das Wechseln zu School B"),
        ),
      ).toBeInTheDocument();
      // "Erneut versuchen" below is the retry; the alert brings none twice.
      expect(
        screen.getAllByRole("button", { name: /Erneut versuchen|Wiederholen/ }),
      ).toHaveLength(1);
    });
  });
  // Review of #5147: the page of the old school stayed on screen, with its
  // buttons, until the provider answered; a tab on path mode was never
  // bound to its school (#5130).
  describe("when a tenant route refuses the session", () => {
    const sessionOf = (tenantId: number) => ({
      user: { tenantId, token: "access-token" },
    });

    /**
     * Provider stand-in. `answer` delivers a provider session the guard did
     * not ask for (a lookup that left earlier); `settleLookup` answers the
     * guard's own session lookup (update()) with the cookie's school, or
     * with null for a failed lookup.
     */
    let answer: (tenantId: number) => void = () => undefined;
    let settleLookup: (tenantId: number | null) => void = () => undefined;
    const providerUpdate = vi.fn();
    function Tab() {
      const [data, setData] = useState(sessionOf(1));
      answer = (tenantId) => setData(sessionOf(tenantId));
      providerUpdate.mockImplementation(
        () =>
          new Promise((resolve) => {
            settleLookup = (tenantId) => {
              const next = tenantId === null ? null : sessionOf(tenantId);
              if (next) setData(next);
              resolve(next);
            };
          }),
      );
      mockUseSession.mockReturnValue({
        data,
        status: "authenticated",
        update: providerUpdate,
      });
      return (
        <TenantGuard>
          <div>Protected Content</div>
        </TenantGuard>
      );
    }

    afterEach(() => {
      vi.unstubAllGlobals();
    });

    const CHECK_FAILED =
      "moto kann gerade nicht prüfen, bei welcher Schule Sie angemeldet sind. Bitte versuchen Sie es gleich noch einmal.";

    beforeEach(() => {
      providerUpdate.mockReset();
      // After a null lookup the guard asks the session route once; it fails
      // here unless a test answers it.
      vi.stubGlobal(
        "fetch",
        vi.fn(async () => new Response(null, { status: 503 })),
      );
      mockUseTenant.mockReturnValue({
        tenantSlug: "school-a",
        tenant: tenantA,
        routingMode: "subdomain",
      });
      // Keep the switch back open so the switching state stays visible.
      mockPerformTenantSwitch.mockReturnValue(new Promise(vi.fn()));
    });

    it("hides the page at once, before the provider answers", () => {
      render(<Tab />);
      // Anker: die Seite der eigenen Schule ist sichtbar.
      expect(screen.getByText("Protected Content")).toBeInTheDocument();

      act(() => reportSessionTenantMismatch());

      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
      expect(providerUpdate).toHaveBeenCalledTimes(1);
    });

    it("hides the page on a refusal of another account and says it reloads (#5183)", async () => {
      render(<Tab />);
      // Anker: die Seite der eigenen Schule ist sichtbar.
      expect(screen.getByText("Protected Content")).toBeInTheDocument();

      act(() =>
        reportSessionTenantMismatch("identity.session_account_mismatch"),
      );

      expect(screen.getByText("Seite wird neu geladen...")).toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
      expect(providerUpdate).toHaveBeenCalledTimes(1);

      // The lookup names this school: no switch; the new account's session
      // is the provider's now, and PersonalStorageGuard leaves the page.
      await act(async () => settleLookup(1));

      expect(screen.getByText("Protected Content")).toBeInTheDocument();
      expect(mockPerformTenantSwitch).not.toHaveBeenCalled();
    });

    it("shows the page again once its own lookup names this school", async () => {
      render(<Tab />);
      act(() => reportSessionTenantMismatch());

      await act(async () => settleLookup(1));

      expect(screen.getByText("Protected Content")).toBeInTheDocument();
    });

    it("switches back when its own lookup names the other school", async () => {
      render(<Tab />);
      act(() => reportSessionTenantMismatch());

      await act(async () => settleLookup(2));

      await waitFor(() =>
        expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
          "school-a",
          mockMutate,
        ),
      );
      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
    });

    // Review of #5175: a late answer, not the check, used to decide.
    it("keeps the page hidden when a late answer still names this school", () => {
      render(<Tab />);
      act(() => reportSessionTenantMismatch());

      // A lookup that left before the refusal answers with school 1.
      act(() => answer(1));

      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
    });

    it("switches back when a late answer names another school", async () => {
      render(<Tab />);
      act(() => reportSessionTenantMismatch());

      // A lookup that left before the refusal answers with school 2.
      act(() => answer(2));

      await waitFor(() =>
        expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
          "school-a",
          mockMutate,
        ),
      );
      // Anker: der Umschaltzustand steht da.
      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
    });

    // Review of #5175: after 10 s the old page came back and every further
    // refusal hid it again.
    it("offers a way out instead of the old page when the check times out", async () => {
      vi.useFakeTimers();
      render(<Tab />);
      act(() => reportSessionTenantMismatch());

      await act(async () => {
        await vi.advanceTimersByTimeAsync(9_999);
      });
      // Anker: bis zur Frist bleibt die Seite gesperrt.
      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1);
      });

      expect(screen.getByText(CHECK_FAILED)).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Erneut versuchen" }),
      ).toBeInTheDocument();
      // Review of #5175: no sign-out that a late answer could undo.
      expect(
        screen.queryByRole("button", { name: "Neu anmelden" }),
      ).not.toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
    });

    it("offers a way out when the lookup fails and checks again on request", async () => {
      render(<Tab />);
      act(() => reportSessionTenantMismatch());

      await act(async () => settleLookup(null));
      expect(screen.getByText(CHECK_FAILED)).toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Erneut versuchen" }));
      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();
      expect(providerUpdate).toHaveBeenCalledTimes(2);
      await act(async () => settleLookup(1));

      expect(screen.getByText("Protected Content")).toBeInTheDocument();
    });

    // quorum on #5175: a session that ended meanwhile (logout in another
    // tab) also answers null; retrying could never succeed.
    it("sends the tab to the login when the session ended meanwhile", async () => {
      const sessionRoute = vi.fn(async () => Response.json(null));
      vi.stubGlobal("fetch", sessionRoute);
      render(<Tab />);
      // Anker: die Seite der eigenen Schule ist sichtbar.
      expect(screen.getByText("Protected Content")).toBeInTheDocument();
      act(() => reportSessionTenantMismatch());

      await act(async () => settleLookup(null));

      await waitFor(() =>
        expect(window.location.assign).toHaveBeenCalledWith(
          "/?error=SessionExpired",
        ),
      );
      expect(sessionRoute).toHaveBeenCalledWith("/api/auth/session", {
        cache: "no-store",
        signal: expect.any(AbortSignal),
      });
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
    });

    // quorum on #5175: a stalled read of the session route kept the guard
    // in "checking" without a way out.
    it("offers a way out when the session route stalls after a null lookup", async () => {
      vi.useFakeTimers();
      vi.stubGlobal(
        "fetch",
        vi.fn(
          (_url: string, init?: RequestInit) =>
            new Promise<Response>((_resolve, reject) => {
              init?.signal?.addEventListener("abort", () =>
                reject(new DOMException("aborted", "AbortError")),
              );
            }),
        ),
      );
      render(<Tab />);
      act(() => reportSessionTenantMismatch());
      await act(async () => settleLookup(null));
      // Anker: die Prüfung läuft noch.
      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(10_000);
      });

      expect(screen.getByText(CHECK_FAILED)).toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
    });

    it("starts one check for repeated refusals and never shows the old page", async () => {
      render(<Tab />);
      act(() => {
        reportSessionTenantMismatch();
        reportSessionTenantMismatch();
        reportSessionTenantMismatch();
      });
      expect(providerUpdate).toHaveBeenCalledTimes(1);
      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();

      await act(async () => settleLookup(null));
      act(() => reportSessionTenantMismatch());

      // Anker: der Ausweg steht da, nicht die alte Seite.
      expect(screen.getByText(CHECK_FAILED)).toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
      expect(providerUpdate).toHaveBeenCalledTimes(1);
    });

    // tenant-mfa-switch E2E: once the provider names the other school, the
    // switch flow owns the screen; a further lookup raced its sign-out and
    // wrote the old cookie back.
    it("leaves refusals to the switch flow once the provider names another school", async () => {
      mockPerformTenantSwitch.mockRejectedValue(
        new ApiError("MFA required", 401, {
          code: "identity.mfa_session_required",
        }),
      );
      render(<Tab />);
      act(() => answer(2));
      expect(
        await screen.findByRole("button", { name: "Zur Schulanmeldung" }),
      ).toBeInTheDocument();

      act(() => {
        reportSessionTenantMismatch();
        reportSessionTenantMismatch();
      });

      // Anker: der Ausweg des Wechsels bleibt stehen.
      expect(
        screen.getByRole("button", { name: "Zur Schulanmeldung" }),
      ).toBeInTheDocument();
      expect(providerUpdate).not.toHaveBeenCalled();
    });

    it("asks again while another session lookup of the provider still runs", async () => {
      render(<Tab />);
      // The first update() returns nothing: another lookup still runs.
      providerUpdate.mockImplementationOnce(async () => undefined);
      act(() => reportSessionTenantMismatch());

      await waitFor(() => expect(providerUpdate).toHaveBeenCalledTimes(2));
      expect(screen.getByText("Schule wird gewechselt...")).toBeInTheDocument();
    });

    it("hides the page after a refused fetch of a client that never reads the body", async () => {
      vi.stubGlobal(
        "fetch",
        vi.fn(async () =>
          Response.json(
            { status: 409, code: "identity.session_tenant_mismatch" },
            { status: 409 },
          ),
        ),
      );
      Object.defineProperty(window, "location", {
        writable: true,
        value: {
          origin: "http://school-a.localhost:3000",
          pathname: "/students",
          assign: vi.fn(),
        },
      });
      // The guard's module installed the observer on the real fetch; wrap
      // this test's fetch instead.
      installSessionTenantMismatchObserver()();
      const restoreObserver = installSessionTenantMismatchObserver();
      try {
        render(<Tab />);
        // Anker: die Seite der eigenen Schule ist sichtbar.
        expect(screen.getByText("Protected Content")).toBeInTheDocument();

        await act(async () => {
          const response = await fetch("/api/students/school-classes");
          expect(response.ok).toBe(false);
        });

        expect(
          screen.getByText("Schule wird gewechselt..."),
        ).toBeInTheDocument();
        expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
      } finally {
        restoreObserver();
        vi.unstubAllGlobals();
      }
    });

    it("sends a tab still on path mode to the school's subdomain", () => {
      const redirect = vi.fn();
      mockUseTenant.mockReturnValue({
        tenantSlug: "school-a",
        tenant: tenantA,
        routingMode: "path",
      });
      Object.defineProperty(window, "location", {
        writable: true,
        value: {
          protocol: "https:",
          port: "",
          pathname: "/school-a/students",
          search: "?q=1",
          hash: "#top",
          assign: vi.fn(),
        },
      });
      mockUseSession.mockReturnValue({
        data: sessionOf(1),
        status: "authenticated",
        update: vi.fn(),
      });
      render(
        <TenantGuard redirect={redirect}>
          <div>Protected Content</div>
        </TenantGuard>,
      );
      // Anker: die Seite der eigenen Schule ist sichtbar.
      expect(screen.getByText("Protected Content")).toBeInTheDocument();

      act(() => reportSessionTenantMismatch());

      expect(redirect).toHaveBeenCalledWith(
        "https://school-a.localhost/students?q=1#top",
      );
    });
  });
});
