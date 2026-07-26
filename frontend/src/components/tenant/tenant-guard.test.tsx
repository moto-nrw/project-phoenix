import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// ============================================================================
// Mocks
// ============================================================================

const { mockUseSession, mockSignIn, mockSignOut } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockSignIn: vi.fn(),
  mockSignOut: vi.fn(),
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
  TenantSwitchError: class TenantSwitchError extends Error {
    status: number;
    code: "access_denied" | "unknown";

    constructor(
      message: string,
      status: number,
      code: "access_denied" | "unknown" = "unknown",
    ) {
      super(message);
      this.name = "TenantSwitchError";
      this.status = status;
      this.code = code;
    }
  },
}));

vi.mock("~/lib/swr", () => ({
  mutate: mockMutate,
}));

vi.mock("~/lib/session-cache", () => ({
  clearSessionCache: mockClearSessionCache,
}));

import { TenantGuard } from "./tenant-guard";
import { TenantSwitchError } from "~/lib/tenant-api";

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

// ============================================================================
// Tests
// ============================================================================

describe("TenantGuard", () => {
  let originalLocation: Location;

  beforeEach(() => {
    vi.clearAllMocks();
    mockMutate.mockResolvedValue(undefined);
    mockSignIn.mockResolvedValue({ ok: true });
    mockSignOut.mockResolvedValue(undefined);
    sessionStorage.clear();

    // Mock window.location for redirect assertions
    originalLocation = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...originalLocation, href: "", assign: vi.fn() },
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
      new TenantSwitchError(
        "account does not have access to this tenant",
        401,
        "access_denied",
      ),
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

  it("signs out invalid authenticated session with no token", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          tenantId: 1,
          token: "",
          roles: [],
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
});
