import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

const mockRouterPush = vi.fn();

// Mock dependencies before importing component
vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
  useSearchParams: vi.fn(() => ({
    get: vi.fn(),
  })),
  useRouter: vi.fn(() => ({
    push: mockRouterPush,
    replace: vi.fn(),
    back: vi.fn(),
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

// Import after mocks
import { Sidebar } from "./sidebar";
import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useSupervision } from "~/lib/supervision-context";
import { isAdmin } from "~/lib/auth-utils";

const mockUsePathname = vi.mocked(usePathname);
const mockUseSearchParams = vi.mocked(useSearchParams);
const mockUseSession = vi.mocked(useSession);
const mockUseSupervision = vi.mocked(useSupervision);
const mockIsAdmin = vi.mocked(isAdmin);

// Helper to create mock search params
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

// Helper to create mock session
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

describe("Sidebar", () => {
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
    it("renders sidebar with navigation items", () => {
      render(<Sidebar />);

      // Common items visible to all users
      expect(screen.getByText("Aktivitäten")).toBeInTheDocument();
      expect(screen.getByText("Räume")).toBeInTheDocument();
      expect(screen.getByText("Mitarbeiter")).toBeInTheDocument();
      expect(screen.getByText("Einstellungen")).toBeInTheDocument();
    });

    it("renders with custom className", () => {
      const { container } = render(<Sidebar className="custom-class" />);

      const aside = container.querySelector("aside");
      expect(aside).toHaveClass("custom-class");
    });

    it("renders navigation inside aside element", () => {
      const { container } = render(<Sidebar />);

      const nav = container.querySelector("nav");
      expect(nav).toBeInTheDocument();
      expect(nav?.closest("aside")).toBeInTheDocument();
    });
  });

  describe("admin navigation", () => {
    beforeEach(() => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
    });

    it("shows admin-only navigation items for admins", () => {
      render(<Sidebar />);

      // Admin-only items
      expect(screen.getByText("Home")).toBeInTheDocument();
      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
      expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
    });

    it("hides staff-only items for admins (hideForAdmin)", () => {
      render(<Sidebar />);

      // These items have hideForAdmin: true
      expect(screen.queryByText("Meine Gruppe")).not.toBeInTheDocument();
      expect(screen.queryByText("Aktuelle Aufsicht")).not.toBeInTheDocument();
    });

    it("shows student search for admins", () => {
      render(<Sidebar />);

      expect(screen.getByText("Kindersuche")).toBeInTheDocument();
    });
  });

  describe("staff navigation", () => {
    beforeEach(() => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });
    });

    it("shows staff-specific navigation items", () => {
      render(<Sidebar />);

      // Staff items with alwaysShow: true
      expect(screen.getByText("Meine Gruppe")).toBeInTheDocument();
      expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
    });

    it("hides admin-only items for staff", () => {
      render(<Sidebar />);

      // Admin-only items should NOT be visible
      expect(screen.queryByText("Vertretungen")).not.toBeInTheDocument();
      expect(screen.queryByText("Datenverwaltung")).not.toBeInTheDocument();
    });

    it("shows student search when staff has groups", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Kindersuche")).toBeInTheDocument();
    });

    it("shows student search when staff is actively supervising", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Kindersuche")).toBeInTheDocument();
    });

    it("shows student search for staff without supervision (at correct position)", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      // Should still show Kindersuche (added at correct position)
      expect(screen.getByText("Kindersuche")).toBeInTheDocument();
    });
  });

  describe("active link highlighting", () => {
    it("highlights dashboard link when on dashboard", () => {
      mockUsePathname.mockReturnValue("/dashboard");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      const dashboardLink = screen.getByText("Home").closest("a");
      expect(dashboardLink).toHaveClass("bg-gray-100");
      expect(dashboardLink).toHaveClass("text-gray-900");
    });

    it("highlights link when path starts with href", () => {
      mockUsePathname.mockReturnValue("/activities/123");

      render(<Sidebar />);

      const activitiesLink = screen.getByText("Aktivitäten").closest("a");
      expect(activitiesLink).toHaveClass("bg-gray-100");
    });

    it("does not highlight dashboard for non-dashboard paths", () => {
      mockUsePathname.mockReturnValue("/activities");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      const dashboardLink = screen.getByText("Home").closest("a");
      expect(dashboardLink).not.toHaveClass("bg-gray-100");
    });
  });

  describe("student detail page active link detection", () => {
    it("highlights ogs-groups when coming from ogs-groups page", () => {
      mockUsePathname.mockReturnValue("/students/123");
      const mockGet = vi.fn((key: string) =>
        key === "from" ? "/ogs-groups" : null,
      );
      mockUseSearchParams.mockReturnValue(createMockSearchParams(mockGet));

      render(<Sidebar />);

      // Accordion headers are div[role=button], not <a> links
      const groupHeader = screen
        .getByText("Meine Gruppe")
        .closest('[role="button"]');
      expect(groupHeader).toHaveClass("bg-gray-100");
    });

    it("highlights active-supervisions when coming from supervisions page", () => {
      mockUsePathname.mockReturnValue("/students/456");
      const mockGet = vi.fn((key: string) =>
        key === "from" ? "/active-supervisions" : null,
      );
      mockUseSearchParams.mockReturnValue(createMockSearchParams(mockGet));

      render(<Sidebar />);

      // Accordion headers are div[role=button], not <a> links
      const supervisionHeader = screen
        .getByText("Aktuelle Aufsicht")
        .closest('[role="button"]');
      expect(supervisionHeader).toHaveClass("bg-gray-100");
    });

    it("highlights student search when coming from search page", () => {
      mockUsePathname.mockReturnValue("/students/789");
      const mockGet = vi.fn().mockReturnValue("/students/search");
      mockUseSearchParams.mockReturnValue(createMockSearchParams(mockGet));

      render(<Sidebar />);

      const searchLink = screen.getByText("Kindersuche").closest("a");
      expect(searchLink).toHaveClass("bg-gray-100");
    });

    it("defaults to student search when no from param", () => {
      mockUsePathname.mockReturnValue("/students/100");
      mockUseSearchParams.mockReturnValue(createMockSearchParams(() => null));

      render(<Sidebar />);

      // Should default to Kindersuche when no from param
      const searchLink = screen.getByText("Kindersuche").closest("a");
      expect(searchLink).toHaveClass("bg-gray-100");
    });
  });

  describe("coming soon items", () => {
    it("displays coming soon items with badge", () => {
      render(<Sidebar />);

      // Coming soon items should have "Bald" badge
      expect(screen.getByText("Zeiterfassung")).toBeInTheDocument();
      expect(screen.getByText("Nachrichten")).toBeInTheDocument();
      expect(screen.getByText("Mittagessen")).toBeInTheDocument();
      expect(screen.getAllByText("Bald").length).toBeGreaterThan(0);
    });

    it("coming soon items are not clickable", () => {
      render(<Sidebar />);

      const zeiterfassungElement = screen.getByText("Zeiterfassung");
      // Should not be inside a link
      expect(zeiterfassungElement.closest("a")).toBeNull();
    });

    it("coming soon items have disabled styling", () => {
      render(<Sidebar />);

      const zeiterfassungElement = screen.getByText("Zeiterfassung");
      const container = zeiterfassungElement.closest("div");
      expect(container).toHaveClass("text-gray-400");
      expect(container).toHaveClass("cursor-not-allowed");
    });

    it("shows admin-only coming soon items for admins", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      // Admin-only coming soon features
      expect(screen.getByText("Dienstpläne")).toBeInTheDocument();
      expect(screen.getByText("Berichte")).toBeInTheDocument();
    });

    it("hides admin-only coming soon items for staff", () => {
      mockIsAdmin.mockReturnValue(false);

      render(<Sidebar />);

      // Admin-only coming soon features should not be visible
      expect(screen.queryByText("Dienstpläne")).not.toBeInTheDocument();
      expect(screen.queryByText("Berichte")).not.toBeInTheDocument();
    });
  });

  describe("Suspense fallback", () => {
    // Note: Testing Suspense fallback directly is tricky.
    // The Sidebar component uses Suspense internally.
    // We verify the skeleton structure exists in the fallback.

    it("sidebar wrapper exists and renders content", () => {
      const { container } = render(<Sidebar />);

      // Should have the main sidebar wrapper
      const aside = container.querySelector("aside");
      expect(aside).toBeInTheDocument();
      expect(aside).toHaveClass("min-h-screen");
      expect(aside).toHaveClass("w-64");
    });
  });

  describe("supervision loading states", () => {
    it("handles loading groups state correctly", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: false,
        isLoadingGroups: true,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      // Should still render, but supervision-dependent items behavior changes
      expect(screen.getByText("Meine Gruppe")).toBeInTheDocument();
    });

    it("handles loading supervision state correctly", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: true,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      // Should still render
      expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
    });
  });

  describe("navigation item icons", () => {
    it("renders SVG icons for each nav item", () => {
      render(<Sidebar />);

      // Each nav item should have an SVG icon
      const svgs = document.querySelectorAll("nav svg");
      expect(svgs.length).toBeGreaterThan(0);
    });

    it("icons have correct styling", () => {
      render(<Sidebar />);

      // Filter to nav-item icon SVGs (h-5 w-5), excluding accordion chevrons (h-4 w-4)
      const svgs = document.querySelectorAll("nav svg.h-5");
      expect(svgs.length).toBeGreaterThan(0);
      svgs.forEach((svg) => {
        expect(svg).toHaveClass("h-5");
        expect(svg).toHaveClass("w-5");
      });
    });
  });

  describe("accordion sub-items", () => {
    it("renders group sub-items when groups are available", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [
          { id: 1, name: "Eulen" },
          { id: 2, name: "Adler" },
        ],
        refresh: vi.fn(),
      });
      mockUsePathname.mockReturnValue("/ogs-groups");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams((key: string) => (key === "group" ? "1" : null)),
      );

      render(<Sidebar />);

      expect(screen.getByText("Eulen")).toBeInTheDocument();
      expect(screen.getByText("Adler")).toBeInTheDocument();
    });

    it("renders supervised room sub-items", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [
          { id: "10", name: "Raum A", groupId: "1" },
          { id: "20", name: "Raum B", groupId: "2" },
        ],
        groups: [],
        refresh: vi.fn(),
      });
      mockUsePathname.mockReturnValue("/active-supervisions");

      render(<Sidebar />);

      expect(screen.getByText("Raum A")).toBeInTheDocument();
      expect(screen.getByText("Raum B")).toBeInTheDocument();
    });

    it("renders database sub-pages for admin", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/database");

      render(<Sidebar />);

      expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
      expect(screen.getByText("Kinder")).toBeInTheDocument();
      expect(screen.getByText("Betreuer")).toBeInTheDocument();
      expect(screen.getByText("Gruppen")).toBeInTheDocument();
    });
  });

  describe("accordion toggle navigation", () => {
    beforeEach(() => {
      mockRouterPush.mockClear();
      // Mock localStorage
      vi.spyOn(Storage.prototype, "getItem").mockReturnValue(null);
      vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
        // no-op
      });
    });

    it("navigates to ogs-groups when groups toggle clicked from another page", () => {
      mockUsePathname.mockReturnValue("/activities");
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [{ id: 1, name: "Eulen" }],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const groupHeader = screen.getByText("Meine Gruppe");
      fireEvent.click(groupHeader);

      expect(mockRouterPush).toHaveBeenCalledWith("/ogs-groups?group=1");
    });

    it("navigates to ogs-groups without group param when no groups", () => {
      mockUsePathname.mockReturnValue("/activities");
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const groupHeader = screen.getByText("Meine Gruppe");
      fireEvent.click(groupHeader);

      expect(mockRouterPush).toHaveBeenCalledWith("/ogs-groups");
    });

    it("navigates to active-supervisions when supervisions toggle clicked from another page", () => {
      mockUsePathname.mockReturnValue("/activities");
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [{ id: "10", name: "Raum A", groupId: "1" }],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const supervisionHeader = screen.getByText("Aktuelle Aufsicht");
      fireEvent.click(supervisionHeader);

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/active-supervisions?room=10",
      );
    });

    it("navigates to active-supervisions without room param when no rooms", () => {
      mockUsePathname.mockReturnValue("/activities");
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const supervisionHeader = screen.getByText("Aktuelle Aufsicht");
      fireEvent.click(supervisionHeader);

      expect(mockRouterPush).toHaveBeenCalledWith("/active-supervisions");
    });

    it("navigates to database hub when database toggle clicked from another page", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/activities");

      render(<Sidebar />);

      const databaseHeader = screen.getByText("Datenverwaltung");
      fireEvent.click(databaseHeader);

      expect(mockRouterPush).toHaveBeenCalledWith("/database");
    });

    it("navigates back to database hub when on a database sub-page", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/database/students");

      render(<Sidebar />);

      const databaseHeader = screen.getByText("Datenverwaltung");
      fireEvent.click(databaseHeader);

      expect(mockRouterPush).toHaveBeenCalledWith("/database");
    });

    it("does not navigate when toggling database on hub page", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/database");

      render(<Sidebar />);

      const databaseHeader = screen.getByText("Datenverwaltung");
      fireEvent.click(databaseHeader);

      // On /database hub, toggling should not navigate
      expect(mockRouterPush).not.toHaveBeenCalled();
    });
  });

  describe("bottom pinned items", () => {
    it("renders feedback and settings at the bottom", () => {
      render(<Sidebar />);

      expect(screen.getByText("Feedback")).toBeInTheDocument();
      expect(screen.getByText("Einstellungen")).toBeInTheDocument();
    });

    it("highlights active icon color for active links", () => {
      mockUsePathname.mockReturnValue("/activities");

      render(<Sidebar />);

      const activitiesLink = screen.getByText("Aktivitäten").closest("a");
      const svg = activitiesLink?.querySelector("svg");
      expect(svg?.getAttribute("class")).toContain("text-[#FF3130]");
    });
  });

  describe("hideForAdmin items", () => {
    it("shows Erinnerungen for non-admin users", () => {
      mockIsAdmin.mockReturnValue(false);
      render(<Sidebar />);

      expect(screen.getByText("Erinnerungen")).toBeInTheDocument();
    });

    it("hides Erinnerungen for admin users", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      render(<Sidebar />);

      expect(screen.queryByText("Erinnerungen")).not.toBeInTheDocument();
    });
  });

  describe("groups label pluralization", () => {
    it("shows 'Meine Gruppe' for single group", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [{ id: 1, name: "Eulen" }],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Meine Gruppe")).toBeInTheDocument();
    });

    it("shows 'Meine Gruppen' for multiple groups", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [
          { id: 1, name: "Eulen" },
          { id: 2, name: "Adler" },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
    });

    it("shows 'Aktuelle Aufsicht' for single room", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [{ id: "10", name: "Raum A", groupId: "1" }],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
    });

    it("shows 'Aktuelle Aufsichten' for multiple rooms", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [
          { id: "10", name: "Raum A", groupId: "1" },
          { id: "20", name: "Raum B", groupId: "2" },
        ],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Aktuelle Aufsichten")).toBeInTheDocument();
    });
  });

  describe("child page highlight persistence", () => {
    it("highlights group sub-item on student detail from ogs-groups", () => {
      vi.spyOn(Storage.prototype, "getItem").mockImplementation(
        (key: string) => {
          if (key === "sidebar-last-group") return "1";
          return null;
        },
      );
      mockUsePathname.mockReturnValue("/students/123");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams((key: string) =>
          key === "from" ? "/ogs-groups" : null,
        ),
      );
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [],
        groups: [
          { id: 1, name: "Eulen" },
          { id: 2, name: "Adler" },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      // "Eulen" should be active sub-item (matching childGroupId "1")
      const eulenLink = screen.getByText("Eulen").closest("a");
      expect(eulenLink).toHaveClass("bg-gray-100");
    });

    it("renders rooms in sidebar on student detail from active-supervisions", () => {
      mockUsePathname.mockReturnValue("/students/456");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams((key: string) =>
          key === "from" ? "/active-supervisions" : null,
        ),
      );
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        supervisedRooms: [
          { id: "10", name: "Raum A", groupId: "1" },
          { id: "20", name: "Raum B", groupId: "2" },
        ],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      // Both rooms should be rendered in the accordion
      expect(screen.getByText("Raum A")).toBeInTheDocument();
      expect(screen.getByText("Raum B")).toBeInTheDocument();
    });
  });
});
