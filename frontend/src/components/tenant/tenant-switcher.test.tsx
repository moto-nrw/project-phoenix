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
    NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
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
    mockMutate.mockResolvedValue(undefined);

    // Mock window.location with realistic values for subdomain-based redirect
    originalLocation = window.location;
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...originalLocation, href: "", protocol: "http:", port: "3000" },
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

    // Should clear caches before navigating away
    expect(mockMutate).toHaveBeenCalled();
    expect(mockClearSessionCache).toHaveBeenCalled();

    // Should redirect to callback page with tokens in hash fragment
    expect(window.location.href).toBe(
      "http://school-b.localhost:3000/auth/tenant-callback#t=new-access&r=new-refresh",
    );
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

  it("omits port from redirect URL when no port is set", async () => {
    Object.defineProperty(window, "location", {
      writable: true,
      value: { ...window.location, href: "", protocol: "https:", port: "" },
    });

    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("School A"));
    fireEvent.click(screen.getByText("School B"));

    await waitFor(() => {
      expect(mockSwitchTenant).toHaveBeenCalledWith("school-b");
    });

    expect(window.location.href).toBe(
      "https://school-b.localhost/auth/tenant-callback#t=new-access&r=new-refresh",
    );
  });

  it("falls back to slug when current tenant is not in list", async () => {
    mockUseTenantSlugSafe.mockReturnValue("unknown-slug");
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      // Should fall back to showing the slug since no tenant matches
      expect(screen.getByText("unknown-slug")).toBeInTheDocument();
    });
  });

  it("uses 'Andere' as fallback org name when organizationName is empty", async () => {
    const tenantNoOrg = {
      ...tenantB,
      tenantId: 99,
      slug: "no-org",
      name: "No Org Tenant",
      subdomain: "no-org",
      organizationName: "",
    };

    mockListAvailableTenants.mockResolvedValue([tenantA, tenantC, tenantNoOrg]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("School A"));

    // Should show org headers including the fallback
    expect(screen.getByText("Org Beta")).toBeInTheDocument();
    expect(screen.getByText("Andere")).toBeInTheDocument();
  });

  it("logs non-Error exceptions as strings on switch failure", async () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(vi.fn());
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);
    mockSwitchTenant.mockRejectedValue("string error");

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("School A"));
    fireEvent.click(screen.getByText("School B"));

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith("tenant_switch_failed", {
        error: "string error",
        target_slug: "school-b",
      });
    });

    consoleError.mockRestore();
  });

  it("prevents concurrent switches when already switching", async () => {
    // Make switchTenant hang to keep isSwitching=true
    let resolveSwitchTenant: (value: unknown) => void;
    mockSwitchTenant.mockReturnValue(
      new Promise((resolve) => {
        resolveSwitchTenant = resolve;
      }),
    );

    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB, tenantC]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    // Start first switch
    fireEvent.click(screen.getByText("School A"));
    fireEvent.click(screen.getByText("School B"));

    await waitFor(() => {
      expect(mockSwitchTenant).toHaveBeenCalledTimes(1);
    });

    // The trigger button should now be disabled
    const trigger = screen.getByRole("button");
    expect(trigger).toBeDisabled();

    // Resolve the pending switch to clean up
    resolveSwitchTenant!({
      access_token: "tok",
      refresh_token: "ref",
    });
  });
});
