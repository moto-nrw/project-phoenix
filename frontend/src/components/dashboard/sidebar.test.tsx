import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
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
  useOptionalSupervision: vi.fn(),
}));

vi.mock("~/lib/auth-utils", () => {
  const isAdminFn = vi.fn();
  return {
    isAdmin: isAdminFn,
    isCaregiver: vi.fn(() => !isAdminFn()),
    hasRole: vi.fn((_session: unknown, role: string) => {
      if (role === "admin") return isAdminFn();
      if (role === "user") return !isAdminFn();
      return false;
    }),
    // Elternmitteilungen (#1669) gates on this; admins hold it via admin:*.
    // The nav item additionally requires operations.parent_news_enabled (off
    // in these tests' settings schema), so it stays hidden regardless.
    hasPermission: vi.fn((_session: unknown, _permission: string) =>
      isAdminFn(),
    ),
  };
});

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: vi.fn(),
}));

vi.mock("~/lib/operator-url", () => ({
  operatorPath: (path: string) => path,
}));

vi.mock("~/lib/hooks/use-staff-absences-pending", () => ({
  useStaffAbsencesPending: vi.fn(() => ({
    unreadCount: 0,
    isLoading: false,
    refresh: vi.fn(),
  })),
}));

vi.mock("~/lib/hooks/use-change-requests-pending", () => ({
  useChangeRequestsPending: vi.fn(() => ({
    unreadCount: 0,
    isLoading: false,
    refresh: vi.fn(),
  })),
}));

vi.mock("~/lib/hooks/use-messages-unread", () => ({
  useMessagesUnread: vi.fn(() => ({
    unreadCount: 0,
    isLoading: false,
    refresh: vi.fn(),
  })),
}));

vi.mock("~/lib/hooks/use-enrollment-requests-pending", () => ({
  useEnrollmentRequestsPending: vi.fn(() => ({
    unreadCount: 0,
    isLoading: false,
    refresh: vi.fn(),
  })),
}));

vi.mock("~/lib/hooks/use-care-withdrawals-pending", () => ({
  useCareWithdrawalsPending: vi.fn(() => ({
    unreadCount: 0,
    isLoading: false,
    refresh: vi.fn(),
  })),
}));

// Import after mocks
import { Sidebar } from "./sidebar";
import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useStaffAbsencesPending } from "~/lib/hooks/use-staff-absences-pending";
import { useChangeRequestsPending } from "~/lib/hooks/use-change-requests-pending";
import { useCareWithdrawalsPending } from "~/lib/hooks/use-care-withdrawals-pending";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
  useStaffMessagingEnabled,
  useTenantRoutingModeSafe,
} from "~/lib/tenant-context";
import useSWR from "swr";

const mockUsePathname = vi.mocked(usePathname);
const mockUseSearchParams = vi.mocked(useSearchParams);
const mockUseSession = vi.mocked(useSession);
const mockUseSupervision = vi.mocked(useOptionalSupervision);
const mockIsAdmin = vi.mocked(isAdmin);
const mockHasPermission = vi.mocked(hasPermission);
// Standardverhalten des geteilten hasPermission-Mocks: Rechte hat nur der
// Admin. Tests, die einzelne Rechte gezielt vergeben, stellen darüber wieder
// den Ausgangszustand her (vi.clearAllMocks setzt Implementierungen nicht
// zurück).
const restoreDefaultHasPermission = () =>
  mockHasPermission.mockImplementation((session) => mockIsAdmin(session));
const mockUseShellAuth = vi.mocked(useShellAuth);
const mockUseStaffAbsencesPending = vi.mocked(useStaffAbsencesPending);
const mockUseChangeRequestsPending = vi.mocked(useChangeRequestsPending);
const mockUseCareWithdrawalsPending = vi.mocked(useCareWithdrawalsPending);
const mockUsePresenceMode = vi.mocked(usePresenceMode);
const mockUseNFCEnabled = vi.mocked(useNFCEnabled);
const mockUseOpenCareGroupMode = vi.mocked(useOpenCareGroupMode);
const mockUseStaffMessagingEnabled = vi.mocked(useStaffMessagingEnabled);
const mockUseTenantRoutingModeSafe = vi.mocked(useTenantRoutingModeSafe);
const mockUseSWRDefault = vi.mocked(useSWR);

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
    localStorage.clear();

    // Default mock implementations
    mockUseShellAuth.mockReturnValue({
      user: { name: "Test User", email: "test@example.com", roles: [] },
      profile: { firstName: "Test", lastName: "User" },
      status: "authenticated",
      isSessionExpired: false,
      logout: vi.fn(),
      mode: "teacher",
      homeUrl: "/dashboard",

      profileUrl: "/profile",
    });
    mockUsePathname.mockReturnValue("/dashboard");
    mockUseSearchParams.mockReturnValue(createMockSearchParams());
    mockUseSession.mockReturnValue(createMockSession(false));
    mockUseSupervision.mockReturnValue({
      hasGroups: true,
      isSupervising: false,
      isLoadingGroups: false,
      isLoadingSupervision: false,
      overviewEnabled: false,
      supervisedRooms: [],
      groups: [],
      refresh: vi.fn(),
    });
    mockIsAdmin.mockReturnValue(false);
    restoreDefaultHasPermission();
    mockUsePresenceMode.mockReturnValue("detailed");
    mockUseNFCEnabled.mockReturnValue(true);
    mockUseOpenCareGroupMode.mockReturnValue(false);
    mockUseStaffMessagingEnabled.mockReturnValue(false);
    mockUseTenantRoutingModeSafe.mockReturnValue("path");
    mockUseSWRDefault.mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true,
      isValidating: false,
      mutate: vi.fn(),
    } as unknown as ReturnType<typeof useSWR>);
    mockUseStaffAbsencesPending.mockReturnValue({
      unreadCount: 0,
      isLoading: false,
      refresh: vi.fn(),
    });
    mockUseChangeRequestsPending.mockReturnValue({
      unreadCount: 0,
      isLoading: false,
      refresh: vi.fn(),
    });
    mockUseCareWithdrawalsPending.mockReturnValue({
      unreadCount: 0,
      isLoading: false,
      refresh: vi.fn(),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("rendering", () => {
    it("renders sidebar with navigation items", () => {
      render(<Sidebar />);

      // Common items visible to all users
      expect(screen.getByText("Aktivitäten")).toBeInTheDocument();
      expect(screen.getByText("Räume")).toBeInTheDocument();
      expect(screen.getByText("Mitarbeitende")).toBeInTheDocument();
      // Einstellungen is admin-only (requiresAdmin: true in sidebar nav items)
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
      expect(screen.getByText("Start")).toBeInTheDocument();
      // Der Bereich „Datenverwaltung" ist aufgelöst; der Vertretungszugriff
      // ist ein Reiter bei den Mitarbeitenden.
      expect(screen.queryByText("Datenverwaltung")).not.toBeInTheDocument();
      expect(screen.queryByText("Vertretungszugriff")).not.toBeInTheDocument();
      // Die Planungsbereiche sind Unterpunkte des Planung-Akkordeons (#1946),
      // inklusive Zeiträume und Kalender.
      expect(screen.getByText("Planung")).toBeInTheDocument();
      expect(screen.getByText("Betreuungsplan")).toBeInTheDocument();
      expect(screen.getByText("Dienstplan")).toBeInTheDocument();
      expect(screen.getByText("Vertretung")).toBeInTheDocument();
      expect(screen.getByText("Zeiträume")).toBeInTheDocument();
    });

    it("labels the personal calendar entry 'Kalender'", () => {
      // Ein Begriff, ein Wort: der Eintrag heißt überall „Kalender".
      render(<Sidebar />);

      expect(screen.getByText("Kalender").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/calendar",
      );
      expect(screen.queryByText("Mein Kalender")).not.toBeInTheDocument();
    });

    it("does not carry the removed dynamic accordions", () => {
      render(<Sidebar />);

      expect(screen.queryByText("Meine Gruppe")).not.toBeInTheDocument();
      expect(screen.queryByText("Aktuelle Aufsicht")).not.toBeInTheDocument();
    });

    it("shows the children entry with the children concept icon", () => {
      mockUsePathname.mockReturnValue("/students/search");
      render(<Sidebar />);

      const link = screen.getByText("Kinder").closest("a");
      expect(link).toBeInTheDocument();
      expect(link?.querySelector("svg")).toBeInTheDocument();
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
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });
    });

    it("shows staff-specific navigation items", () => {
      render(<Sidebar />);

      // Staff items with alwaysShow: true
      expect(screen.getByText("Kinder")).toBeInTheDocument();
      expect(screen.getByText("Räume")).toBeInTheDocument();
      expect(screen.getByText("Mitarbeitende")).toBeInTheDocument();
    });

    it("prefixes Anfragen and aggregates the visible request counts", () => {
      mockHasPermission.mockImplementation(
        (_session, permission) =>
          permission === "users:update" ||
          permission === "users:delete" ||
          permission === "vacation:approve",
      );
      mockUseChangeRequestsPending.mockReturnValue({
        unreadCount: 2,
        isLoading: false,
        refresh: vi.fn(),
      });
      mockUseStaffAbsencesPending.mockReturnValue({
        unreadCount: 3,
        isLoading: false,
        refresh: vi.fn(),
      });
      mockUseCareWithdrawalsPending.mockReturnValue({
        unreadCount: 4,
        isLoading: false,
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      // „Anfragen" steht seit dem Navigationsumbau als Unterpunkt im
      // Eltern-Bereich; der Zähler hängt am Eintrag und am Bereichskopf.
      expect(screen.getByText("Anfragen").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/anfragen",
      );
    });

    it("prefixes the Team-Chat link in path-routing mode", () => {
      mockUseStaffMessagingEnabled.mockReturnValue(true);

      render(<Sidebar />);

      expect(screen.getByText("Team-Chat").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/team-chat",
      );
    });

    it("hides admin-only items for staff", () => {
      render(<Sidebar />);

      // Admin-only items should NOT be visible
      expect(screen.queryByText("Datenverwaltung")).not.toBeInTheDocument();
      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Dienstplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Vertretung")).not.toBeInTheDocument();
    });

    it("shows student search when staff has groups", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Kinder")).toBeInTheDocument();
    });

    it("shows student search when staff is actively supervising", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Kinder")).toBeInTheDocument();
    });

    it("shows student search for staff without supervision (at correct position)", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      // Should still show Kindersuche (added at correct position)
      expect(screen.getByText("Kinder")).toBeInTheDocument();
    });
  });

  describe("active link highlighting", () => {
    it("highlights dashboard link when on dashboard", () => {
      mockUsePathname.mockReturnValue("/dashboard");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      const dashboardLink = screen.getByText("Start").closest("a");
      expect(dashboardLink).toHaveClass("bg-gray-100");
      expect(dashboardLink).toHaveClass("text-gray-900");
    });

    it("highlights the canonical activities link", () => {
      mockUsePathname.mockReturnValue("/activities");

      render(<Sidebar />);

      const activitiesLink = screen.getByText("Aktivitäten").closest("a");
      expect(activitiesLink).toHaveClass("bg-gray-100");
    });

    it("highlights Dienstplan without also highlighting Mitarbeiter", () => {
      // /staff/dienstplan ist nur noch der Redirect-Frame des Dienstplans
      // (Planung-Redesign) und darf die Mitarbeiter-Sektion nicht färben.
      mockUsePathname.mockReturnValue("/staff/dienstplan");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      const dienstplanLink = screen.getByText("Dienstplan").closest("a");
      const staffLink = screen.getByText("Mitarbeitende").closest("a");
      expect(dienstplanLink).toHaveClass("bg-gray-100");
      expect(staffLink).not.toHaveClass("bg-gray-100");
    });

    it("expands and highlights Planung under a tenant-prefixed path", () => {
      mockUsePathname.mockReturnValue("/test-tenant/dienstplan");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      expect(screen.getByText("Dienstplan").closest("a")).toHaveClass(
        "bg-gray-100",
      );
      expect(screen.getByText("Planung")).toBeInTheDocument();
    });

    it("does not strip a route-like slug in subdomain mode", () => {
      mockUseTenantRoutingModeSafe.mockReturnValue("subdomain");
      mockUsePathname.mockReturnValue("/dienstplan");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      expect(screen.getByText("Dienstplan").closest("a")).toHaveClass(
        "bg-gray-100",
      );
    });

    it("highlights Zeiträume on /calendar-periods", () => {
      // Die Zeitraum-Verwaltung ist eigener Unterpunkt im Planung-Akkordeon
      // (#1946) und leuchtet dort selbst, nicht mehr im Betreuungsplan.
      mockUsePathname.mockReturnValue("/calendar-periods");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      const periodsLink = screen.getByText("Zeiträume").closest("a");
      const betreuungsplanLink = screen
        .getByText("Betreuungsplan")
        .closest("a");
      const kalenderLink = screen.getByText("Kalender").closest("a");
      expect(periodsLink).toHaveClass("bg-gray-100");
      expect(betreuungsplanLink).not.toHaveClass("bg-gray-100");
      // /calendar darf nicht per Präfix auf /calendar-periods mitleuchten.
      expect(kalenderLink).not.toHaveClass("bg-gray-100");
    });

    it("does not highlight dashboard for non-dashboard paths", () => {
      mockUsePathname.mockReturnValue("/activities");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      const dashboardLink = screen.getByText("Start").closest("a");
      expect(dashboardLink).not.toHaveClass("bg-gray-100");
    });

    it("does not render the removed enrollment reports subpage", () => {
      mockUsePathname.mockReturnValue("/admin/enrollments");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      // Der Hub der Anmeldungen heißt wie der Block selbst; er steht als
      // Unterpunkt im Eltern-Bereich.
      const hubLinks = screen.getAllByText("Anmeldungen");
      expect(hubLinks[0]?.closest("a")).toHaveClass("bg-gray-100");
    });
  });

  describe("student detail page active link detection", () => {
    it("highlights student search when coming from search page", () => {
      mockUsePathname.mockReturnValue("/students/789");
      const mockGet = vi.fn().mockReturnValue("/students/search");
      mockUseSearchParams.mockReturnValue(createMockSearchParams(mockGet));

      render(<Sidebar />);

      const searchLink = screen.getByText("Kinder").closest("a");
      expect(searchLink).toHaveClass("bg-gray-100");
    });

    it("highlights rooms when coming from a room detail page", () => {
      mockUsePathname.mockReturnValue("/students/321");
      const mockGet = vi.fn((key: string) =>
        key === "from" ? "/rooms/42" : null,
      );
      mockUseSearchParams.mockReturnValue(createMockSearchParams(mockGet));

      render(<Sidebar />);

      const roomsLink = screen.getByText("Räume").closest("a");
      expect(roomsLink).toHaveClass("bg-gray-100");
    });

    it("highlights rooms when coming from the room detail modal", () => {
      // Modal flow at /rooms?room={id} (#1374) — sidebar highlight has
      // to match the legacy subpage drill-in for the same UX.
      mockUsePathname.mockReturnValue("/students/321");
      const mockGet = vi.fn((key: string) =>
        key === "from" ? "/rooms?room=42" : null,
      );
      mockUseSearchParams.mockReturnValue(createMockSearchParams(mockGet));

      render(<Sidebar />);

      const roomsLink = screen.getByText("Räume").closest("a");
      expect(roomsLink).toHaveClass("bg-gray-100");
    });

    it("defaults to student search when no from param", () => {
      mockUsePathname.mockReturnValue("/students/100");
      mockUseSearchParams.mockReturnValue(createMockSearchParams(() => null));

      render(<Sidebar />);

      // Should default to Kindersuche when no from param
      const searchLink = screen.getByText("Kinder").closest("a");
      expect(searchLink).toHaveClass("bg-gray-100");
    });
  });

  describe("Statistik entry (#2606, formerly the Berichte placeholder)", () => {
    it("is a real navigation link for admins, without a coming-soon badge", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      const link = screen.getByText("Statistik").closest("a");
      expect(link).not.toBeNull();
      expect(link).toHaveAttribute("href", "/test-tenant/statistics");
      expect(screen.queryByText("Berichte")).not.toBeInTheDocument();
      expect(screen.queryByText("Bald")).not.toBeInTheDocument();
    });

    it("Zeiterfassung is an active navigation link", () => {
      render(<Sidebar />);

      const zeiterfassungElement = screen.getByText("Zeiterfassung");
      const link = zeiterfassungElement.closest("a");
      expect(link).not.toBeNull();
      expect(link).toHaveAttribute("href", "/test-tenant/time-tracking");
    });

    it("Nachrichten is an active navigation link", () => {
      render(<Sidebar />);

      const nachrichtenElement = screen.getByText("Nachrichten");
      const link = nachrichtenElement.closest("a");
      expect(link).not.toBeNull();
      // Eltern sub-page links are tenant-aware: in path-routing mode (the test
      // setup mocks tenantSlug="test-tenant", routingMode="path") the href is
      // prefixed with the tenant segment, matching the /eltern hub card links.
      expect(link).toHaveAttribute("href", "/test-tenant/messages");
    });

    it("does not show the old Dienstpläne placeholder for admins", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      expect(screen.queryByText("Dienstpläne")).not.toBeInTheDocument();
      expect(screen.getByText("Statistik")).toBeInTheDocument();
    });

    it("hides the Statistik entry unless staff have both required permissions", () => {
      mockIsAdmin.mockReturnValue(false);
      mockHasPermission.mockImplementation(
        (_session, permission) => permission === "config:read",
      );

      render(<Sidebar />);

      expect(screen.queryByText("Dienstpläne")).not.toBeInTheDocument();
      expect(screen.queryByText("Statistik")).not.toBeInTheDocument();
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

  describe("bottom pinned items", () => {
    it("renders settings at the bottom for admins", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      render(<Sidebar />);

      expect(screen.getByText("Einstellungen")).toBeInTheDocument();
    });

    it("marks the active entry without a colour of its own", () => {
      // Die Seitenleiste ist einfarbig: der aktive Eintrag wird über Fläche
      // und Schriftschnitt markiert (BAUARTEN-SPEC, Teil 2).
      mockUsePathname.mockReturnValue("/activities");

      render(<Sidebar />);

      const activitiesLink = screen.getByText("Aktivitäten").closest("a");
      expect(activitiesLink).toHaveClass("bg-gray-100");
      expect(activitiesLink).toHaveClass("font-semibold");
      const svg = activitiesLink?.querySelector("svg");
      expect(svg).not.toHaveAttribute("data-moto-duotone-tone");
    });
  });

  describe("operator mode navigation", () => {
    beforeEach(() => {
      mockUseShellAuth.mockReturnValue({
        user: {
          name: "Operator User",
          email: "op@example.com",
          roles: ["operator"],
        },
        profile: { firstName: "Operator", lastName: "User" },
        status: "authenticated",
        isSessionExpired: false,
        logout: vi.fn(),
        mode: "operator",
        homeUrl: "/operator/organizations",

        profileUrl: "/operator/settings",
      });
      mockUsePathname.mockReturnValue("/operator/organizations");
    });

    it("renders operator navigation items", () => {
      render(<Sidebar />);

      expect(screen.getByText("Ankündigungen")).toBeInTheDocument();
      expect(screen.queryByText("Einstellungen")).not.toBeInTheDocument();
    });

    it("does not render teacher-specific items", () => {
      render(<Sidebar />);

      expect(screen.queryByText("Kinder")).not.toBeInTheDocument();
      expect(screen.queryByText("Aktivitäten")).not.toBeInTheDocument();
      expect(screen.queryByText("Räume")).not.toBeInTheDocument();
      expect(screen.queryByText("Mitarbeiter")).not.toBeInTheDocument();
    });

    it("renders with custom className in operator mode", () => {
      const { container } = render(<Sidebar className="op-class" />);

      const aside = container.querySelector("aside");
      expect(aside).toHaveClass("op-class");
    });

    it("does not render settings in operator navigation", () => {
      render(<Sidebar />);

      expect(screen.queryByText("Einstellungen")).not.toBeInTheDocument();
    });
  });

  describe("binary presence mode", () => {
    // The sidebar hides room/activity nav + the supervision accordion when
    // the tenant runs in binary mode. These two assertions cover both
    // `isBinaryMode` branches (BINARY_HIDDEN_HREFS filter + accordion gate).

    beforeEach(() => {
      mockUsePresenceMode.mockReturnValue("binary");
    });

    afterEach(() => {
      mockUsePresenceMode.mockReturnValue("detailed");
    });

    it("hides Räume and Aktivitäten nav items", () => {
      render(<Sidebar />);
      expect(screen.queryByText("Räume")).not.toBeInTheDocument();
      expect(screen.queryByText("Aktivitäten")).not.toBeInTheDocument();
    });

    it("hides the Aktuelle-Aufsicht accordion for supervising staff", () => {
      // Give the user both groups and supervised rooms so detailed mode
      // would definitely render the accordion — then assert binary hides it.
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [
          { id: "r1", name: "Raum A", groupId: "g1", isSchulhof: false },
        ],
        groups: [{ id: 1, name: "1a" }],
        refresh: vi.fn(),
      });
      render(<Sidebar />);
      // Accordion header is "Aktuelle Aufsicht" (singular) or "Aktuelle
      // Aufsichten" (plural) — neither should appear in binary mode.
      expect(screen.queryByText(/Aktuelle Aufsicht/)).not.toBeInTheDocument();
    });

    it("keeps Kinder and Mitarbeitende visible (not binary-hidden)", () => {
      render(<Sidebar />);
      expect(screen.getByText("Kinder")).toBeInTheDocument();
      expect(screen.getByText("Mitarbeitende")).toBeInTheDocument();
    });
  });

  describe("NFC mode", () => {
    beforeEach(() => {
      mockUseNFCEnabled.mockReturnValue(false);
    });

    afterEach(() => {
      mockUseNFCEnabled.mockReturnValue(true);
    });

    it("hides the classic Aktivitäten nav item when NFC is disabled", () => {
      render(<Sidebar />);

      expect(screen.queryByText("Aktivitäten")).not.toBeInTheDocument();
    });

    it("hides NFC-only database sub-pages when NFC is disabled", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/database");

      render(<Sidebar />);

      // Die Register sind Reiter an ihrer Fläche; die Seitenleiste führt
      // weder „Datenverwaltung" noch „Geräte".
      expect(screen.queryByText("Datenverwaltung")).not.toBeInTheDocument();
      expect(screen.getByText("Kinder")).toBeInTheDocument();
      expect(screen.queryByText("Geräte")).not.toBeInTheDocument();
      expect(screen.queryByText("Aktivitäten")).not.toBeInTheDocument();
    });
  });

  describe("Planung accordion gating (#1946)", () => {
    beforeEach(() => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
    });

    afterEach(() => {
      // Restore the global SWR default (data: undefined) so the schema
      // override doesn't leak into other tests.
      mockUseSWRDefault.mockReturnValue({
        data: undefined,
        error: undefined,
        isLoading: true,
        isValidating: false,
        mutate: vi.fn(),
      } as unknown as ReturnType<typeof useSWR>);
    });

    it("keeps Zeiträume and Abrechnung when timetable.enabled is false", () => {
      mockUseSWRDefault.mockReturnValue({
        data: {
          tabs: [
            {
              categories: [
                { items: [{ key: "timetable.enabled", value: false }] },
              ],
            },
          ],
        },
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      } as unknown as ReturnType<typeof useSWR>);

      render(<Sidebar />);

      // Kalenderzeiträume bleiben erreichbar — die Anmeldephasen hängen
      // daran; nur die timetable-spezifischen Seiten verschwinden.
      expect(screen.getByText("Planung")).toBeInTheDocument();
      expect(screen.getByText("Zeiträume")).toBeInTheDocument();
      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Dienstplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Vertretung")).not.toBeInTheDocument();
      expect(screen.getByText("Abrechnung")).toBeInTheDocument();
    });

    it("reads the settings schema from the tenant-scoped SWR key", () => {
      render(<Sidebar />);

      expect(mockUseSWRDefault).toHaveBeenCalledWith(
        "test-tenant:settings-schema",
        expect.any(Function),
        expect.any(Object),
      );
    });

    it("shows the Planung accordion while the settings schema is loading", () => {
      // data undefined (globaler SWR-Default) — der Bereich darf beim Laden
      // nicht kurz verschwinden (`!== false`-Gate).
      render(<Sidebar />);

      expect(screen.getByText("Planung")).toBeInTheDocument();
    });

    it("navigates the header to Betreuungsplan when planning is enabled", () => {
      mockRouterPush.mockClear();
      mockUsePathname.mockReturnValue("/dashboard");

      render(<Sidebar />);
      fireEvent.click(screen.getByText("Planung"));

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/betreuungsplan",
      );
    });

    it("navigates the header to Zeiträume when timetable.enabled is false", () => {
      // Hub folgt der ersten sichtbaren Unterseite — sonst landete der
      // Header-Klick auf der "Betreuungsplan ist deaktiviert"-Hinweisseite.
      mockRouterPush.mockClear();
      mockUsePathname.mockReturnValue("/dashboard");
      mockUseSWRDefault.mockReturnValue({
        data: {
          tabs: [
            {
              categories: [
                { items: [{ key: "timetable.enabled", value: false }] },
              ],
            },
          ],
        },
        error: undefined,
        isLoading: false,
        isValidating: false,
        mutate: vi.fn(),
      } as unknown as ReturnType<typeof useSWR>);

      render(<Sidebar />);
      fireEvent.click(screen.getByText("Planung"));

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/calendar-periods",
      );
    });
  });

  // Abrechnung (#1417) war ein flacher Top-Level-Eintrag mit
  // config:manage-Gate und ist jetzt Unterpunkt des Planung-Akkordeons. Damit
  // gilt dort dasselbe Gate wie für den ganzen Bereich: nur Admins, und nur
  // solange timetable.enabled nicht ausgeschaltet ist.
  describe("Abrechnung im Planung-Akkordeon", () => {
    beforeEach(() => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
    });

    afterEach(() => {
      restoreDefaultHasPermission();
    });

    it("renders Abrechnung as an Auswertung sub-item, not as a flat entry", () => {
      render(<Sidebar />);

      const abrechnungLink = screen.getByText("Abrechnung").closest("a");
      expect(abrechnungLink).toHaveAttribute("href", "/test-tenant/payroll");

      // Der Eintrag steckt im Auswertungs-Akkordeon (gemeinsamer Container
      // mit dem Bereichs-Header), nicht als eigener Top-Level-Eintrag.
      const reportsSection = screen.getByText("Auswertung").closest("div");
      expect(reportsSection).toContainElement(abrechnungLink);

      // Unterpunkte tragen die Einrückung und kein eigenes Icon; flache
      // NAV_ITEMS rendern beides genau umgekehrt.
      expect(abrechnungLink).toHaveClass("pl-10");
      expect(abrechnungLink?.querySelector("svg")).toBeNull();
    });

    it("highlights Abrechnung on /payroll", () => {
      mockUsePathname.mockReturnValue("/payroll");

      render(<Sidebar />);

      expect(screen.getByText("Abrechnung").closest("a")).toHaveClass(
        "bg-gray-100",
      );
      expect(screen.getByText("Betreuungsplan").closest("a")).not.toHaveClass(
        "bg-gray-100",
      );
    });

    it("keeps Abrechnung visible for non-admins holding config:manage", () => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSession.mockReturnValue(createMockSession(false));
      mockHasPermission.mockImplementation(
        (_session, permission) => permission === "config:manage",
      );

      render(<Sidebar />);

      expect(screen.getByText("Auswertung")).toBeInTheDocument();
      expect(screen.getByText("Abrechnung").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/payroll",
      );
      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
    });

    // Leseansicht (#2283): Nicht-Admins erreichen den Betreuungsplan als Tab
    // in "Mein Kalender" — die Sidebar zeigt ihnen KEINEN eigenen Eintrag,
    // auch nicht mit schedules:read.
    it("shows no Betreuungsplan entry for non-admins with schedules:read", () => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSession.mockReturnValue(createMockSession(false));
      mockHasPermission.mockImplementation(
        (_session, permission) =>
          permission === "schedules:read" || permission === "calendar:own",
      );

      render(<Sidebar />);

      // Der Kalender ist der einzige Planungseintrag, den Nicht-Admins
      // sehen (calendar:own).
      expect(screen.getByText("Planung")).toBeInTheDocument();
      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
      expect(screen.getByText("Kalender")).toBeInTheDocument();
    });

    it("hides Betreuungsplan for non-admins without schedules:read", () => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSession.mockReturnValue(createMockSession(false));
      mockHasPermission.mockReturnValue(false);

      render(<Sidebar />);

      expect(screen.queryByText("Planung")).not.toBeInTheDocument();
      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
    });
  });
});
