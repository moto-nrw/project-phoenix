import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// ============================================================================
// Mocks
// ============================================================================

const { mockListAvailableTenants, mockPerformTenantSwitch } = vi.hoisted(
  () => ({
    mockListAvailableTenants: vi.fn(),
    mockPerformTenantSwitch: vi.fn(),
  }),
);

const { mockSignIn, mockUseSession } = vi.hoisted(() => ({
  mockSignIn: vi.fn(),
  mockUseSession: vi.fn(),
}));

const { mockMutate } = vi.hoisted(() => ({
  mockMutate: vi.fn(),
}));

const { mockUseTenantSlugSafe } = vi.hoisted(() => ({
  mockUseTenantSlugSafe: vi.fn(),
}));

vi.mock("~/lib/tenant-api", () => ({
  listAvailableTenants: mockListAvailableTenants,
  performTenantSwitch: mockPerformTenantSwitch,
}));

vi.mock("next-auth/react", () => ({
  signIn: mockSignIn,
  useSession: mockUseSession,
}));

vi.mock("~/lib/swr", () => ({
  mutate: mockMutate,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: mockUseTenantSlugSafe,
  useNFCEnabled: vi.fn(() => true),
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
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockUseTenantSlugSafe.mockReturnValue("school-a");
    mockListAvailableTenants.mockResolvedValue([]);
    mockPerformTenantSwitch.mockResolvedValue({
      access_token: "new-access",
      refresh_token: "new-refresh",
    });
    mockSignIn.mockResolvedValue({ ok: true });
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

  it("skips tenant fetch when not authenticated", async () => {
    mockUseSession.mockReturnValue({ status: "loading" });
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    const { container } = render(<TenantSwitcher />);

    // Wait a tick to ensure the effect had a chance to fire
    await waitFor(() => {
      expect(mockListAvailableTenants).not.toHaveBeenCalled();
    });

    expect(container.innerHTML).toBe("");
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
      expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
        "school-b",
        mockSignIn,
        mockMutate,
      );
    });

    // Should redirect to the new tenant subdomain
    expect(window.location.href).toBe(
      "http://school-b.localhost:3000/dashboard",
    );
  });

  it("handles switch error gracefully", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);
    mockPerformTenantSwitch.mockRejectedValue(new Error("switch failed"));

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    // Open dropdown and select tenant B
    fireEvent.click(screen.getByText("School A"));
    fireEvent.click(screen.getByText("School B"));

    await waitFor(() => {
      expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
        "school-b",
        mockSignIn,
        mockMutate,
      );
    });

    // Should NOT redirect (performTenantSwitch threw)
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

  // Regression tests for #1975: slug and subdomain are independent columns
  // and can differ (prod example: slug "ogs-burbach", subdomain "burbach").
  // The switch contract is subdomain-based.

  it("sends the subdomain, not the slug, when they differ (#1975)", async () => {
    const divergentTenantB = {
      ...tenantB,
      slug: "ogs-school-b",
      subdomain: "school-b",
    };
    mockListAvailableTenants.mockResolvedValue([tenantA, divergentTenantB]);

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("School A"));
    fireEvent.click(screen.getByText("School B"));

    await waitFor(() => {
      expect(mockPerformTenantSwitch).toHaveBeenCalledWith(
        "school-b",
        mockSignIn,
        mockMutate,
      );
    });

    expect(window.location.href).toBe(
      "http://school-b.localhost:3000/dashboard",
    );
  });

  it("matches the current tenant by subdomain when slug differs (#1975)", async () => {
    const divergentTenantA = {
      ...tenantA,
      slug: "ogs-school-a",
      subdomain: "school-a",
    };
    mockListAvailableTenants.mockResolvedValue([divergentTenantA, tenantB]);

    render(<TenantSwitcher />);

    // currentSlug (URL) is "school-a" — must match via subdomain, so the
    // trigger shows the school name, not the raw slug fallback.
    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    // The current tenant must not appear as a switch target.
    fireEvent.click(screen.getByText("School A"));
    expect(screen.getAllByText("School A")).toHaveLength(1);
    expect(screen.getByText("School B")).toBeInTheDocument();
  });

  it("shows a visible error message when switching fails", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);
    mockPerformTenantSwitch.mockRejectedValue(new Error("switch failed"));

    render(<TenantSwitcher />);

    await waitFor(() => {
      expect(screen.getByText("School A")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("School A"));
    fireEvent.click(screen.getByText("School B"));

    await waitFor(() => {
      expect(
        screen.getByText(/Wechsel zu School B fehlgeschlagen/),
      ).toBeInTheDocument();
    });

    // Re-opening the dropdown clears the error.
    fireEvent.click(screen.getByText("School A"));
    expect(
      screen.queryByText(/Wechsel zu School B fehlgeschlagen/),
    ).not.toBeInTheDocument();
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
