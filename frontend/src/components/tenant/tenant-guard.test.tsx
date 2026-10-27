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
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { reportTenantSessionMismatch } from "~/lib/tenant-session-mismatch";

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

vi.mock("~/lib/session-cache", () => ({
  clearSessionCache: mockClearSessionCache,
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

    expect(screen.getByText("Mandant wird gewechselt...")).toBeInTheDocument();
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
        mockSignIn,
        mockMutate,
      );
    });

    expect(mockUpdate).toHaveBeenCalled();
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
        mockSignIn,
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
        mockSignIn,
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
        mockSignIn,
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
  // "Mandant wird gewechselt..." forever.
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
          mockSignIn,
          mockMutate,
        );
      });
      expect(
        screen.getByText("Mandant wird gewechselt..."),
      ).toBeInTheDocument();
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
        screen.queryByText("Mandant wird gewechselt..."),
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
  // #5130: a tenant route refused a request because the shared cookie holds
  // another school's session. The tab drops its caches, asks the provider
  // for the session and switches back to its own school.
  describe("when a tenant route refuses the session (#5130)", () => {
    const sessionOf = (tenantId: number) => ({
      user: { tenantId, token: "access-token" },
    });

    /** Provider stand-in: update() answers with what the cookie holds now. */
    function Tab({ cookieTenantId }: { cookieTenantId: number }) {
      const [data, setData] = useState(sessionOf(1));
      mockUseSession.mockReturnValue({
        data,
        status: "authenticated",
        update: mockProviderUpdate.mockImplementation(async () => {
          const next = sessionOf(cookieTenantId);
          setData(next);
          return next;
        }),
      });
      return (
        <TenantGuard>
          <div>Protected Content</div>
        </TenantGuard>
      );
    }

    const mockProviderUpdate = vi.fn();

    beforeEach(() => {
      mockProviderUpdate.mockReset();
      mockUseTenant.mockReturnValue({
        tenantSlug: "school-a",
        tenant: tenantA,
      });
      // Keep the switch back open so the switching state stays visible.
      mockPerformTenantSwitch.mockReturnValue(new Promise(vi.fn()));
    });

    it("drops the caches, reloads the session and switches back", async () => {
      render(<Tab cookieTenantId={2} />);
      // Anker: die Seite der eigenen Schule ist sichtbar.
      expect(screen.getByText("Protected Content")).toBeInTheDocument();

      act(() => reportTenantSessionMismatch());

      expect(
        await screen.findByText("Mandant wird gewechselt..."),
      ).toBeInTheDocument();
      expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
      expect(mockClearSessionCache).toHaveBeenCalled();
      expect(mockMutate).toHaveBeenCalledWith(expect.any(Function), undefined, {
        revalidate: false,
      });
      const matcher = mockMutate.mock.calls[0]?.[0] as (
        key: unknown,
      ) => boolean;
      expect(matcher("/api/students")).toBe(true);
      // The caches are gone before the provider's answer can render anything.
      expect(mockMutate.mock.invocationCallOrder[0]).toBeLessThan(
        mockProviderUpdate.mock.invocationCallOrder[0]!,
      );
      await waitFor(() =>
        expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
          "school-a",
          mockSignIn,
          mockMutate,
        ),
      );
    });

    it("asks the provider once while a reload runs", async () => {
      let finishUpdate: () => void = () => undefined;
      mockUseSession.mockReturnValue({
        data: sessionOf(1),
        status: "authenticated",
        update: mockProviderUpdate.mockImplementation(
          () =>
            new Promise<void>((resolve) => {
              finishUpdate = resolve;
            }),
        ),
      });
      render(
        <TenantGuard>
          <div>Protected Content</div>
        </TenantGuard>,
      );
      // Anker: die Seite der eigenen Schule ist sichtbar.
      expect(screen.getByText("Protected Content")).toBeInTheDocument();

      act(() => {
        reportTenantSessionMismatch();
        reportTenantSessionMismatch();
      });
      await waitFor(() => expect(mockProviderUpdate).toHaveBeenCalledTimes(1));

      await act(async () => finishUpdate());
      act(() => reportTenantSessionMismatch());
      await waitFor(() => expect(mockProviderUpdate).toHaveBeenCalledTimes(2));
    });
  });
});
