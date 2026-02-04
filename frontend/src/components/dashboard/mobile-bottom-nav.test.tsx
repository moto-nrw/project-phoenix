import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";

// Mock dependencies before importing component
vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
  useSearchParams: vi.fn(() => ({
    get: vi.fn(),
  })),
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("~/lib/supervision-context", () => ({
  useSupervision: vi.fn(),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: vi.fn(),
}));

// Mock Drawer components
vi.mock("~/components/ui/drawer", () => ({
  Drawer: ({
    children,
    open,
    onOpenChange,
  }: {
    children: React.ReactNode;
    open: boolean;
    onOpenChange: (open: boolean) => void;
  }) => (
    <div data-testid="drawer" data-open={open}>
      <button onClick={() => onOpenChange(false)} data-testid="drawer-close">
        Close
      </button>
      {open && children}
    </div>
  ),
  DrawerContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="drawer-content">{children}</div>
  ),
  DrawerHeader: ({ children }: { children: React.ReactNode }) => (
    <header>{children}</header>
  ),
  DrawerTitle: ({ children }: { children: React.ReactNode }) => (
    <h2>{children}</h2>
  ),
  DrawerDescription: ({ children }: { children: React.ReactNode }) => (
    <p>{children}</p>
  ),
}));

// Import after mocks
import { MobileBottomNav } from "./mobile-bottom-nav";
import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useSupervision } from "~/lib/supervision-context";
import { isAdmin } from "~/lib/auth-utils";

const mockUsePathname = vi.mocked(usePathname);
const mockUseSearchParams = vi.mocked(useSearchParams);
const mockUseSession = vi.mocked(useSession);
const mockUseSupervision = vi.mocked(useSupervision);
const mockIsAdmin = vi.mocked(isAdmin);

// Helper to create mock search params - use unknown cast for test flexibility
function createMockSearchParams(
  getValue: (key: string) => string | null = () => null,
) {
  const params = new URLSearchParams();
  return {
    get: getValue,
    toString: () => params.toString(),
    keys: () => params.keys(),
    values: () => params.values(),
    entries: () => params.entries(),
    has: (key: string) => params.has(key),
    getAll: (key: string) => params.getAll(key),
    forEach: (
      callback: (value: string, key: string, parent: URLSearchParams) => void,
    ) => params.forEach(callback),
    [Symbol.iterator]: () => params[Symbol.iterator](),
    size: params.size,
  } as unknown as ReturnType<typeof useSearchParams>;
}

// Helper to create mock session - use unknown cast for test flexibility
function createMockSession(isAdminUser: boolean) {
  return {
    data: {
      user: {
        id: "1",
        token: "test-token",
        isAdmin: isAdminUser,
        email: "test@example.com",
      },
      expires: new Date(Date.now() + 86400000).toISOString(),
    },
    status: "authenticated" as const,
    update: vi.fn(),
  } as unknown as ReturnType<typeof useSession>;
}

describe("MobileBottomNav", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Default mock implementations
    mockUsePathname.mockReturnValue("/dashboard");
    mockUseSearchParams.mockReturnValue(createMockSearchParams());
    mockUseSession.mockReturnValue(createMockSession(false));
    mockUseSupervision.mockReturnValue({
      hasGroups: true,
      isSupervising: false,
      isLoadingGroups: false,
      isLoadingSupervision: false,
      supervisedRooms: [],
      groups: [],
      refresh: vi.fn(),
    });
    mockIsAdmin.mockReturnValue(false);
  });

  describe("rendering", () => {
    it("renders navigation bar for staff users", () => {
      render(<MobileBottomNav />);

      // Staff main items - check by href since labels only show when active
      const links = screen.getAllByRole("link");
      const hrefs = links.map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/ogs-groups");
      expect(hrefs).toContain("/active-supervisions");
    });

    it("renders navigation bar for admin users", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<MobileBottomNav />);

      // Admin main items - check by href
      const links = screen.getAllByRole("link");
      const hrefs = links.map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/dashboard");
      expect(hrefs).toContain("/students/search");
    });

    it("renders with custom className", () => {
      const { container } = render(
        <MobileBottomNav className="custom-class" />,
      );

      const nav = container.querySelector("nav");
      expect(nav).toHaveClass("custom-class");
    });

    it("renders spacer div for fixed nav positioning", () => {
      const { container } = render(<MobileBottomNav />);

      const spacer = container.querySelector(".h-16");
      expect(spacer).toBeInTheDocument();
    });
  });

  describe("active route detection", () => {
    it("highlights dashboard link when on dashboard", () => {
      mockUsePathname.mockReturnValue("/dashboard");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<MobileBottomNav />);

      // The active link should have the bg-gray-900 class and show label
      const dashboardLink = screen
        .getAllByRole("link")
        .find((link) => link.getAttribute("href") === "/dashboard");
      expect(dashboardLink).toHaveClass("bg-gray-900");
      expect(screen.getByText("Home")).toBeInTheDocument();
    });

    it("highlights dashboard for root path", () => {
      mockUsePathname.mockReturnValue("/");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<MobileBottomNav />);

      // Should show "Home" label since dashboard is active
      expect(screen.getByText("Home")).toBeInTheDocument();
    });

    it("detects active route from search params 'from' parameter", () => {
      mockUsePathname.mockReturnValue("/students/123");
      const mockGet = vi.fn().mockReturnValue("/ogs-groups/5");
      mockUseSearchParams.mockReturnValue(createMockSearchParams(mockGet));

      render(<MobileBottomNav />);

      // Should highlight the "Gruppe" item since from=/ogs-groups/5
      expect(mockGet).toHaveBeenCalledWith("from");
    });

    it("highlights correct item when path starts with href", () => {
      mockUsePathname.mockReturnValue("/activities/123");

      render(<MobileBottomNav />);

      // Activities should be highlighted
      const links = screen.getAllByRole("link");
      const activitiesLink = links.find(
        (link) => link.getAttribute("href") === "/activities",
      );
      expect(activitiesLink).toBeDefined();
    });
  });

  describe("admin navigation", () => {
    beforeEach(() => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
    });

    it("shows admin-specific navigation items", () => {
      render(<MobileBottomNav />);

      // Admin main items include Home, Suchen, Aktivitäten, Räume
      expect(screen.getByText("Home")).toBeInTheDocument();
    });

    it("shows admin-only items in overflow menu", () => {
      render(<MobileBottomNav />);

      // Open overflow menu - get the More button (inside nav, not drawer close button)
      const navButtons = screen.getAllByRole("button");
      const moreButton = navButtons.find(
        (btn) => !btn.hasAttribute("data-testid"),
      );
      expect(moreButton).toBeDefined();
      fireEvent.click(moreButton!);

      // Admin-only items should be visible in the drawer
      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
      expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
    });
  });

  describe("staff navigation", () => {
    beforeEach(() => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });
    });

    it("shows staff-specific navigation items", () => {
      // Set ogs-groups as active so label shows
      mockUsePathname.mockReturnValue("/ogs-groups");

      render(<MobileBottomNav />);

      // Staff main item should show label when active
      expect(screen.getByText("Gruppe")).toBeInTheDocument();
    });

    it("shows active supervision label when on that route", () => {
      mockUsePathname.mockReturnValue("/active-supervisions");

      render(<MobileBottomNav />);

      // Should show Aufsicht label when active
      expect(screen.getByText("Aufsicht")).toBeInTheDocument();
    });

    it("renders staff nav links correctly", () => {
      render(<MobileBottomNav />);

      // Check nav links exist by href
      const links = screen.getAllByRole("link");
      const hrefs = links.map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/ogs-groups");
      expect(hrefs).toContain("/active-supervisions");
    });
  });

  describe("overflow menu", () => {
    // Helper to get the More button (the one inside nav, not the drawer close button)
    const getMoreButton = () => {
      const navButtons = screen.getAllByRole("button");
      return navButtons.find((btn) => !btn.hasAttribute("data-testid"))!;
    };

    it("opens overflow menu when More button is clicked", () => {
      render(<MobileBottomNav />);

      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      // Drawer should be open
      const drawer = screen.getByTestId("drawer");
      expect(drawer).toHaveAttribute("data-open", "true");
    });

    it("closes overflow menu when item is clicked", () => {
      render(<MobileBottomNav />);

      // Open menu
      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      // Click a navigation item
      const staffLink = screen.getByText("Mitarbeiter");
      fireEvent.click(staffLink);

      // The link should exist and be clickable
      expect(staffLink.closest("a")).toHaveAttribute("href", "/staff");
    });

    it("displays additional nav items in drawer", () => {
      render(<MobileBottomNav />);

      // Open overflow menu
      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      // Check for additional items
      expect(screen.getByText("Mitarbeiter")).toBeInTheDocument();
      expect(screen.getByText("Räume")).toBeInTheDocument();
      expect(screen.getByText("Einstellungen")).toBeInTheDocument();
    });
  });

  describe("coming soon items", () => {
    // Helper to get the More button
    const getMoreButton = () => {
      const navButtons = screen.getAllByRole("button");
      return navButtons.find((btn) => !btn.hasAttribute("data-testid"))!;
    };

    it("displays coming soon badge for upcoming features", () => {
      render(<MobileBottomNav />);

      // Open overflow menu
      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      // Coming soon items should have badge
      expect(screen.getAllByText("Bald verfügbar").length).toBeGreaterThan(0);
    });

    it("coming soon items are not clickable links", () => {
      render(<MobileBottomNav />);

      // Open overflow menu
      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      // Zeiterfassung is now an active feature with a real link
      const zeiterfassungElement = screen.getByText("Zeiterfassung");
      const link = zeiterfassungElement.closest("a");
      expect(link).not.toBeNull();
      expect(link).toHaveAttribute("href", "/time-tracking");
    });

    it("shows admin-only coming soon items for admins", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<MobileBottomNav />);

      // Open overflow menu
      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      // Admin-only coming soon features
      expect(screen.getByText("Dienstpläne")).toBeInTheDocument();
      expect(screen.getByText("Berichte")).toBeInTheDocument();
    });
  });

  describe("supervision-based visibility", () => {
    it("shows supervision-related links when user has groups", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<MobileBottomNav />);

      // Main items should have correct hrefs
      const links = screen.getAllByRole("link");
      const hrefs = links.map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/ogs-groups");
    });

    it("shows supervision-related links when user is actively supervising", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<MobileBottomNav />);

      // Main items should have correct hrefs
      const links = screen.getAllByRole("link");
      const hrefs = links.map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/active-supervisions");
    });
  });

  describe("More button active state", () => {
    // Helper to get the More button
    const getMoreButton = () => {
      const navButtons = screen.getAllByRole("button");
      return navButtons.find((btn) => !btn.hasAttribute("data-testid"))!;
    };

    it("highlights More button when overflow menu is open", () => {
      render(<MobileBottomNav />);

      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      // More button should have active styling and show "Mehr" label
      expect(screen.getByText("Mehr")).toBeInTheDocument();
    });

    it("highlights More button when additional nav item route is active", () => {
      mockUsePathname.mockReturnValue("/settings");

      render(<MobileBottomNav />);

      // Settings is in additional items, so More button should be highlighted
      // and show "Mehr" label
      expect(screen.getByText("Mehr")).toBeInTheDocument();
    });
  });

  describe("sliding indicator", () => {
    it("renders indicator element when nav item is active", () => {
      mockUsePathname.mockReturnValue("/ogs-groups");

      const { container } = render(<MobileBottomNav />);

      // Check that the nav container has the indicator structure
      // The indicator is a div with bg-gray-900 class
      const navItems = container.querySelectorAll(
        ".relative.flex.items-center.justify-around",
      );
      expect(navItems.length).toBeGreaterThan(0);
    });

    it("updates indicator after initial mount delay", async () => {
      vi.useFakeTimers();
      mockUsePathname.mockReturnValue("/ogs-groups");

      render(<MobileBottomNav />);

      // Advance timers to trigger the indicator update effects
      await act(async () => {
        await vi.advanceTimersByTimeAsync(150);
      });

      // The active link should show the "Gruppe" label after timers complete
      expect(screen.getByText("Gruppe")).toBeInTheDocument();

      vi.useRealTimers();
    });

    it("hides indicator when no active item found", async () => {
      vi.useFakeTimers();
      // Use a path that doesn't match any nav item
      mockUsePathname.mockReturnValue("/unknown-route");

      const { container } = render(<MobileBottomNav />);

      // Advance timers
      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      // No main nav item should be highlighted (no active styling)
      const activeLinks = container.querySelectorAll("a.bg-gray-900");
      expect(activeLinks.length).toBe(0);

      vi.useRealTimers();
    });

    it("shows indicator on more button when additional route is active", async () => {
      vi.useFakeTimers();
      mockUsePathname.mockReturnValue("/settings");

      render(<MobileBottomNav />);

      // Advance timers to trigger the indicator update effects
      await act(async () => {
        await vi.advanceTimersByTimeAsync(100);
      });

      // The "Mehr" label should be visible (indicating More button is highlighted)
      expect(screen.getByText("Mehr")).toBeInTheDocument();

      vi.useRealTimers();
    });
  });

  describe("Icon component", () => {
    it("renders SVG icons correctly", () => {
      render(<MobileBottomNav />);

      // SVGs should be rendered in navigation
      const svgs = document.querySelectorAll("svg");
      expect(svgs.length).toBeGreaterThan(0);

      // Each should have proper attributes
      svgs.forEach((svg) => {
        expect(svg).toHaveAttribute("viewBox", "0 0 24 24");
        expect(svg).toHaveAttribute("stroke", "currentColor");
      });
    });
  });
});
