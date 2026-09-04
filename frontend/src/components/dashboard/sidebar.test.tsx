import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  expectIdleRenderBudget,
  RENDER_BUDGET_MAX_COMMITS,
} from "~/test/render-budget";
import {
  act,
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";

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
  const hasEffectiveAdminScopeFn = vi.fn(() => isAdminFn());
  return {
    isAdmin: isAdminFn,
    hasEffectiveAdminScope: hasEffectiveAdminScopeFn,
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

vi.mock("~/lib/hooks/use-change-request-access", () => ({
  useChangeRequestAccess: vi.fn(),
}));

// Tagesinformationen-Badge (#2180): der echte Hook würde /api/staff-notices/today
// laden; hier zählt nur, dass die Seitenleiste ihn einbindet.
vi.mock("~/lib/hooks/use-staff-notices-pending", () => ({
  useStaffNoticesPending: () => ({
    unreadCount: 0,
    isLoading: false,
    refresh: vi.fn(),
  }),
  STAFF_NOTICES_REFRESH_EVENT: "staff-notices-refresh",
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
import {
  hasEffectiveAdminScope,
  hasPermission,
  isAdmin,
} from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useStaffAbsencesPending } from "~/lib/hooks/use-staff-absences-pending";
import { useChangeRequestsPending } from "~/lib/hooks/use-change-requests-pending";
import { useChangeRequestAccess } from "~/lib/hooks/use-change-request-access";
import { useCareWithdrawalsPending } from "~/lib/hooks/use-care-withdrawals-pending";
import { useMessagesUnread } from "~/lib/hooks/use-messages-unread";
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
const mockHasEffectiveAdminScope = vi.mocked(hasEffectiveAdminScope);
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
const mockUseChangeRequestAccess = vi.mocked(useChangeRequestAccess);
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
    mockHasEffectiveAdminScope.mockImplementation((session) =>
      mockIsAdmin(session),
    );
    restoreDefaultHasPermission();
    vi.mocked(useMessagesUnread).mockReturnValue({
      unreadCount: 0,
      isLoading: false,
      refresh: vi.fn(),
    });
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
    mockUseChangeRequestAccess.mockReturnValue({
      canOpenRequestsPage: false,
    } as ReturnType<typeof useChangeRequestAccess>);
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
      expect(screen.getByText("Mitarbeiter")).toBeInTheDocument();
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
      expect(screen.getByText("Home")).toBeInTheDocument();
      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
      expect(screen.queryByText("Übergaben")).not.toBeInTheDocument();
      expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
      // Die Planungsbereiche sind Unterpunkte des Planung-Akkordeons (#1946),
      // inklusive Schuljahr und Ferien.
      expect(screen.getByText("Planung")).toBeInTheDocument();
      expect(screen.getByText("Betreuungsplan")).toBeInTheDocument();
      expect(screen.getByText("Dienstplan")).toBeInTheDocument();
      expect(screen.getByText("Vertretungsplan")).toBeInTheDocument();
      expect(screen.getByText("Schuljahr und Ferien")).toBeInTheDocument();
    });

    it("labels the personal calendar entry 'Mein Kalender'", () => {
      // Der Eintrag trägt jetzt denselben Namen wie die H1 der Seite; das
      // alte, unspezifische "Kalender" darf nicht mehr auftauchen.
      render(<Sidebar />);

      expect(screen.getByText("Mein Kalender").closest("a")).toHaveAttribute(
        "href",
        "/calendar",
      );
      expect(screen.queryByText("Kalender")).not.toBeInTheDocument();
    });

    it("shows group navigation but hides room supervision for admins", () => {
      render(<Sidebar />);

      expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
      expect(screen.queryByText("Aktuelle Aufsicht")).not.toBeInTheDocument();
    });

    it("puts all admin-visible groups under Weitere Gruppen", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen", is_personal: false },
          { id: "2", name: "Adler", is_personal: false },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
      expect(screen.getByText("Weitere Gruppen")).toBeInTheDocument();
    });

    it("shows group navigation for an effective admin", () => {
      mockHasEffectiveAdminScope.mockReturnValue(true);

      render(<Sidebar />);

      expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
    });

    it("shows all children with the children concept icon for admins", () => {
      mockUsePathname.mockReturnValue("/students/search");
      render(<Sidebar />);

      const link = screen.getByText("Alle Kinder").closest("a");
      expect(link).toBeInTheDocument();
      expect(link?.querySelector("svg")).toHaveAttribute(
        "data-moto-duotone-tone",
        "greenVivid",
      );
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
      expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
      expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
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
      mockUseChangeRequestAccess.mockReturnValue({
        canOpenRequestsPage: true,
      } as ReturnType<typeof useChangeRequestAccess>);

      render(<Sidebar />);

      expect(screen.getByText("Anfragen").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/anfragen",
      );
      expect(screen.getByLabelText("9 offene Anfragen")).toBeInTheDocument();
    });

    it("hides Anfragen without a current effective review scope", () => {
      mockHasPermission.mockImplementation(
        (_session, permission) => permission === "users:update",
      );
      mockUseChangeRequestsPending.mockReturnValue({
        unreadCount: 2,
        isLoading: false,
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.queryByText("Anfragen")).not.toBeInTheDocument();
      expect(
        screen.queryByLabelText("2 offene Anfragen"),
      ).not.toBeInTheDocument();
    });

    it("prefixes the Team-Chat link in path-routing mode", () => {
      mockUseStaffMessagingEnabled.mockReturnValue(true);

      render(<Sidebar />);

      expect(screen.getByText("Team-Chat").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/team-chat",
      );
    });

    it("keeps the Team group without its internal pages when none is accessible", () => {
      mockHasPermission.mockReturnValue(false);

      render(<Sidebar />);

      // Team-Chat ist aus, Tagesinformationen brauchen users:read — die
      // Gruppe bleibt wegen Zeiterfassung und Mitarbeiter trotzdem da.
      expect(screen.queryByText("Team-Chat")).not.toBeInTheDocument();
      expect(screen.queryByText("Tagesinformationen")).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Team" })).toBeInTheDocument();
      expect(screen.getByText("Zeiterfassung")).toBeInTheDocument();
    });

    it("hides admin-only items for staff", () => {
      render(<Sidebar />);

      // Admin-only items should NOT be visible
      expect(screen.queryByText("Übergaben")).not.toBeInTheDocument();
      expect(screen.queryByText("Datenverwaltung")).not.toBeInTheDocument();
      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Dienstplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Vertretungsplan")).not.toBeInTheDocument();
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

      expect(screen.getByText("Alle Kinder")).toBeInTheDocument();
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

      expect(screen.getByText("Alle Kinder")).toBeInTheDocument();
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
      expect(screen.getByText("Alle Kinder")).toBeInTheDocument();
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
      const staffLink = screen.getByText("Mitarbeiter").closest("a");
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

    it("highlights Schuljahr und Ferien on /calendar-periods", () => {
      // Die Zeitraum-Verwaltung ist eigener Unterpunkt im Planung-Akkordeon
      // (#1946) und leuchtet dort selbst, nicht mehr im Betreuungsplan.
      mockUsePathname.mockReturnValue("/calendar-periods");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      const periodsLink = screen.getByText("Schuljahr und Ferien").closest("a");
      const betreuungsplanLink = screen
        .getByText("Betreuungsplan")
        .closest("a");
      // Der /calendar-Eintrag heißt "Mein Kalender" (wie die H1 der Seite).
      const kalenderLink = screen.getByText("Mein Kalender").closest("a");
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

      const dashboardLink = screen.getByText("Home").closest("a");
      expect(dashboardLink).not.toHaveClass("bg-gray-100");
    });

    it("does not render the removed enrollment reports subpage", () => {
      mockUsePathname.mockReturnValue("/admin/enrollments");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<Sidebar />);

      const overviewLink = screen.getByText("Überblick").closest("a");
      expect(overviewLink).toHaveClass("bg-gray-100");
      expect(screen.queryByText("Auswertung")).not.toBeInTheDocument();
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

      const groupHeader = screen.getByText("Meine Gruppen").closest("button");
      expect(groupHeader).toHaveClass("bg-gray-100");
    });

    it("highlights active-supervisions when coming from supervisions page", () => {
      mockUsePathname.mockReturnValue("/students/456");
      const mockGet = vi.fn((key: string) =>
        key === "from" ? "/active-supervisions" : null,
      );
      mockUseSearchParams.mockReturnValue(createMockSearchParams(mockGet));

      render(<Sidebar />);

      const supervisionHeader = screen
        .getByText("Aktuelle Aufsicht")
        .closest("button");
      expect(supervisionHeader).toHaveClass("bg-gray-100");
    });

    it("highlights student search when coming from search page", () => {
      mockUsePathname.mockReturnValue("/students/789");
      const mockGet = vi.fn().mockReturnValue("/students/search");
      mockUseSearchParams.mockReturnValue(createMockSearchParams(mockGet));

      render(<Sidebar />);

      const searchLink = screen.getByText("Alle Kinder").closest("a");
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
      const searchLink = screen.getByText("Alle Kinder").closest("a");
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
      expect(link).toHaveAttribute("href", "/statistics");
      expect(screen.queryByText("Berichte")).not.toBeInTheDocument();
      expect(screen.queryByText("Bald")).not.toBeInTheDocument();
    });

    it("Zeiterfassung is an active navigation link", () => {
      render(<Sidebar />);

      const zeiterfassungElement = screen.getByText("Zeiterfassung");
      const link = zeiterfassungElement.closest("a");
      expect(link).not.toBeNull();
      expect(link).toHaveAttribute("href", "/time-tracking");
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

  describe("supervision loading states", () => {
    it("handles loading groups state correctly", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: false,
        isLoadingGroups: true,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      // Should still render, but supervision-dependent items behavior changes
      expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
    });

    it("handles loading supervision state correctly", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: true,
        overviewEnabled: false,
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
    it("opens personal groups and keeps additional groups closed by default", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen", is_personal: true },
          { id: "2", name: "Adler", is_personal: false },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(
        screen.getByText("Meine Gruppen").closest("button"),
      ).toHaveAttribute("aria-expanded", "true");
      expect(
        screen.getByText("Weitere Gruppen").closest("button"),
      ).toHaveAttribute("aria-expanded", "false");
    });

    it("keeps personal and additional groups mutually exclusive", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen", is_personal: true },
          { id: "2", name: "Adler", is_personal: false },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);
      fireEvent.click(screen.getByText("Weitere Gruppen"));

      expect(
        screen.getByText("Meine Gruppen").closest("button"),
      ).toHaveAttribute("aria-expanded", "false");
      expect(
        screen.getByText("Weitere Gruppen").closest("button"),
      ).toHaveAttribute("aria-expanded", "true");
    });

    it("closes additional groups after leaving the groups section", async () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen", is_personal: true },
          { id: "2", name: "Adler", is_personal: false },
        ],
        refresh: vi.fn(),
      });

      const { rerender } = render(<Sidebar />);
      fireEvent.click(screen.getByText("Weitere Gruppen"));
      mockUsePathname.mockReturnValue("/activities");
      rerender(<Sidebar />);
      await waitFor(() =>
        expect(
          screen.getByText("Weitere Gruppen").closest("button"),
        ).toHaveAttribute("aria-expanded", "false"),
      );

      mockUsePathname.mockReturnValue("/ogs-groups");
      rerender(<Sidebar />);
      expect(
        screen.getByText("Meine Gruppen").closest("button"),
      ).toHaveAttribute("aria-expanded", "true");
    });

    it("opens additional groups when the current group is selected", () => {
      mockUsePathname.mockReturnValue("/ogs-groups");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams((key: string) => (key === "group" ? "2" : null)),
      );
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen", is_personal: true },
          { id: "2", name: "Adler", is_personal: false },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(
        screen.getByText("Weitere Gruppen").closest("button"),
      ).toHaveAttribute("aria-expanded", "true");
    });

    it("hides additional groups when the backend returns only personal groups", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [{ id: "1", name: "Eulen", is_personal: true }],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
      expect(screen.queryByText("Weitere Gruppen")).not.toBeInTheDocument();
    });

    it("renders group sub-items when groups are available", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen" },
          { id: "2", name: "Adler" },
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
        overviewEnabled: false,
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
      expect(screen.getByText("Kinderdaten")).toBeInTheDocument();
      expect(screen.getByText("Personal")).toBeInTheDocument();
      expect(screen.getByText("Gruppen")).toBeInTheDocument();
    });
  });

  describe("accordion toggle navigation", () => {
    beforeEach(() => {
      mockRouterPush.mockClear();
      // Mock localStorage
      vi.spyOn(localStorage, "getItem").mockReturnValue(null);
      vi.spyOn(localStorage, "setItem").mockImplementation(() => {
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
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [{ id: "1", name: "Eulen" }],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const groupHeader = screen.getByText("Meine Gruppen");
      fireEvent.click(groupHeader);

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/ogs-groups?group=1",
      );
    });

    it("does not navigate from an empty personal groups section", () => {
      mockUsePathname.mockReturnValue("/activities");
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

      const groupHeader = screen.getByText("Meine Gruppen");
      fireEvent.click(groupHeader);

      expect(mockRouterPush).not.toHaveBeenCalled();
    });

    it("navigates to active-supervisions when supervisions toggle clicked from another page", () => {
      mockUsePathname.mockReturnValue("/activities");
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [{ id: "10", name: "Raum A", groupId: "1" }],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const supervisionHeader = screen.getByText("Aktuelle Aufsicht");
      fireEvent.click(supervisionHeader);

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?session=1",
      );
    });

    it("navigates to active-supervisions without room param when no rooms", () => {
      mockUsePathname.mockReturnValue("/activities");
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

      const supervisionHeader = screen.getByText("Aktuelle Aufsicht");
      fireEvent.click(supervisionHeader);

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions",
      );
    });

    it("navigates to database hub when database toggle clicked from another page", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/activities");

      render(<Sidebar />);

      const databaseHeader = screen.getByText("Datenverwaltung");
      fireEvent.click(databaseHeader);

      expect(mockRouterPush).toHaveBeenCalledWith("/test-tenant/database");
    });

    it("navigates back to database hub when on a database sub-page", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/database/students");

      render(<Sidebar />);

      const databaseHeader = screen.getByText("Datenverwaltung");
      fireEvent.click(databaseHeader);

      expect(mockRouterPush).toHaveBeenCalledWith("/test-tenant/database");
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
    it("renders settings at the bottom for admins", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      render(<Sidebar />);

      expect(screen.getByText("Einstellungen")).toBeInTheDocument();
    });

    it("highlights active icon color for active links", () => {
      mockUsePathname.mockReturnValue("/activities");

      render(<Sidebar />);

      const activitiesLink = screen.getByText("Aktivitäten").closest("a");
      const svg = activitiesLink?.querySelector("svg");
      expect(svg).toHaveAttribute("data-moto-duotone-tone", "coral");
      expect(svg).toHaveStyle({ color: "#A83A2E" });
    });
  });

  describe("groups label pluralization", () => {
    it("shows 'Meine Gruppen' for a single group", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [{ id: "1", name: "Eulen" }],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
    });

    it("shows 'Meine Gruppen' for multiple groups", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen" },
          { id: "2", name: "Adler" },
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
        overviewEnabled: false,
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
        overviewEnabled: false,
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
      vi.spyOn(localStorage, "getItem").mockImplementation((key: string) => {
        if (key === "sidebar-last-group") return "1";
        return null;
      });
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
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen" },
          { id: "2", name: "Adler" },
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
        overviewEnabled: false,
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

    it("highlights room sub-item on student detail from active-supervisions", () => {
      vi.spyOn(localStorage, "getItem").mockImplementation((key: string) => {
        if (key === "sidebar-last-room") return "10";
        return null;
      });
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
        overviewEnabled: false,
        supervisedRooms: [
          { id: "10", name: "Raum A", groupId: "1" },
          { id: "20", name: "Raum B", groupId: "2" },
        ],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const raumALink = screen.getByText("Raum A").closest("a");
      expect(raumALink).toHaveClass("bg-gray-100");
    });
  });

  describe("localStorage persistence", () => {
    const mockSetItem = vi.fn();

    beforeEach(() => {
      mockSetItem.mockClear();
      vi.spyOn(localStorage, "setItem").mockImplementation(mockSetItem);
      vi.spyOn(localStorage, "getItem").mockReturnValue(null);
      vi.spyOn(localStorage, "removeItem").mockImplementation(() => undefined);
    });

    it("persists selected group and group name to localStorage", () => {
      mockUsePathname.mockReturnValue("/ogs-groups");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams((key: string) => (key === "group" ? "2" : null)),
      );
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen" },
          { id: "2", name: "Adler" },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(mockSetItem).toHaveBeenCalledWith("sidebar-last-group", "2");
      expect(mockSetItem).toHaveBeenCalledWith(
        "sidebar-last-group-name",
        "Adler",
      );
    });

    it("persists selected room and room name to localStorage", () => {
      mockUsePathname.mockReturnValue("/active-supervisions");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams((key: string) => (key === "room" ? "20" : null)),
      );
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [
          { id: "10", name: "Raum A", groupId: "1" },
          { id: "20", name: "Raum B", groupId: "2" },
        ],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(mockSetItem).toHaveBeenCalledWith("sidebar-last-room", "20");
      expect(mockSetItem).toHaveBeenCalledWith(
        "sidebar-last-room-name",
        "Raum B",
      );
    });

    it("persists current database sub-page to localStorage", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/database/students");

      render(<Sidebar />);

      expect(mockSetItem).toHaveBeenCalledWith(
        "sidebar-last-database",
        "/database/students",
      );
    });

    it("does not persist group name when group is not found", () => {
      mockUsePathname.mockReturnValue("/ogs-groups");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams((key: string) =>
          key === "group" ? "999" : null,
        ),
      );
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [{ id: "1", name: "Eulen" }],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(mockSetItem).toHaveBeenCalledWith("sidebar-last-group", "999");
      expect(mockSetItem).not.toHaveBeenCalledWith(
        "sidebar-last-group-name",
        expect.any(String),
      );
    });

    it("does not persist room name when room is not found", () => {
      mockUsePathname.mockReturnValue("/active-supervisions");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams((key: string) =>
          key === "room" ? "999" : null,
        ),
      );
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [{ id: "10", name: "Raum A", groupId: "1" }],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      expect(mockSetItem).toHaveBeenCalledWith("sidebar-last-room", "999");
      expect(mockSetItem).not.toHaveBeenCalledWith(
        "sidebar-last-room-name",
        expect.any(String),
      );
    });
  });

  describe("accordion toggle with saved localStorage", () => {
    beforeEach(() => {
      mockRouterPush.mockClear();
      vi.spyOn(localStorage, "setItem").mockImplementation(() => undefined);
      vi.spyOn(localStorage, "removeItem").mockImplementation(() => undefined);
    });

    it("navigates to saved group from localStorage when toggling groups", () => {
      const mockGetItem = vi.fn((key: string) => {
        if (key === "sidebar-last-group") return "2";
        return null;
      });
      vi.spyOn(localStorage, "getItem").mockImplementation(mockGetItem);
      mockUsePathname.mockReturnValue("/activities");
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen" },
          { id: "2", name: "Adler" },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const groupHeader = screen.getByText("Meine Gruppen");
      fireEvent.click(groupHeader);

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/ogs-groups?group=2",
      );
    });

    it("navigates to saved room from localStorage when toggling supervisions", () => {
      const mockGetItem = vi.fn((key: string) => {
        if (key === "sidebar-last-room") return "20";
        return null;
      });
      vi.spyOn(localStorage, "getItem").mockImplementation(mockGetItem);
      mockUsePathname.mockReturnValue("/activities");
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [
          { id: "10", name: "Raum A", groupId: "1" },
          { id: "20", name: "Raum B", groupId: "2" },
        ],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const supervisionHeader = screen.getByText("Aktuelle Aufsichten");
      fireEvent.click(supervisionHeader);

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?session=2",
      );
    });

    it("falls back to first group when saved group not found", () => {
      const mockGetItem = vi.fn((key: string) => {
        if (key === "sidebar-last-group") return "999";
        return null;
      });
      vi.spyOn(localStorage, "getItem").mockImplementation(mockGetItem);
      mockUsePathname.mockReturnValue("/activities");
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen" },
          { id: "2", name: "Adler" },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const groupHeader = screen.getByText("Meine Gruppen");
      fireEvent.click(groupHeader);

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/ogs-groups?group=1",
      );
    });

    it("falls back to first room when saved room not found", () => {
      const mockGetItem = vi.fn((key: string) => {
        if (key === "sidebar-last-room") return "999";
        return null;
      });
      vi.spyOn(localStorage, "getItem").mockImplementation(mockGetItem);
      mockUsePathname.mockReturnValue("/activities");
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [
          { id: "10", name: "Raum A", groupId: "1" },
          { id: "20", name: "Raum B", groupId: "2" },
        ],
        groups: [],
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      const supervisionHeader = screen.getByText("Aktuelle Aufsichten");
      fireEvent.click(supervisionHeader);

      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?session=1",
      );
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

      expect(screen.queryByText("Alle Kinder")).not.toBeInTheDocument();
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

  describe("Schulhof room handling", () => {
    it("renders Schulhof room with special styling", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [
          { id: "10", name: "Raum A", groupId: "1" },
          {
            id: "schulhof",
            name: "Schulhof",
            groupId: "schulhof",
            isSchulhof: true,
          },
        ],
        groups: [],
        refresh: vi.fn(),
      });
      mockUsePathname.mockReturnValue("/active-supervisions");

      render(<Sidebar />);

      expect(screen.getByText("Schulhof")).toBeInTheDocument();
      expect(screen.getByText("Raum A")).toBeInTheDocument();
    });

    it("navigates to schulhof param when Schulhof room clicked", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [
          {
            id: "schulhof",
            name: "Schulhof",
            groupId: "schulhof",
            isSchulhof: true,
          },
        ],
        groups: [],
        refresh: vi.fn(),
      });
      mockUsePathname.mockReturnValue("/active-supervisions");

      render(<Sidebar />);

      const schulhofLink = screen.getByText("Schulhof").closest("a");
      expect(schulhofLink).toHaveAttribute(
        "href",
        "/active-supervisions?session=schulhof",
      );
    });

    it("uses schulhof string in navigation for Schulhof rooms", () => {
      // Test the condition: room.isSchulhof ? "schulhof" : room.id
      const room = { id: "schulhof", name: "Schulhof", isSchulhof: true };
      const navParam = room.isSchulhof ? "schulhof" : room.id;

      expect(navParam).toBe("schulhof");
    });

    it("uses room id in navigation for regular rooms", () => {
      const room = { id: "10", name: "Raum A", isSchulhof: false };
      const navParam = room.isSchulhof ? "schulhof" : room.id;

      expect(navParam).toBe("10");
    });

    it("generates correct href for Schulhof room", () => {
      const room = { id: "schulhof", name: "Schulhof", isSchulhof: true };
      const basePath = "/active-supervisions";

      const href = room.isSchulhof
        ? `${basePath}?room=schulhof`
        : `${basePath}?room=${room.id}`;

      expect(href).toBe("/active-supervisions?room=schulhof");
    });

    it("generates correct href for regular room", () => {
      const room = { id: "20", name: "Raum B", isSchulhof: false };
      const basePath = "/active-supervisions";

      const href = room.isSchulhof
        ? `${basePath}?room=schulhof`
        : `${basePath}?room=${room.id}`;

      expect(href).toBe("/active-supervisions?room=20");
    });

    it("includes Schulhof in supervised rooms list", () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [
          { id: "10", name: "Raum A", groupId: "1" },
          {
            id: "schulhof",
            name: "Schulhof",
            groupId: "schulhof",
            isSchulhof: true,
          },
        ],
        groups: [],
        refresh: vi.fn(),
      });
      mockUsePathname.mockReturnValue("/active-supervisions");

      render(<Sidebar />);

      // Both rooms should be visible
      const links = screen.getAllByRole("link");
      const roomLinks = links.filter(
        (link) =>
          link.textContent === "Raum A" || link.textContent === "Schulhof",
      );
      expect(roomLinks).toHaveLength(2);
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
        groups: [{ id: "1", name: "1a" }],
        refresh: vi.fn(),
      });
      render(<Sidebar />);
      // Accordion header is "Aktuelle Aufsicht" (singular) or "Aktuelle
      // Aufsichten" (plural) — neither should appear in binary mode.
      expect(screen.queryByText(/Aktuelle Aufsicht/)).not.toBeInTheDocument();
    });

    it("keeps Kindersuche and Mitarbeiter visible (not binary-hidden)", () => {
      render(<Sidebar />);
      expect(screen.getByText("Alle Kinder")).toBeInTheDocument();
      expect(screen.getByText("Mitarbeiter")).toBeInTheDocument();
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

      expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
      expect(screen.getByText("Kinderdaten")).toBeInTheDocument();
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

    it("keeps Schuljahr und Ferien and Abrechnung when timetable.enabled is false", () => {
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

      // Schuljahr und Ferien bleiben erreichbar — die Anmeldephasen hängen
      // daran; nur die timetable-spezifischen Seiten verschwinden.
      expect(screen.getByText("Planung")).toBeInTheDocument();
      expect(screen.getByText("Schuljahr und Ferien")).toBeInTheDocument();
      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Dienstplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Vertretungsplan")).not.toBeInTheDocument();
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

    it("only toggles the Planung group on a header click, without navigating (#2826)", () => {
      // Die Gruppenzeile ist ein Schalter, keine Seite: das frühere
      // Navigate-on-expand des Planung-Akkordeons entfällt.
      mockRouterPush.mockClear();
      mockUsePathname.mockReturnValue("/dashboard");

      render(<Sidebar />);
      const header = screen.getByRole("button", { name: "Planung" });
      expect(header).toHaveAttribute("aria-expanded", "false");

      fireEvent.click(header);

      expect(header).toHaveAttribute("aria-expanded", "true");
      expect(mockRouterPush).not.toHaveBeenCalled();
    });

    it("opens the Planung group by itself on a planning page", () => {
      mockUsePathname.mockReturnValue("/dienstplan");

      render(<Sidebar />);

      expect(screen.getByRole("button", { name: "Planung" })).toHaveAttribute(
        "aria-expanded",
        "true",
      );
      // Der Tagesbetrieb bleibt daneben offen — ein Seitenwechsel schließt
      // nichts.
      expect(
        screen.getByRole("button", { name: "Tagesbetrieb" }),
      ).toHaveAttribute("aria-expanded", "true");
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

    it("renders Abrechnung as a row of the Planung group, not as a top-level entry", () => {
      render(<Sidebar />);

      const abrechnungLink = screen.getByText("Abrechnung").closest("a");
      expect(abrechnungLink).toHaveAttribute("href", "/test-tenant/payroll");

      // Der Eintrag steckt in der Gruppe Planung (gemeinsamer Container mit
      // der Gruppenzeile), nicht als eigener Eintrag daneben (#2826).
      const planningGroup = screen
        .getByRole("button", { name: "Planung" })
        .closest("div");
      expect(planningGroup).toContainElement(abrechnungLink);

      // Zeilen einer Gruppe tragen ein Icon und stehen im Zeilenraster;
      // nur die Unterpunkte der Akkordeons sind eingerückt.
      expect(abrechnungLink).not.toHaveClass("pl-11");
      expect(abrechnungLink?.querySelector("svg")).not.toBeNull();
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

      expect(screen.getByText("Planung")).toBeInTheDocument();
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

      expect(screen.queryByText("Planung")).not.toBeInTheDocument();
      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
      expect(screen.getByText("Mein Kalender")).toBeInTheDocument();
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

  describe("Vertretungen navigation (#2806)", () => {
    beforeEach(() => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
    });

    afterEach(() => {
      mockUseOpenCareGroupMode.mockReturnValue(false);
    });

    it("keeps Vertretungen for open-care tenants", () => {
      mockUseOpenCareGroupMode.mockReturnValue(true);

      render(<Sidebar />);

      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
    });

    it("shows Vertretungen for fixed-groups tenants", () => {
      render(<Sidebar />);

      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
    });

    it("shows Vertretungen to staff", () => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSession.mockReturnValue(createMockSession(false));

      render(<Sidebar />);

      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
    });
  });

  describe("Meine Gruppen gating (#1544)", () => {
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

    afterEach(() => {
      mockUseOpenCareGroupMode.mockReturnValue(false);
    });

    it("hides the Meine Gruppen accordion for open-care tenants", () => {
      mockUseOpenCareGroupMode.mockReturnValue(true);

      render(<Sidebar />);

      expect(screen.queryByText("Meine Gruppen")).not.toBeInTheDocument();
      expect(
        screen.queryByText("Keine Gruppen zugeordnet"),
      ).not.toBeInTheDocument();
      // Aufsicht und Kindersuche bleiben als Staff-Einstiege erhalten.
      expect(screen.getByText("Aktuelle Aufsicht")).toBeInTheDocument();
      expect(screen.getByText("Alle Kinder")).toBeInTheDocument();
    });

    it("shows the Meine Gruppen accordion for fixed-groups tenants", () => {
      render(<Sidebar />);

      expect(screen.getByText("Meine Gruppen")).toBeInTheDocument();
    });
  });

  describe("collapsible sidebar (#2825)", () => {
    // Umgeschaltet wird über den Toggle-Button in der Kopfzeile
    // (header.test.tsx); die Seitenleiste selbst folgt nur dem geteilten
    // useSidebarCollapsed-Store.
    it("renders expanded by default on wide viewports", () => {
      const { container } = render(<Sidebar />);

      expect(container.querySelector("aside")).toHaveClass("w-64");
      expect(screen.getByText("Aktivitäten")).toBeInTheDocument();
    });

    it("renders the icon rail when the stored state is collapsed", () => {
      localStorage.setItem("sidebar-collapsed", "true");

      const { container } = render(<Sidebar />);

      expect(container.querySelector("aside")).toHaveClass("w-16");
      // Labels verschwinden; die Ziele bleiben als beschriftete Icons da.
      expect(screen.queryByText("Aktivitäten")).not.toBeInTheDocument();
      expect(screen.getByLabelText("Aktivitäten")).toBeInTheDocument();
      expect(screen.getByLabelText("Räume")).toBeInTheDocument();
    });

    it("follows the header toggle via the shared store while mounted", () => {
      const { container } = render(<Sidebar />);
      expect(container.querySelector("aside")).toHaveClass("w-64");

      // Simuliert den Klick auf den Kopfzeilen-Toggle: derselbe Schreibpfad
      // (localStorage + Custom-Event), den useSidebarCollapsed nutzt.
      act(() => {
        localStorage.setItem("sidebar-collapsed", "true");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });

      expect(container.querySelector("aside")).toHaveClass("w-16");
    });

    it("expands the sidebar and opens the section when a rail accordion icon is clicked", () => {
      // Betreuungskraft mit Aufsicht: der Aufsicht-Bereich ist im Streifen
      // zu (nur "Meine Gruppen" steht standardmäßig offen).
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [
          { id: "7", name: "Raum 1", groupId: "g1", isSchulhof: false },
        ],
        groups: [],
        refresh: vi.fn(),
      });
      localStorage.setItem("sidebar-collapsed", "true");
      const { container } = render(<Sidebar />);

      fireEvent.click(
        screen.getByRole("button", { name: "Aktuelle Aufsicht" }),
      );

      // Aufklappen + Navigate-on-expand wie im ausgeklappten Zustand.
      expect(container.querySelector("aside")).toHaveClass("w-64");
      expect(mockRouterPush).toHaveBeenCalledWith(
        "/test-tenant/active-supervisions?session=g1",
      );
      expect(localStorage.getItem("sidebar-collapsed")).toBe("false");
    });

    it("toggles a group in the rail without expanding the sidebar (#2826)", () => {
      // Die Zeilen einer Gruppe haben Icons und passen in den Streifen; die
      // Gruppenzeile klappt dort nur, statt die Leiste zu öffnen.
      localStorage.setItem("sidebar-collapsed", "true");
      const { container } = render(<Sidebar />);

      const team = screen.getByRole("button", { name: "Team" });
      expect(team).toHaveAttribute("aria-expanded", "false");
      fireEvent.click(team);

      expect(team).toHaveAttribute("aria-expanded", "true");
      expect(container.querySelector("aside")).toHaveClass("w-16");
      expect(screen.getByLabelText("Zeiterfassung")).toBeInTheDocument();
    });

    it("keeps the bottom-pinned items reachable as icons in the rail", () => {
      localStorage.setItem("sidebar-collapsed", "true");
      render(<Sidebar />);

      expect(screen.getByLabelText("Notfall")).toBeInTheDocument();
      expect(screen.getByLabelText("Hilfe")).toBeInTheDocument();
    });

    it("keeps Tagesplan reachable as an icon in the rail (#2383)", () => {
      // Betreuungskraft mit schedules:read: ausgeklappt steht Tagesplan ganz
      // oben — der Streifen darf den Einstieg nicht verlieren.
      mockHasPermission.mockImplementation(
        (_session, permission) => permission === "schedules:read",
      );
      localStorage.setItem("sidebar-collapsed", "true");

      render(<Sidebar />);

      expect(screen.getByLabelText("Tagesplan")).toBeInTheDocument();
    });
  });

  describe("ruhiges Klappen (#2923)", () => {
    // Ein Raster, ein Baum: eingeklappt und ausgeklappt sind dieselben
    // Zeilen, damit beim Umschalten nichts ein zweites Mal springt.
    const rowOf = (labelOrText: string) =>
      screen.getByRole("link", { name: labelOrText });

    it("gibt Zeilen in beiden Zuständen dasselbe Raster", () => {
      const { rerender } = render(<Sidebar />);
      const expandedClasses = rowOf("Aktivitäten").className;

      act(() => {
        localStorage.setItem("sidebar-collapsed", "true");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });
      rerender(<Sidebar />);

      // Höhe, Innenabstand und Rundung sind identisch — nur die Breite der
      // Leiste ändert sich.
      for (const token of ["h-10", "px-3", "rounded-lg"]) {
        expect(expandedClasses).toContain(token);
        expect(rowOf("Aktivitäten").className).toContain(token);
      }
    });

    it("gibt Bereichs-Schaltern dasselbe Raster wie den Links", () => {
      render(<Sidebar />);

      // Die Kopfzeile eines Bereichs ist ein Kit-Button, trägt aber das
      // Zeilenraster der Seitenleiste. Die Grundklassen des Buttons dürfen
      // dabei nicht durchschlagen: keine zentrierte Ausrichtung, keine
      // eigene Innenbreite, kein transparenter Grund über dem Aktiv-Zustand.
      const header = screen.getByRole("button", { name: "Meine Gruppen" });
      for (const token of ["h-10", "px-3", "rounded-lg", "justify-start"]) {
        expect(header.className).toContain(token);
      }
      for (const token of ["px-4", "justify-center"]) {
        expect(header.className).not.toContain(token);
      }

      // Die Gruppenzeile (#2826) teilt Innenabstand und Icon-Spalte, ist
      // aber flacher — sie ist ein Schalter, keine Seite.
      const group = screen.getByRole("button", { name: "Team" });
      for (const token of ["h-8", "px-3", "rounded-lg", "justify-start"]) {
        expect(group.className).toContain(token);
      }
      for (const token of ["px-4", "justify-center"]) {
        expect(group.className).not.toContain(token);
      }
    });

    it("hält den Aktiv-Zustand über den Grundklassen des Buttons", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/database");

      render(<Sidebar />);

      // Der graue Grund des aktiven Bereichs darf nicht vom bg-transparent
      // der Ghost-Variante überschrieben werden.
      const header = screen.getByRole("button", { name: "Datenverwaltung" });
      expect(header.className).toContain("bg-gray-100");
      expect(header.className).not.toContain("bg-transparent");
    });

    it("animiert Hülle und Inhalt mit derselben Bewegung", () => {
      const { container } = render(<Sidebar />);

      const aside = container.querySelector("aside");
      const sticky = aside?.firstElementChild;
      expect(aside?.className).toContain("motion-safe:transition-[width]");
      expect(sticky?.className).toContain("motion-safe:transition-[width]");
      // Gleiche Breite auf beiden Ebenen: der Inhalt wandert mit der Kante,
      // statt am Ende der Bewegung noch einmal umzuspringen.
      expect(aside?.className).toContain("w-64");
      expect(sticky?.className).toContain("w-64");
    });

    it("blendet die Bezeichnung aus, statt sie sofort zu entfernen", async () => {
      render(<Sidebar />);

      act(() => {
        localStorage.setItem("sidebar-collapsed", "true");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });

      // Während der Breitenänderung steht der Text noch und blendet aus …
      const label = screen.getByText("Aktivitäten");
      expect(label.className).toContain("opacity-0");
      expect(label.className).toContain("truncate");

      // … und ist erst nach der Bewegung aus dem Baum verschwunden.
      await waitFor(() =>
        expect(screen.queryByText("Aktivitäten")).not.toBeInTheDocument(),
      );
      expect(screen.getByLabelText("Aktivitäten")).toBeInTheDocument();
    });

    // "Weitere Gruppen" erscheint nur, wenn es Gruppen gibt, die der Person
    // nicht selbst zugeordnet sind.
    const withOtherGroups = () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [{ id: "1", name: "Eulen", is_personal: false }],
        refresh: vi.fn(),
      });
    };

    it("zeigt im Streifen kein zweites, gleich aussehendes Gruppen-Icon", () => {
      withOtherGroups();
      localStorage.setItem("sidebar-collapsed", "true");

      render(<Sidebar />);

      // "Weitere Gruppen" trägt dasselbe Icon wie "Meine Gruppen" — ohne
      // Bezeichnung wären das zwei nicht unterscheidbare Schaltflächen.
      expect(
        screen.getByRole("button", { name: "Meine Gruppen" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Weitere Gruppen" }),
      ).not.toBeInTheDocument();
    });

    it("blendet das zweite Gruppen-Icon während des Einklappens aus", async () => {
      withOtherGroups();

      const { container } = render(<Sidebar />);
      expect(
        screen.getByRole("button", { name: "Weitere Gruppen" }),
      ).toBeInTheDocument();

      act(() => {
        localStorage.setItem("sidebar-collapsed", "true");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });

      // Der Bereich bleibt für die Dauer der Bewegung stehen — sonst
      // sprängen die Zeilen darunter im ersten Bild um seine ganze Höhe
      // hoch. Sichtbar ist er dabei nicht: er blendet aus und zieht seine
      // Höhe auf null, exponiert wird er auch nicht.
      const fading = container.querySelector(
        'div[aria-hidden="true"].grid.opacity-0.grid-rows-\\[0fr\\]',
      );
      expect(fading).not.toBeNull();
      expect(fading).toContainElement(screen.getByText("Weitere Gruppen"));
      expect(fading?.firstElementChild).toHaveAttribute("inert");
      // Die übrigen Bezeichnungen stehen noch und blenden aus.
      expect(screen.getByText("Aktivitäten")).toBeInTheDocument();

      // Nach der Bewegung ist der Bereich weg: im Streifen stünden sonst
      // zwei nicht unterscheidbare Gruppen-Icons untereinander.
      await waitFor(() =>
        expect(screen.queryByText("Weitere Gruppen")).not.toBeInTheDocument(),
      );
    });

    it("entfernt die Bezeichnungen bei prefers-reduced-motion sofort", () => {
      // Ohne Bewegung gibt es nichts, worauf die Texte warten könnten: die
      // Breite springt. Blieben sie die Dauer der Blende stehen, stünden im
      // 64px-Streifen für eine Viertelsekunde abgeschnittene Zeilen.
      const original = globalThis.matchMedia;
      globalThis.matchMedia = ((query: string) =>
        ({
          matches: query.includes("prefers-reduced-motion"),
          media: query,
          addEventListener: () => undefined,
          removeEventListener: () => undefined,
          addListener: () => undefined,
          removeListener: () => undefined,
          dispatchEvent: () => false,
          onchange: null,
        }) as unknown as MediaQueryList) as typeof globalThis.matchMedia;

      try {
        render(<Sidebar />);
        expect(screen.getByText("Aktivitäten")).toBeInTheDocument();

        act(() => {
          localStorage.setItem("sidebar-collapsed", "true");
          globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
        });

        expect(screen.queryByText("Aktivitäten")).not.toBeInTheDocument();
      } finally {
        globalThis.matchMedia = original;
      }
    });

    it("blendet das zweite Gruppen-Icon erst mit den Bezeichnungen wieder ein", async () => {
      withOtherGroups();
      localStorage.setItem("sidebar-collapsed", "true");

      const { container } = render(<Sidebar />);
      expect(screen.queryByText("Weitere Gruppen")).not.toBeInTheDocument();

      act(() => {
        localStorage.setItem("sidebar-collapsed", "false");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });

      // Der Bereich hängt sich unsichtbar ein und wächst mit den
      // Bezeichnungen auf — sonst stünden die beiden gleichen Icons ein Bild
      // lang unbeschriftet untereinander.
      const growing = container.querySelector(
        'div[aria-hidden="true"].grid.opacity-0.grid-rows-\\[0fr\\]',
      );
      expect(growing).toContainElement(screen.getByText("Weitere Gruppen"));

      await waitFor(() =>
        expect(
          container.querySelector(
            "div.grid.opacity-100.grid-rows-\\[1fr\\] .truncate",
          ),
        ).toHaveTextContent("Weitere Gruppen"),
      );
    });

    it("öffnet über das Gruppen-Icon wieder Meine Gruppen", async () => {
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen", is_personal: true },
          { id: "2", name: "Adler", is_personal: false },
        ],
        refresh: vi.fn(),
      });

      render(<Sidebar />);
      fireEvent.click(screen.getByText("Weitere Gruppen"));

      act(() => {
        localStorage.setItem("sidebar-collapsed", "true");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });
      await waitFor(() =>
        expect(screen.queryByText("Meine Gruppen")).not.toBeInTheDocument(),
      );

      // Das Icon im Streifen heißt "Meine Gruppen"; danach müssen die
      // eigenen Gruppen offen stehen und nicht der zuletzt gewählte
      // Unterbereich.
      fireEvent.click(screen.getByRole("button", { name: "Meine Gruppen" }));

      // "Weitere Gruppen" kommt erst mit den Bezeichnungen dazu, deshalb
      // beide Erwartungen in derselben Wartebedingung.
      await waitFor(() => {
        expect(
          screen.getByText("Meine Gruppen").closest("button"),
        ).toHaveAttribute("aria-expanded", "true");
        expect(
          screen.getByText("Weitere Gruppen").closest("button"),
        ).toHaveAttribute("aria-expanded", "false");
      });
    });

    it("öffnet Meine Gruppen auch bei geöffneter fremder Gruppe", async () => {
      // Die geöffnete Gruppe ist eine fremde — "Weitere Gruppen" steht
      // deshalb offen. Das Icon im Streifen heißt trotzdem "Meine Gruppen"
      // und muss genau die öffnen.
      mockUsePathname.mockReturnValue("/ogs-groups");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams((key: string) => (key === "group" ? "2" : null)),
      );
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: false,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [
          { id: "1", name: "Eulen", is_personal: true },
          { id: "2", name: "Adler", is_personal: false },
        ],
        refresh: vi.fn(),
      });
      localStorage.setItem("sidebar-collapsed", "true");

      render(<Sidebar />);

      fireEvent.click(screen.getByRole("button", { name: "Meine Gruppen" }));

      // "Weitere Gruppen" kommt erst mit den Bezeichnungen dazu, deshalb
      // beide Erwartungen in derselben Wartebedingung.
      await waitFor(() => {
        expect(
          screen.getByText("Meine Gruppen").closest("button"),
        ).toHaveAttribute("aria-expanded", "true");
        expect(
          screen.getByText("Weitere Gruppen").closest("button"),
        ).toHaveAttribute("aria-expanded", "false");
      });
    });

    it("nennt den Zähler während der Bewegung nur einmal", () => {
      mockHasPermission.mockImplementation(
        (_session, permission) =>
          permission === "users:update" ||
          permission === "users:delete" ||
          permission === "vacation:approve",
      );
      mockUseChangeRequestsPending.mockReturnValue({
        unreadCount: 9,
        isLoading: false,
        refresh: vi.fn(),
      });
      mockUseChangeRequestAccess.mockReturnValue({
        canOpenRequestsPage: true,
      } as ReturnType<typeof useChangeRequestAccess>);

      render(<Sidebar />);

      act(() => {
        localStorage.setItem("sidebar-collapsed", "true");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });

      // Beide Zähler stehen für die Dauer der Gegenblende im Baum — der
      // ausblendende ist unsichtbar und darf deshalb auch nicht vorgelesen
      // werden.
      const badges = screen.getAllByLabelText("9 offene Anfragen");
      expect(badges).toHaveLength(2);
      expect(
        badges.filter(
          (badge) =>
            badge.parentElement?.getAttribute("aria-hidden") !== "true",
        ),
      ).toHaveLength(1);
    });

    it("markiert den Bereich im Streifen auch bei geöffnetem Unterpunkt", () => {
      // Ausgeklappt trägt der Unterpunkt "Räume" die Markierung, die
      // Kopfzeile bleibt deshalb ungrau. Im Streifen ist der Unterpunkt nicht
      // sichtbar — dort muss der Bereich selbst markiert sein, sonst steht
      // die Leiste ganz ohne Hinweis da, wo man gerade ist.
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUsePathname.mockReturnValue("/database/rooms");

      const { unmount } = render(<Sidebar />);
      expect(
        screen.getByRole("button", { name: "Datenverwaltung" }).className,
      ).not.toContain("bg-gray-100");
      unmount();

      localStorage.setItem("sidebar-collapsed", "true");
      render(<Sidebar />);

      expect(
        screen.getByRole("button", { name: "Datenverwaltung" }).className,
      ).toContain("bg-gray-100");
    });

    it("nennt den Sammelzähler eines Bereichs während der Bewegung nur einmal", () => {
      vi.mocked(useMessagesUnread).mockReturnValue({
        unreadCount: 4,
        isLoading: false,
        refresh: vi.fn(),
      });

      render(<Sidebar />);

      act(() => {
        localStorage.setItem("sidebar-collapsed", "true");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });

      // Für die Dauer der Gegenblende stehen beide Zähler im Baum; der
      // ausblendende ist unsichtbar und darf nicht mitgelesen werden.
      const header = screen.getByText("Eltern").closest("button");
      const badges = within(header!).getAllByLabelText(
        "4 ungelesene Nachrichten",
      );
      expect(badges).toHaveLength(2);
      expect(
        badges.filter(
          (badge) =>
            badge.parentElement?.getAttribute("aria-hidden") !== "true",
        ),
      ).toHaveLength(1);
    });

    it("hält den offenen Bereich 'Weitere Gruppen' samt Unterpunkten bis zum Ende der Bewegung", async () => {
      withOtherGroups();

      const { container } = render(<Sidebar />);
      fireEvent.click(screen.getByText("Weitere Gruppen"));
      await waitFor(() =>
        expect(screen.getByText("Eulen")).toBeInTheDocument(),
      );

      act(() => {
        localStorage.setItem("sidebar-collapsed", "true");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });

      // Der ganze Bereich bleibt stehen — Kopfzeile und Unterpunkte —, und
      // seine Höhe geht mit derselben Kurve auf null wie die Breite der
      // Leiste. Verschwände er sofort, sprängen alle Zeilen darunter im
      // ersten Bild um seine volle Höhe hoch.
      const fading = container.querySelector(
        'div[aria-hidden="true"].grid.grid-rows-\\[0fr\\]',
      );
      expect(fading).toContainElement(screen.getByText("Weitere Gruppen"));
      expect(fading).toContainElement(screen.getByText("Eulen"));

      // Erst nach der Bewegung geht der Bereich aus dem Baum.
      await waitFor(() =>
        expect(screen.queryByText("Weitere Gruppen")).not.toBeInTheDocument(),
      );
      expect(screen.queryByText("Eulen")).not.toBeInTheDocument();
    });

    it("klappt den offenen Bereich nicht zusätzlich zur Hülle zu", async () => {
      withOtherGroups();

      render(<Sidebar />);
      fireEvent.click(screen.getByText("Weitere Gruppen"));
      await waitFor(() =>
        expect(screen.getByText("Eulen")).toBeInTheDocument(),
      );

      act(() => {
        localStorage.setItem("sidebar-collapsed", "true");
        globalThis.dispatchEvent(new Event("sidebar-collapsed-change"));
      });

      // Die Hülle zieht die volle Höhe zusammen. Der Inhalt behält seine
      // Höhe bis zum Ende der Bewegung — zwei geschachtelte Höhenwechsel
      // ergäben sonst eine zweite Bewegung der Zeilen darunter.
      const header = screen.getByText("Weitere Gruppen").closest("button");
      expect(header!.nextElementSibling).toHaveClass("grid-rows-[1fr]");
    });

    it("hält geschlossene Bereiche aus der Tastaturreihenfolge heraus", () => {
      render(<Sidebar />);

      // Der Bereich "Eltern" ist zu; seine Unterpunkte sind unsichtbar und
      // dürfen den Tastaturfokus nicht fangen.
      const header = screen.getByRole("button", { name: "Eltern" });
      const body = header.nextElementSibling?.firstElementChild;
      expect(body).toHaveAttribute("inert");
    });
  });

  // Render-Budget (#2939): Profiler-Commits in 5 s Leerlauf, siehe
  // ~/test/render-budget. Eine neue Effekt-Schleife in der Shell fällt hier auf.
  describe("render budget", () => {
    afterEach(() => {
      vi.useRealTimers();
    });

    it.each([
      ["/dashboard", true],
      ["/ogs-groups", false],
    ])(
      `commits at most ${RENDER_BUDGET_MAX_COMMITS} times in idle on %s`,
      async (pathname, admin) => {
        vi.useFakeTimers();
        mockIsAdmin.mockReturnValue(admin);
        mockUseSession.mockReturnValue(createMockSession(admin));
        mockUsePathname.mockReturnValue(pathname);

        await expectIdleRenderBudget(<Sidebar />);
      },
    );
  });
});
