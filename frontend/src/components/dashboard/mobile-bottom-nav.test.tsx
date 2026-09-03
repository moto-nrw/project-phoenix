import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
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
  useOptionalSupervision: vi.fn(),
}));

vi.mock("~/lib/auth-utils", () => {
  const isAdminFn = vi.fn();
  const isCaregiverFn = vi.fn(() => !isAdminFn());
  return {
    isAdmin: isAdminFn,
    isCaregiver: isCaregiverFn,
    hasEffectiveAdminScope: vi.fn(() => isAdminFn()),
    hasRole: vi.fn((_session: unknown, role: string) => {
      if (role === "admin") return isAdminFn();
      if (role === "user") return !isAdminFn();
      return false;
    }),
    hasPermission: vi.fn(() => false),
  };
});

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
      <button
        type="button"
        onClick={() => onOpenChange(false)}
        data-testid="drawer-close"
      >
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

vi.mock("./header/refresh-button", () => ({
  RefreshButton: () => <button type="button">Aktualisieren</button>,
}));

vi.mock("~/components/staff-preview/staff-preview-modal", () => ({
  StaffPreviewModal: ({ isOpen }: { isOpen: boolean }) =>
    isOpen ? <div>Vorschau-Dialog</div> : null,
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: vi.fn(() => ({
    user: { name: "Test User", email: "test@example.com", roles: [] },
    profile: { firstName: "Test", lastName: "User" },
    status: "authenticated",
    isSessionExpired: false,
    logout: vi.fn(),
    mode: "teacher",
    homeUrl: "/dashboard",

    profileUrl: "/profile",
  })),
}));

vi.mock("~/lib/hooks/use-change-request-access", () => ({
  useChangeRequestAccess: vi.fn(),
}));

vi.mock("~/lib/operator-url", () => ({
  operatorPath: (path: string) => path,
}));

// Import after mocks
import { MobileBottomNav } from "./mobile-bottom-nav";
import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useOptionalSupervision } from "~/lib/supervision-context";
import {
  hasEffectiveAdminScope,
  hasPermission,
  isAdmin,
  isCaregiver,
} from "~/lib/auth-utils";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useChangeRequestAccess } from "~/lib/hooks/use-change-request-access";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
  useStaffMessagingEnabled,
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import useSWR from "swr";

const mockUsePathname = vi.mocked(usePathname);
const mockUseSearchParams = vi.mocked(useSearchParams);
const mockUseSession = vi.mocked(useSession);
const mockUseSupervision = vi.mocked(useOptionalSupervision);
const mockIsAdmin = vi.mocked(isAdmin);
const mockIsCaregiver = vi.mocked(isCaregiver);
const mockHasEffectiveAdminScope = vi.mocked(hasEffectiveAdminScope);
const mockHasPermission = vi.mocked(hasPermission);
const mockUseShellAuth = vi.mocked(useShellAuth);
const mockUseChangeRequestAccess = vi.mocked(useChangeRequestAccess);
const mockUseNFCEnabled = vi.mocked(useNFCEnabled);
const mockUsePresenceMode = vi.mocked(usePresenceMode);
const mockUseTenantRoutingModeSafe = vi.mocked(useTenantRoutingModeSafe);
const mockUseTenantSlugSafe = vi.mocked(useTenantSlugSafe);
const mockUseOpenCareGroupMode = vi.mocked(useOpenCareGroupMode);
const mockUseStaffMessagingEnabled = vi.mocked(useStaffMessagingEnabled);
const mockUseSWRDefault = vi.mocked(useSWR);

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
    mockUseShellAuth.mockReturnValue({
      user: { name: "Test User", email: "test@example.com", roles: [] },
      profile: { firstName: "Test", lastName: "User" },
      status: "authenticated",
      isSessionExpired: false,
      logout: vi.fn(),
      mode: "teacher",
      homeUrl: "/dashboard",

      profileUrl: "/profile",
      canStartStaffPreview: false,
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
    mockIsCaregiver.mockImplementation((session) => !mockIsAdmin(session));
    mockHasEffectiveAdminScope.mockImplementation((session) =>
      mockIsAdmin(session),
    );
    mockHasPermission.mockReturnValue(false);
    mockUseNFCEnabled.mockReturnValue(true);
    mockUsePresenceMode.mockReturnValue("detailed");
    // Re-establish tenant defaults each test so per-test subdomain/slug
    // overrides (below) don't leak — vi.clearAllMocks() keeps the setup.ts
    // implementations but individual mockReturnValue calls would otherwise
    // persist across tests.
    mockUseTenantRoutingModeSafe.mockReturnValue("path");
    mockUseTenantSlugSafe.mockReturnValue("test-tenant");
    mockUseStaffMessagingEnabled.mockReturnValue(false);
    mockUseChangeRequestAccess.mockReturnValue({
      canOpenRequestsPage: false,
    } as ReturnType<typeof useChangeRequestAccess>);
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

    it("names icon-only staff nav controls for assistive technology", () => {
      render(<MobileBottomNav />);

      expect(screen.getByRole("link", { name: "Gruppe" })).toHaveAttribute(
        "href",
        "/ogs-groups",
      );
      expect(screen.getByRole("link", { name: "Aufsicht" })).toHaveAttribute(
        "href",
        "/active-supervisions",
      );
      expect(screen.getByRole("button", { name: "Mehr" })).toBeInTheDocument();
    });

    it("renders navigation bar for admin users", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<MobileBottomNav />);

      // Admin main items - check by href
      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/dashboard");
      expect(hrefs).toContain("/students/search");

      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));
      expect(screen.getByRole("link", { name: "Gruppe" })).toHaveAttribute(
        "href",
        "/ogs-groups",
      );
    });

    it("hides groups from users without staff or admin access", () => {
      mockIsCaregiver.mockReturnValue(false);

      render(<MobileBottomNav />);

      expect(screen.queryByRole("link", { name: "Gruppe" })).toBeNull();
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));
      expect(screen.queryByRole("link", { name: "Gruppe" })).toBeNull();
    });

    it("shows groups to effective admins", () => {
      mockIsCaregiver.mockReturnValue(false);
      mockHasEffectiveAdminScope.mockReturnValue(true);

      render(<MobileBottomNav />);

      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));
      expect(screen.getByRole("link", { name: "Gruppe" })).toHaveAttribute(
        "href",
        "/ogs-groups",
      );
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

    it("keeps header actions available in the overflow menu", () => {
      mockUseShellAuth.mockReturnValue({
        user: { name: "Test User", email: "test@example.com", roles: [] },
        profile: { firstName: "Test", lastName: "User" },
        status: "authenticated",
        isSessionExpired: true,
        logout: vi.fn(),
        mode: "teacher",
        homeUrl: "/dashboard",
        profileUrl: "/profile",
        canStartStaffPreview: true,
      });

      render(<MobileBottomNav />);
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

      expect(
        screen.getByText(
          "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
        ),
      ).toBeInTheDocument();
      expect(screen.getAllByText("Aktualisieren")).not.toHaveLength(0);
      expect(screen.getByText("Erinnerungen").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/reminders",
      );
      expect(screen.getByText("Profil").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/profile",
      );
      fireEvent.click(screen.getByText("Ansicht eines Mitarbeitenden"));
      expect(screen.getByText("Vorschau-Dialog")).toBeInTheDocument();
    });

    it("prefixes the Anfragen overflow link in path-routing mode", () => {
      mockHasPermission.mockImplementation(
        (_session, permission) => permission === "vacation:approve",
      );
      mockUseChangeRequestAccess.mockReturnValue({
        canOpenRequestsPage: true,
      } as ReturnType<typeof useChangeRequestAccess>);

      render(<MobileBottomNav />);
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

      expect(screen.getByText("Anfragen").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/anfragen",
      );
    });

    it("hides Anfragen without a current effective review scope", () => {
      mockHasPermission.mockImplementation(
        (_session, permission) => permission === "users:update",
      );

      render(<MobileBottomNav />);
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

      expect(screen.queryByText("Anfragen")).not.toBeInTheDocument();
    });

    it("prefixes the Team-Chat overflow link in path-routing mode", () => {
      mockUseStaffMessagingEnabled.mockReturnValue(true);

      render(<MobileBottomNav />);
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

      expect(screen.getByText("Team-Chat").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/team-chat",
      );
    });
  });

  describe("active route detection", () => {
    it("highlights dashboard link when on dashboard", () => {
      mockUsePathname.mockReturnValue("/dashboard");
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<MobileBottomNav />);

      // The active link should have the bg-gray-100 class and show label
      const dashboardLink = screen
        .getAllByRole("link")
        .find((link) => link.getAttribute("href") === "/dashboard");
      expect(dashboardLink).toHaveClass("bg-gray-100");
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

    it("highlights the Eltern group when a child page was reached via one of its activePaths", () => {
      // A grouped item (Eltern) owns several routes through activePaths. A
      // child page opened with ?from=/messages must still light up the "Mehr"
      // entry that hosts Eltern — the referrer check compares `from` against
      // both the item href and its activePaths.
      mockUsePathname.mockReturnValue("/students/123");
      mockUseSearchParams.mockReturnValue(
        createMockSearchParams(() => "/messages"),
      );

      render(<MobileBottomNav />);

      expect(screen.getByRole("button", { name: "Mehr" })).toHaveClass(
        "bg-gray-100",
      );
    });

    it("does not strip a slug-shaped path segment in subdomain routing", () => {
      // A tenant whose slug collides with a real route (e.g. "messages") on
      // messages.<domain>/messages: usePathname() is already unprefixed in
      // subdomain mode, so the tenant-prefix normalization must be a no-op.
      // Otherwise "/messages" would be mistaken for the bare tenant root and
      // stripped to "/", mis-highlighting Home instead of the Eltern group.
      mockUseTenantRoutingModeSafe.mockReturnValue("subdomain");
      mockUseTenantSlugSafe.mockReturnValue("messages");
      mockUsePathname.mockReturnValue("/messages");

      render(<MobileBottomNav />);

      // Home must NOT be active (its label only renders when active)…
      expect(screen.queryByText("Home")).not.toBeInTheDocument();
      // …and the Eltern group ("Mehr") is active via its /messages activePath.
      expect(screen.getByRole("button", { name: "Mehr" })).toHaveClass(
        "bg-gray-100",
      );
    });

    it("highlights the canonical activities route", () => {
      mockUsePathname.mockReturnValue("/activities");

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
      expect(screen.getByText("Betreuungsplan")).toBeInTheDocument();
      expect(screen.getByText("Dienstplan")).toBeInTheDocument();
      expect(screen.getByText("Terminvertretungen")).toBeInTheDocument();
      expect(screen.queryByText("Planung")).not.toBeInTheDocument();
      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
      expect(screen.queryByText("Übergaben")).not.toBeInTheDocument();
      expect(screen.getByText("Datenverwaltung")).toBeInTheDocument();
    });

    it("labels the staff calendar entry 'Mein Kalender' in the overflow menu", () => {
      // Der Staff-Eintrag auf /calendar heißt wie die H1 der Seite. Der
      // gleichnamige Eltern-Eintrag (/parents/calendar) bleibt "Kalender" —
      // siehe "parent mode navigation" unten.
      render(<MobileBottomNav />);
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

      // Nur Planungs- und Eltern-Hub-Links tragen das Tenant-Präfix; /calendar
      // bleibt bar.
      expect(screen.getByText("Mein Kalender").closest("a")).toHaveAttribute(
        "href",
        "/calendar",
      );
      expect(screen.queryByText("Kalender")).not.toBeInTheDocument();
    });

    it("lists Abrechnung in the overflow menu like every other planning page", () => {
      // Abrechnung ist jetzt Unterpunkt des Planung-Bereichs und steht damit
      // auch mobil in der flachen Navigation — PLANNING_SUB_PAGES verlangt das
      // für jede Planungsseite (siehe planning-navigation.test.ts).
      render(<MobileBottomNav />);
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

      expect(screen.getByText("Dienstplan")).toBeInTheDocument();
      expect(screen.getByText("Abrechnung")).toBeInTheDocument();
      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/test-tenant/payroll");
    });

    it("lists only Abrechnung for non-admins holding config:manage", () => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSession.mockReturnValue(createMockSession(false));
      mockHasPermission.mockImplementation(
        (_session, permission) => permission === "config:manage",
      );

      render(<MobileBottomNav />);
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

      expect(screen.getByText("Abrechnung").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/payroll",
      );
      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
    });

    it("prefixes all planning links in tenant path-routing mode", () => {
      render(<MobileBottomNav />);

      const moreButton = screen.getByRole("button", { name: "Mehr" });
      fireEvent.click(moreButton);

      expect(screen.getByText("Betreuungsplan").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/betreuungsplan",
      );
      expect(screen.getByText("Dienstplan").closest("a")).toHaveAttribute(
        "href",
        "/test-tenant/dienstplan",
      );
      expect(
        screen.getByText("Terminvertretungen").closest("a"),
      ).toHaveAttribute("href", "/test-tenant/vertretung");
    });

    it("keeps planning links bare in subdomain mode", () => {
      mockUseTenantRoutingModeSafe.mockReturnValue("subdomain");

      render(<MobileBottomNav />);
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

      expect(screen.getByText("Betreuungsplan").closest("a")).toHaveAttribute(
        "href",
        "/betreuungsplan",
      );
      expect(screen.getByText("Dienstplan").closest("a")).toHaveAttribute(
        "href",
        "/dienstplan",
      );
      expect(
        screen.getByText("Terminvertretungen").closest("a"),
      ).toHaveAttribute("href", "/vertretung");
    });

    it("highlights Dienstplan without also highlighting Mitarbeiter in the overflow menu", () => {
      // /staff/dienstplan ist nur noch der Redirect-Frame des Dienstplans
      // (Planung-Redesign) und gehört nicht mehr zur Mitarbeiter-Sektion.
      mockUsePathname.mockReturnValue("/staff/dienstplan");

      render(<MobileBottomNav />);

      const navButtons = screen.getAllByRole("button");
      const moreButton = navButtons.find(
        (btn) => !btn.hasAttribute("data-testid"),
      );
      expect(moreButton).toBeDefined();
      fireEvent.click(moreButton!);

      const dienstplanLink = screen.getByText("Dienstplan").closest("a");
      const staffLink = screen.getByText("Mitarbeiter").closest("a");
      expect(dienstplanLink).toHaveClass("bg-gray-100");
      expect(staffLink).not.toHaveClass("bg-gray-100");
    });

    it("highlights Dienstplan under a tenant-prefixed path", () => {
      mockUsePathname.mockReturnValue("/test-tenant/dienstplan");

      render(<MobileBottomNav />);
      fireEvent.click(screen.getByRole("button", { name: "Mehr" }));

      expect(screen.getByText("Dienstplan").closest("a")).toHaveClass(
        "bg-gray-100",
      );
    });

    it("highlights Kalenderzeiträume as its own overflow entry", () => {
      // Kalenderzeiträume und Tageslisten waren mobil ausgeblendet und liehen
      // sich die Hervorhebung vom Betreuungsplan. Erreichbar waren sie dadurch
      // nicht: es gibt keinen Verweis vom Betreuungsplan dorthin. Beide sind
      // jetzt eigene Einträge und markieren sich selbst.
      mockUsePathname.mockReturnValue("/calendar-periods");

      render(<MobileBottomNav />);

      const navButtons = screen.getAllByRole("button");
      const moreButton = navButtons.find(
        (btn) => !btn.hasAttribute("data-testid"),
      );
      expect(moreButton).toBeDefined();
      fireEvent.click(moreButton!);

      expect(screen.getByText("Kalenderzeiträume").closest("a")).toHaveClass(
        "bg-gray-100",
      );
      expect(screen.getByText("Tageslisten").closest("a")).toBeInTheDocument();
      expect(screen.getByText("Betreuungsplan").closest("a")).not.toHaveClass(
        "bg-gray-100",
      );
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
        overviewEnabled: false,
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

  describe("NFC visibility", () => {
    beforeEach(() => {
      mockUseNFCEnabled.mockReturnValue(false);
    });

    it("hides the classic activities item from staff main navigation", () => {
      render(<MobileBottomNav />);

      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
      expect(hrefs).not.toContain("/activities");
      expect(hrefs).toContain("/ogs-groups");
      expect(hrefs).toContain("/active-supervisions");
    });

    it("hides the classic activities item from the overflow drawer", () => {
      render(<MobileBottomNav />);

      const navButtons = screen.getAllByRole("button");
      const moreButton = navButtons.find(
        (btn) => !btn.hasAttribute("data-testid"),
      );
      expect(moreButton).toBeDefined();
      fireEvent.click(moreButton!);

      const drawerLinks = screen
        .getByTestId("drawer-content")
        .querySelectorAll("a");
      const drawerHrefs = Array.from(drawerLinks).map((link) =>
        link.getAttribute("href"),
      );
      expect(drawerHrefs).not.toContain("/activities");
    });

    it("hides the classic activities item in binary presence mode", () => {
      mockUseNFCEnabled.mockReturnValue(true);
      mockUsePresenceMode.mockReturnValue("binary");

      render(<MobileBottomNav />);

      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
      expect(hrefs).not.toContain("/activities");
      expect(hrefs).toContain("/ogs-groups");
    });
  });

  // #2915: Räume, Aktivitäten und Aufsicht sperrt der BinaryModeGuard — die
  // mobile Navigation muss sie unter derselben Regel ausblenden wie die
  // Desktop-Sidebar, sonst führt der Eintrag auf eine 404-Seite.
  describe("binary presence mode", () => {
    beforeEach(() => {
      mockUseNFCEnabled.mockReturnValue(true);
      mockUsePresenceMode.mockReturnValue("binary");
    });

    it("hides Aufsicht from the staff main navigation", () => {
      render(<MobileBottomNav />);

      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
      expect(hrefs).not.toContain("/active-supervisions");
      expect(
        screen.queryByRole("link", { name: "Aufsicht" }),
      ).not.toBeInTheDocument();
      expect(hrefs).toContain("/ogs-groups");
    });

    it("hides Aufsicht and Räume from the overflow menu", () => {
      render(<MobileBottomNav />);

      const moreButton = screen
        .getAllByRole("button")
        .find((btn) => !btn.hasAttribute("data-testid"));
      expect(moreButton).toBeDefined();
      fireEvent.click(moreButton!);

      const drawerHrefs = Array.from(
        screen.getByTestId("drawer-content").querySelectorAll("a"),
      ).map((link) => link.getAttribute("href"));
      expect(drawerHrefs).not.toContain("/active-supervisions");
      expect(drawerHrefs).not.toContain("/rooms");
    });

    it("hides the injected admin Aufsicht tab", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: true,
        supervisedRooms: [{ id: "1", name: "Room A", groupId: "g1" }],
        groups: [],
        refresh: vi.fn(),
      });

      render(<MobileBottomNav />);

      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
      expect(hrefs).not.toContain("/active-supervisions");
    });

    it("keeps Aufsicht visible in detailed presence mode", () => {
      mockUsePresenceMode.mockReturnValue("detailed");

      render(<MobileBottomNav />);

      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
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
      // Einstellungen requires admin — test with admin to see all items
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      render(<MobileBottomNav />);

      // Open overflow menu
      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      // Drawer should contain overflow items (those not in the main bottom bar)
      const drawer = screen.getByTestId("drawer");
      expect(drawer).toBeInTheDocument();
      // At minimum, Einstellungen should appear in the drawer for admins
      expect(screen.getByText("Einstellungen")).toBeInTheDocument();
    });
  });

  describe("coming soon items", () => {
    // Helper to get the More button
    const getMoreButton = () => {
      const navButtons = screen.getAllByRole("button");
      return navButtons.find((btn) => !btn.hasAttribute("data-testid"))!;
    };

    it("shows Statistik as a real link for admins, without a coming-soon badge (#2606)", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<MobileBottomNav />);

      // Open overflow menu
      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      const link = screen.getByText("Statistik").closest("a");
      expect(link).not.toBeNull();
      expect(link).toHaveAttribute("href", "/statistics");
      expect(screen.queryByText("Berichte")).not.toBeInTheDocument();
      expect(screen.queryByText("Bald verfügbar")).not.toBeInTheDocument();
    });

    it("shows Statistik to non-admin staff with both required permissions", () => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSession.mockReturnValue(createMockSession(false));
      mockHasPermission.mockImplementation(
        (_session, permission) =>
          permission === "config:read" || permission === "users:read",
      );

      render(<MobileBottomNav />);

      fireEvent.click(getMoreButton());
      expect(screen.getByText("Statistik").closest("a")).toHaveAttribute(
        "href",
        "/statistics",
      );
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

    it("does not show the old Dienstpläne placeholder for admins", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));

      render(<MobileBottomNav />);

      // Open overflow menu
      const moreButton = getMoreButton();
      fireEvent.click(moreButton);

      expect(screen.queryByText("Dienstpläne")).not.toBeInTheDocument();
      expect(screen.getByText("Statistik")).toBeInTheDocument();
    });
  });

  describe("supervision-based visibility", () => {
    it("shows supervision-related links when user has groups", () => {
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
        overviewEnabled: false,
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

    it("injects Aufsicht tab for admins when admin_supervision_overview is enabled", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUseSupervision.mockReturnValue({
        hasGroups: true,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: true,
        supervisedRooms: [{ id: "1", name: "Room A", groupId: "g1" }],
        groups: [],
        refresh: vi.fn(),
      });

      render(<MobileBottomNav />);

      // Admin baseline does NOT include /active-supervisions — injection must add it
      const links = screen.getAllByRole("link");
      const hrefs = links.map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/active-supervisions");
    });

    it("does not inject Aufsicht tab for admin while supervision is still loading", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: false,
        isLoadingGroups: true,
        isLoadingSupervision: true,
        overviewEnabled: false,
        supervisedRooms: [],
        groups: [],
        refresh: vi.fn(),
      });

      render(<MobileBottomNav />);

      const links = screen.getAllByRole("link");
      const hrefs = links.map((link) => link.getAttribute("href"));
      expect(hrefs).not.toContain("/active-supervisions");
    });

    it("does not inject Aufsicht tab when only a synthetic Schulhof room exists (setting off)", () => {
      // P1-A regression guard: Schulhof is injected into supervisedRooms for
      // every tenant that has one. An admin without admin_supervision_overview
      // must not surface the admin tab merely because a Schulhof entry exists.
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
      mockUseSupervision.mockReturnValue({
        hasGroups: false,
        isSupervising: true,
        isLoadingGroups: false,
        isLoadingSupervision: false,
        overviewEnabled: false,
        supervisedRooms: [
          { id: "schulhof", name: "Schulhof", groupId: "g1", isSchulhof: true },
        ],
        groups: [],
        refresh: vi.fn(),
      });

      render(<MobileBottomNav />);

      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
      expect(hrefs).not.toContain("/active-supervisions");
    });

    it("does not inject Aufsicht tab for admin who is not supervising", () => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
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

      render(<MobileBottomNav />);

      const links = screen.getAllByRole("link");
      const hrefs = links.map((link) => link.getAttribute("href"));
      expect(hrefs).not.toContain("/active-supervisions");
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
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
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
      // The indicator is a div with bg-gray-100 class
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
      const activeLinks = container.querySelectorAll("a.bg-gray-100");
      expect(activeLinks.length).toBe(0);

      vi.useRealTimers();
    });

    it("shows indicator on more button when additional route is active", async () => {
      vi.useFakeTimers();
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
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

  describe("operator mode navigation", () => {
    beforeEach(() => {
      mockUseShellAuth.mockReturnValue({
        user: {
          name: "Operator",
          email: "op@example.com",
          roles: ["operator"],
        },
        profile: { firstName: "Operator" },
        status: "authenticated",
        isSessionExpired: false,
        logout: vi.fn(),
        mode: "operator",
        homeUrl: "/operator/organizations",

        profileUrl: "/operator/settings",
      });
      mockUsePathname.mockReturnValue("/operator/organizations");
    });

    it("renders operator main items", () => {
      render(<MobileBottomNav />);

      const links = screen.getAllByRole("link");
      const hrefs = links.map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/operator/announcements");
      // Verwaltung entry now points at the first management page (Träger)
      // since the old single-page /operator/provisioning was split into five
      // dedicated routes. See issue #1282 (operator sidebar restructure).
      expect(hrefs).toContain("/operator/organizations");
      expect(hrefs).toContain("/operator/operators");
    });

    it("shows overflow menu in operator mode with the remaining Verwaltung pages", async () => {
      // Issue #1282: the old operator bottom nav had no overflow because the
      // single /operator/provisioning page housed all Verwaltung tabs. After
      // the tabs were split into dedicated routes the overflow drawer must
      // surface the remaining sibling pages so mobile users can reach them.
      render(<MobileBottomNav />);

      const navButtons = screen.getAllByRole("button");
      const moreButton = navButtons.find(
        (btn) => !btn.hasAttribute("data-testid"),
      );
      expect(moreButton).toBeDefined();

      fireEvent.click(moreButton!);

      const links = await screen.findAllByRole("link");
      const hrefs = links.map((l) => l.getAttribute("href"));
      expect(hrefs).toContain("/operator/schools");
      expect(hrefs).toContain("/operator/accounts");
      expect(hrefs).toContain("/operator/devices");
      expect(hrefs).toContain("/operator/persons");
      expect(hrefs).toContain("/operator/settings");
    });

    it("keeps the refresh action available in the operator overflow menu", () => {
      render(<MobileBottomNav />);

      const moreButton = screen
        .getAllByRole("button")
        .find((button) => !button.hasAttribute("data-testid"));
      expect(moreButton).toBeDefined();
      fireEvent.click(moreButton!);

      expect(
        screen.getByRole("button", { name: "Aktualisieren" }),
      ).toBeInTheDocument();
    });

    it("shows active label for current operator route", () => {
      render(<MobileBottomNav />);

      expect(screen.getByText("Verwaltung")).toBeInTheDocument();
    });
  });

  describe("Icon component", () => {
    it("renders SVG icons correctly", () => {
      render(<MobileBottomNav />);

      // SVGs should be rendered in navigation
      const svgs = document.querySelectorAll("svg");
      expect(svgs.length).toBeGreaterThan(0);

      // Legacy utility icons use a 24px viewBox, Phosphor concept icons use 256px.
      svgs.forEach((svg) => {
        expect(["0 0 24 24", "0 0 256 256"]).toContain(
          svg.getAttribute("viewBox"),
        );
        expect(svg).toHaveAttribute("aria-hidden", "true");
      });
    });
  });

  describe("feature gating (#1940/#1946)", () => {
    beforeEach(() => {
      mockIsAdmin.mockReturnValue(true);
      mockUseSession.mockReturnValue(createMockSession(true));
    });

    afterEach(() => {
      mockUseOpenCareGroupMode.mockReturnValue(false);
      // Restore the global SWR default so the schema override doesn't leak.
      mockUseSWRDefault.mockReturnValue({
        data: undefined,
        error: undefined,
        isLoading: true,
        isValidating: false,
        mutate: vi.fn(),
      } as unknown as ReturnType<typeof useSWR>);
    });

    function openDrawer() {
      const navButtons = screen.getAllByRole("button");
      const moreButton = navButtons.find(
        (btn) => !btn.hasAttribute("data-testid"),
      );
      expect(moreButton).toBeDefined();
      fireEvent.click(moreButton!);
    }

    it("keeps Vertretungen for open-care tenants", () => {
      mockUseOpenCareGroupMode.mockReturnValue(true);

      render(<MobileBottomNav />);
      openDrawer();

      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
      // Planung-Einträge bleiben sichtbar (timetable.enabled ungesetzt).
      expect(screen.getByText("Betreuungsplan")).toBeInTheDocument();
    });

    it("shows Vertretungen to staff", () => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSession.mockReturnValue(createMockSession(false));

      render(<MobileBottomNav />);
      openDrawer();

      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
    });

    it("hides the Gruppe main item for open-care staff (#1544)", () => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSession.mockReturnValue(createMockSession(false));
      mockUseOpenCareGroupMode.mockReturnValue(true);

      render(<MobileBottomNav />);

      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
      expect(hrefs).not.toContain("/ogs-groups");
      // Die übrigen Staff-Einstiege bleiben erhalten.
      expect(hrefs).toContain("/active-supervisions");
      expect(hrefs).toContain("/students/search");
    });

    it("keeps the Gruppe main item for fixed-groups staff (#1544)", () => {
      mockIsAdmin.mockReturnValue(false);
      mockUseSession.mockReturnValue(createMockSession(false));

      render(<MobileBottomNav />);

      const hrefs = screen
        .getAllByRole("link")
        .map((link) => link.getAttribute("href"));
      expect(hrefs).toContain("/ogs-groups");
    });

    it("keeps calendar periods and payroll reachable when timetable.enabled is false", () => {
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

      render(<MobileBottomNav />);
      openDrawer();

      expect(screen.queryByText("Betreuungsplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Dienstplan")).not.toBeInTheDocument();
      expect(screen.queryByText("Terminvertretungen")).not.toBeInTheDocument();
      expect(screen.getByText("Kalenderzeiträume")).toBeInTheDocument();
      expect(screen.getByText("Abrechnung")).toBeInTheDocument();
      expect(screen.getByText("Vertretungen")).toBeInTheDocument();
    });

    it("reads timetable.enabled from the tenant-scoped SWR key", () => {
      render(<MobileBottomNav />);

      expect(mockUseSWRDefault).toHaveBeenCalledWith(
        "test-tenant:settings-schema",
        expect.any(Function),
        expect.any(Object),
      );
    });
  });
});
