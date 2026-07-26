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

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    className,
    onClick,
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
    onClick?: () => void;
  }) => (
    <a href={href} className={className} onClick={onClick}>
      {children}
    </a>
  ),
}));

vi.mock("next/image", () => ({
  default: ({
    src,
    alt,
    width,
    height,
  }: {
    src: string;
    alt: string;
    width: number;
    height: number;
  }) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img src={src} alt={alt} width={width} height={height} />
  ),
}));

import { BrandTenantSwitcher } from "./tenant-switcher";

const TRIGGER_LABEL = "Einrichtung wechseln";

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

describe("BrandTenantSwitcher", () => {
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

  function openDropdown() {
    fireEvent.click(screen.getByRole("button", { name: TRIGGER_LABEL }));
  }

  it("skips tenant fetch when not authenticated and renders the plain brand", async () => {
    mockUseSession.mockReturnValue({ status: "loading" });
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<BrandTenantSwitcher label="School A" />);

    // Wait a tick to ensure the effect had a chance to fire
    await waitFor(() => {
      expect(mockListAvailableTenants).not.toHaveBeenCalled();
    });

    // No dropdown trigger — just the brand link
    expect(
      screen.queryByRole("button", { name: TRIGGER_LABEL }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("School A")).toBeInTheDocument();
  });

  it("renders the plain brand link when user has zero tenants", async () => {
    mockListAvailableTenants.mockResolvedValue([]);
    render(<BrandTenantSwitcher />);

    await waitFor(() => {
      expect(mockListAvailableTenants).toHaveBeenCalled();
    });

    expect(
      screen.queryByRole("button", { name: TRIGGER_LABEL }),
    ).not.toBeInTheDocument();
    // Fallback brand label when no tenant label is provided
    expect(screen.getByText("moto")).toBeInTheDocument();
  });

  it("renders name without dropdown when user has only one tenant", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA]);
    render(<BrandTenantSwitcher label="School A" />);

    await waitFor(() => {
      expect(mockListAvailableTenants).toHaveBeenCalled();
    });

    expect(screen.getByText("School A")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: TRIGGER_LABEL }),
    ).not.toBeInTheDocument();
  });

  it("renders trigger button with current tenant name when user has multiple tenants", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<BrandTenantSwitcher />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });
    expect(screen.getByText("School A")).toBeInTheDocument();
  });

  it("opens dropdown on click and shows other tenants", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<BrandTenantSwitcher />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });

    openDropdown();

    // Should show the other tenant
    expect(screen.getByText("School B")).toBeInTheDocument();
  });

  it("closes dropdown on outside click", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<BrandTenantSwitcher />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });

    openDropdown();
    expect(screen.getByText("School B")).toBeInTheDocument();

    // Click outside
    fireEvent.mouseDown(document.body);

    await waitFor(() => {
      expect(screen.queryByText("School B")).not.toBeInTheDocument();
    });
  });

  it("executes switch flow on tenant selection", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<BrandTenantSwitcher />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });

    openDropdown();
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

    render(<BrandTenantSwitcher />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });

    openDropdown();
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

    render(<BrandTenantSwitcher />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });

    openDropdown();

    // Should show org headers since there are 2 organizations
    expect(screen.getByText("Org Alpha")).toBeInTheDocument();
    expect(screen.getByText("Org Beta")).toBeInTheDocument();
  });

  it("links the current tenant entry to the dashboard instead of offering it as a switch target", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);

    render(<BrandTenantSwitcher href="/dashboard" />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });

    openDropdown();

    // Current tenant appears as a link home, never as a switch button
    const currentEntry = screen.getByRole("link", { name: /School A/ });
    expect(currentEntry).toHaveAttribute("href", "/dashboard");
    expect(
      screen.queryByRole("button", { name: "School A" }),
    ).not.toBeInTheDocument();

    // Selecting the current entry must not trigger a switch
    fireEvent.click(currentEntry);
    expect(mockPerformTenantSwitch).not.toHaveBeenCalled();
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

    render(<BrandTenantSwitcher />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });

    openDropdown();
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

    render(<BrandTenantSwitcher />);

    // currentSlug (URL) is "school-a" — must match via subdomain, so the
    // trigger shows the school name, not the raw slug fallback.
    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });
    expect(screen.getByText("School A")).toBeInTheDocument();

    // The current tenant must not appear as a switch-target button.
    openDropdown();
    expect(
      screen.queryByRole("button", { name: "School A" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "School B" }),
    ).toBeInTheDocument();
  });

  it("shows a visible error message when switching fails", async () => {
    mockListAvailableTenants.mockResolvedValue([tenantA, tenantB]);
    mockPerformTenantSwitch.mockRejectedValue(new Error("switch failed"));

    render(<BrandTenantSwitcher />);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: TRIGGER_LABEL }),
      ).toBeInTheDocument();
    });

    openDropdown();
    fireEvent.click(screen.getByText("School B"));

    await waitFor(() => {
      expect(
        screen.getByText(/Wechsel zu School B fehlgeschlagen/),
      ).toBeInTheDocument();
    });

    // Re-opening the dropdown clears the error.
    openDropdown();
    expect(
      screen.queryByText(/Wechsel zu School B fehlgeschlagen/),
    ).not.toBeInTheDocument();
  });

  it("falls back to the brand link and logs when listing tenants fails", async () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(vi.fn());
    mockListAvailableTenants.mockRejectedValue(new Error("fetch failed"));

    render(<BrandTenantSwitcher label="School A" />);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalled();
    });

    // Falls back to the plain brand — never a dropdown
    expect(
      screen.queryByRole("button", { name: TRIGGER_LABEL }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("School A")).toBeInTheDocument();
    consoleError.mockRestore();
  });
});
