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

const { mockSwitchTenant } = vi.hoisted(() => ({
  mockSwitchTenant: vi.fn(),
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

vi.mock("~/components/tenant/tenant-provider", () => ({
  useTenant: mockUseTenant,
}));

vi.mock("~/lib/tenant-api", () => ({
  switchTenant: mockSwitchTenant,
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
      value: { ...originalLocation, href: "" },
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
      data: { user: { tenantId: 1 } },
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

  it("shows loading state when session is loading", () => {
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

    expect(screen.getByText("Sitzung wird geladen...")).toBeInTheDocument();
    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
  });

  it("shows switching state when mismatch detected", () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1 } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    // Prevent the switch from resolving immediately
    mockSwitchTenant.mockReturnValue(new Promise(vi.fn()));

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    expect(screen.getByText("Mandant wird gewechselt...")).toBeInTheDocument();
    expect(screen.queryByText("Protected Content")).not.toBeInTheDocument();
  });

  it("calls switchTenant + signIn + cache clear on mismatch", async () => {
    const mockUpdate = vi.fn().mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1 } },
      status: "authenticated",
      update: mockUpdate,
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    mockSwitchTenant.mockResolvedValue({
      access_token: "new-access",
      refresh_token: "new-refresh",
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockSwitchTenant).toHaveBeenCalledWith("school-b");
    });

    expect(mockSignIn).toHaveBeenCalledWith("credentials", {
      redirect: false,
      internalRefresh: true,
      token: "new-access",
      refreshToken: "new-refresh",
    });

    expect(mockMutate).toHaveBeenCalled();
    expect(mockClearSessionCache).toHaveBeenCalled();
    expect(mockUpdate).toHaveBeenCalled();
  });

  it("signs out when switchTenant fails", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1 } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    mockSwitchTenant.mockRejectedValue(new Error("No access to tenant"));

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalledWith({ callbackUrl: "/" });
    });

    // Should mark the slug as failed in sessionStorage
    expect(sessionStorage.getItem("tenant-switch-failed")).toBe("school-b");
  });

  it("skips check when session tenantId is undefined", () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: undefined } },
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
    expect(mockSwitchTenant).not.toHaveBeenCalled();
  });

  it("breaks loop by signing out when previous switch to same slug failed", async () => {
    // Simulate a previous failed switch attempt stored in sessionStorage
    sessionStorage.setItem("tenant-switch-failed", "school-b");

    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1 } },
      status: "authenticated",
      update: vi.fn(),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalledWith({ callbackUrl: "/" });
    });

    // Should NOT attempt the actual switch
    expect(mockSwitchTenant).not.toHaveBeenCalled();

    // Should clean up the sessionStorage flag
    expect(sessionStorage.getItem("tenant-switch-failed")).toBeNull();
  });

  it("clears failure flag on successful switch", async () => {
    // Set a stale failure flag for a different slug
    sessionStorage.setItem("tenant-switch-failed", "school-c");

    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1 } },
      status: "authenticated",
      update: vi.fn().mockResolvedValue(undefined),
    });
    mockUseTenant.mockReturnValue({
      tenantSlug: "school-b",
      tenant: tenantB,
    });
    mockSwitchTenant.mockResolvedValue({
      access_token: "new-access",
      refresh_token: "new-refresh",
    });

    render(
      <TenantGuard>
        <div>Protected Content</div>
      </TenantGuard>,
    );

    await waitFor(() => {
      expect(mockSwitchTenant).toHaveBeenCalledWith("school-b");
    });

    // Success should clear the failure flag
    await waitFor(() => {
      expect(sessionStorage.getItem("tenant-switch-failed")).toBeNull();
    });
  });

  it("renders children when tenant context is null", () => {
    mockUseSession.mockReturnValue({
      data: { user: { tenantId: 1 } },
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
    expect(mockSwitchTenant).not.toHaveBeenCalled();
  });
});
