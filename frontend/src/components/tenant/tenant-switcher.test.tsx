import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// ============================================================================
// Mocks
// ============================================================================

const { mockListAvailableTenants, mockSwitchTenant } = vi.hoisted(() => ({
  mockListAvailableTenants: vi.fn(),
  mockSwitchTenant: vi.fn(),
}));

const { mockSignIn } = vi.hoisted(() => ({
  mockSignIn: vi.fn(),
}));

const { mockMutate } = vi.hoisted(() => ({
  mockMutate: vi.fn(),
}));

const { mockClearSessionCache } = vi.hoisted(() => ({
  mockClearSessionCache: vi.fn(),
}));

const { mockUseTenantSlugSafe } = vi.hoisted(() => ({
  mockUseTenantSlugSafe: vi.fn(),
}));

vi.mock("~/lib/tenant-api", () => ({
  listAvailableTenants: mockListAvailableTenants,
  switchTenant: mockSwitchTenant,
}));

vi.mock("next-auth/react", () => ({
  signIn: mockSignIn,
}));

vi.mock("~/lib/swr", () => ({
  mutate: mockMutate,
}));

vi.mock("~/lib/session-cache", () => ({
  clearSessionCache: mockClearSessionCache,
}));

vi.mock("~/components/tenant/tenant-provider", () => ({
  useTenantSlugSafe: mockUseTenantSlugSafe,
}));

vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_TENANT_DOMAIN: "",
  },
}));

import { TenantSwitcher } from "./tenant-switcher";

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
  organizationId: 20,
  organizationName: "Org Beta",
  settings: {},
};

// ============================================================================
// Tests
// ============================================================================

describe("TenantSwitcher", () => {
  let originalLocation: Location;

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseTenantSlugSafe.mockReturnValue("school-a");
    mockListAvailableTenants.mockResolvedValue([]);
    mockSwitchTenant.mockResolvedValue({
      access_token: "new-access",
      refresh_token: "new-refresh",
    });
    mockSignIn.mockResolvedValue({ ok: true });
    mockMutate.mockResolvedValue(undefined);

    // Mock window.location
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
  });

  it("renders nothing when user has zero tenants", async () => {
    mockListAvailableTenants.mockResolvedValue([]);
    const { container } = render(<TenantSwitcher />);

    await waitFor(() => {
      expect(mockListAvailableTenants).toHaveBeenCalled();
    });

    expect(container.innerHTML).toBe("");
  });

  it("renders nothing when user has only one tenant", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA]);
    const { container } = render(<TenantSwitcher />);

    await waitFor(() => {
      expect(mockListAvailableTenants).toHaveBeenCalled();
    });

    expect(container.innerHTML).toBe("");
  });

  it("renders trigger button with current tenant name when user has multiple tenants", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });
  });

  it("opens dropdown on click and shows other tenants", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    // Click trigger to open dropdown
    fireEvent.click(screen.getByText("School A"));

    // Should show the other tenant
    expect(screen.getByText("School B")).toBeInTheDocument();
  });

  it("closes dropdown on outside click", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    // Open dropdown
    fireEvent.click(screen.getByText("School A"));
    expect(screen.getByText("School B")).toBeInTheDocument();

    // Click outside
    fireEvent.mouseDown(document.body);

    await waitFor(() => {
      expect(screen.queryByText("School B")).not.toBeInTheDocument();
    });
  });

  it("executes switch flow on tenant selection", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    // Open dropdown and select tenant B
    fireEvent.click(screen.getByText("School A"));
    fireEvent.click(screen.getByText("School B"));

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

    // Should redirect (path-based since TENANT_DOMAIN is empty)
    expect(window.location.href).toBe("/school-b/dashboard");
  });

  it("handles switch error gracefully", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);
    mockSwitchTenant.mockRejectedValue(new Error("switch failed"));

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    // Open dropdown and select tenant B
    fireEvent.click(screen.getByText("School A"));
    fireEvent.click(screen.getByText("School B"));

    await waitFor(() => {
      expect(mockSwitchTenant).toHaveBeenCalledWith("school-b");
    });

    // Should NOT redirect
    expect(window.location.href).toBe("");
    // Should NOT have called signIn
    expect(mockSignIn).not.toHaveBeenCalled();
  });

  it("shows organization headers when tenants span multiple orgs", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB, tenantC]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    // Open dropdown
    fireEvent.click(screen.getByText("School A"));

    // Should show org headers since there are 2 organizations
    expect(screen.getByText("Org Alpha")).toBeInTheDocument();
    expect(screen.getByText("Org Beta")).toBeInTheDocument();
  });

  it("logs error when listing tenants fails", async () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(vi.fn());
    mockListAvailableTenants.mockRejectedValue(new Error("fetch failed"));

    const { container } = render(<TenantSwitcher />);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalled();
    });

    // Should render nothing
    expect(container.innerHTML).toBe("");
    consoleError.mockRestore();
  });
});
