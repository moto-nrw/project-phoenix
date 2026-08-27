import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import DashboardPage from "./page";

const mockPush = vi.fn();
const mockReplace = vi.fn();

const mockRedirect = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    replace: mockReplace,
  }),
  redirect: (url: string) => mockRedirect(url),
}));

const mockSession = {
  user: {
    id: "1",
    name: "Test Admin",
    email: "admin@test.com",
    token: "test-token",
    isAdmin: true,
    firstName: "Test",
  },
  expires: "2099-12-31",
};

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: mockSession,
    status: "authenticated",
  })),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: vi.fn((session) => session?.user?.isAdmin ?? false),
  hasEffectiveAdminScope: vi.fn((session) => session?.user?.isAdmin ?? false),
  hasPermission: vi.fn((session, permission: string) =>
    permission === "admin:*" ? (session?.user?.isAdmin ?? false) : false,
  ),
  hasRole: vi.fn(
    (session: { user?: { isAdmin?: boolean } } | null, role: string) => {
      if (role === "admin") return session?.user?.isAdmin ?? false;
      if (role === "user") return !(session?.user?.isAdmin ?? false);
      return false;
    },
  ),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/lib/usercontext-context", () => ({
  UserContextProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="user-context-provider">{children}</div>
  ),
}));

vi.mock("~/components/enrollment/phase-expiry-warnings", () => ({
  PhaseExpiryWarnings: () => <div data-testid="phase-expiry-warnings" />,
}));

const mockDashboardData = {
  studentsPresent: 150,
  studentsInRooms: 120,
  studentsInTransit: 20,
  studentsOnPlayground: 10,
  studentsSick: 4,
  studentsExcused: 3,
  studentsHome: 33,
  activeOGSGroups: 8,
  activeActivities: 5,
  capacityUtilization: 0.75,
  supervisorsToday: 10,
  recentActivity: [
    {
      type: "checkin",
      groupName: "Gruppe 1",
      roomName: "Raum 101",
      timestamp: new Date().toISOString(),
      count: 5,
    },
  ],
  currentActivities: [
    {
      id: "101",
      name: "Schach",
      category: "Sport",
      participants: 8,
      maxCapacity: 10,
      status: "active",
    },
  ],
  activeGroupsSummary: [
    {
      type: "ogs",
      name: "OGS Gruppe A",
      location: "Raum 101",
      studentCount: 15,
      status: "active",
    },
  ],
};

vi.mock("~/lib/swr/hooks", () => ({
  useSWRAuth: vi.fn(),
}));

vi.mock("~/lib/tenant-context", () => ({
  useNFCEnabled: vi.fn(() => true),
  useOpenCareGroupMode: vi.fn(() => false),
  usePresenceMode: vi.fn(() => "detailed"),
  useTenantSlugSafe: vi.fn(() => "test-tenant"),
  useTenantRoutingModeSafe: vi.fn(() => "path"),
}));

vi.mock("~/lib/dashboard-helpers", () => ({
  formatRecentActivityTime: vi.fn((timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString("de-DE", {
      hour: "2-digit",
      minute: "2-digit",
    });
  }),
  getActivityStatusColor: vi.fn(() => "bg-green-500"),
  getGroupStatusColor: vi.fn(() => "bg-green-500"),
}));

import { useSession } from "next-auth/react";
import { isAdmin } from "~/lib/auth-utils";
import { hasEffectiveAdminScope } from "~/lib/auth-utils";
import { useSWRAuth } from "~/lib/swr/hooks";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
} from "~/lib/tenant-context";

// Helper to create SWR mock return values
function mockSWR(
  data: typeof mockDashboardData | undefined,
  options?: { isLoading?: boolean; error?: Error | null },
) {
  return {
    data,
    isLoading: options?.isLoading ?? false,
    error: options?.error ?? undefined,
    mutate: vi.fn(),
    isValidating: false,
  };
}

describe("DashboardPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(useSWRAuth).mockReturnValue(mockSWR(mockDashboardData));
    vi.mocked(useNFCEnabled).mockReturnValue(true);
    vi.mocked(useOpenCareGroupMode).mockReturnValue(false);
    vi.mocked(usePresenceMode).mockReturnValue("detailed");
  });

  it("renders dashboard for admin user", async () => {
    render(<DashboardPage />);

    expect(screen.getByTestId("user-context-provider")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText(/Test/)).toBeInTheDocument();
    });
  });

  it("shows access denied for non-admin users", () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    vi.mocked(useSession).mockReturnValue({
      data: { ...mockSession, user: { ...mockSession.user, isAdmin: false } },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    expect(screen.getByText("Kein Zugriff")).toBeInTheDocument();
  });

  it("does not load phase expiry warnings without admin permission", () => {
    vi.mocked(hasEffectiveAdminScope).mockReturnValue(false);

    render(<DashboardPage />);

    expect(
      screen.queryByTestId("phase-expiry-warnings"),
    ).not.toBeInTheDocument();
  });

  it("shows phase expiry warnings for a wildcard admin", () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    vi.mocked(hasEffectiveAdminScope).mockReturnValue(true);

    render(<DashboardPage />);

    expect(screen.getByTestId("phase-expiry-warnings")).toBeInTheDocument();
  });

  it("shows loading state when session is loading", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "loading",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    // RoleGuard intercepts session-loading and renders the page's skeleton
    // via its fallback prop.
    expect(screen.getByTestId("dashboard-skeleton")).toBeInTheDocument();
  });

  it("displays dashboard data after loading", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("150")).toBeInTheDocument(); // studentsPresent
      expect(screen.getByText("120")).toBeInTheDocument(); // studentsInRooms
      expect(screen.getByText("20")).toBeInTheDocument(); // studentsInTransit
      expect(screen.getByText("4")).toBeInTheDocument(); // studentsSick
      expect(screen.getByText("3")).toBeInTheDocument(); // studentsExcused
      expect(screen.getByText("33")).toBeInTheDocument(); // studentsHome
      expect(screen.getByText("10")).toBeInTheDocument(); // studentsOnPlayground
    });
  });

  it("displays stat cards with correct titles", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Kinder anwesend")).toBeInTheDocument();
      expect(screen.getByText("In Räumen")).toBeInTheDocument();
      expect(screen.getByText("Unterwegs")).toBeInTheDocument();
      expect(screen.getByText("Schulhof")).toBeInTheDocument();
      expect(screen.getByText("Krank")).toBeInTheDocument();
      expect(screen.getByText("Entschuldigt")).toBeInTheDocument();
      expect(screen.getByText("Zuhause")).toBeInTheDocument();
      expect(screen.getByText("Aktive Gruppen")).toBeInTheDocument();
      expect(screen.getByText("Aktive Aktivitäten")).toBeInTheDocument();
      expect(screen.queryByText("Freie Räume")).not.toBeInTheDocument();
      expect(screen.getByText("Auslastung")).toBeInTheDocument();
    });
  });

  it("displays capacity utilization as percentage", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("75%")).toBeInTheDocument();
    });
  });

  it("displays info cards", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Letzte Bewegungen")).toBeInTheDocument();
      expect(screen.getByText("Laufende Aktivitäten")).toBeInTheDocument();
      expect(screen.getByText("Aktive Gruppen")).toBeInTheDocument();
      expect(screen.queryByText("Personal heute")).not.toBeInTheDocument();
      expect(screen.queryByText("Kinder je Betreuer")).not.toBeInTheDocument();
      expect(screen.getByTestId("dashboard-stats-grid")).toHaveClass(
        "xl:grid-cols-4",
      );
      expect(screen.getByTestId("dashboard-info-grid")).toHaveClass(
        "xl:grid-cols-3",
      );
    });
  });

  it("hides group dashboard surfaces in open-care group mode", async () => {
    vi.mocked(useOpenCareGroupMode).mockReturnValue(true);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.queryByText("Aktive Gruppen")).not.toBeInTheDocument();
      expect(
        screen.queryByRole("link", { name: /Aktive Gruppen/i }),
      ).not.toBeInTheDocument();
      expect(screen.getByText("Kinder anwesend")).toBeInTheDocument();
      expect(screen.queryByText("Personal heute")).not.toBeInTheDocument();
    });
  });

  it("hides activity dashboard surfaces when NFC is disabled", async () => {
    vi.mocked(useNFCEnabled).mockReturnValue(false);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.queryByText("Aktive Aktivitäten")).not.toBeInTheDocument();
      expect(
        screen.queryByText("Laufende Aktivitäten"),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("link", { name: /Laufende Aktivitäten/i }),
      ).not.toBeInTheDocument();
      expect(screen.getByText("Aktive Gruppen")).toBeInTheDocument();
      expect(screen.queryByText("Freie Räume")).not.toBeInTheDocument();
      expect(screen.queryByText("Personal heute")).not.toBeInTheDocument();
      expect(screen.getByTestId("dashboard-info-grid")).toHaveClass(
        "xl:grid-cols-2",
      );
    });
  });

  it("hides room and activity dashboard surfaces in binary presence mode", async () => {
    vi.mocked(useNFCEnabled).mockReturnValue(true);
    vi.mocked(usePresenceMode).mockReturnValue("binary");

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.queryByText("Aktive Aktivitäten")).not.toBeInTheDocument();
      expect(
        screen.queryByText("Laufende Aktivitäten"),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByRole("link", { name: /Laufende Aktivitäten/i }),
      ).not.toBeInTheDocument();
      expect(screen.queryByText("In Räumen")).not.toBeInTheDocument();
      expect(screen.queryByText("Unterwegs")).not.toBeInTheDocument();
      expect(screen.queryByText("Freie Räume")).not.toBeInTheDocument();
      expect(screen.queryByText("Auslastung")).not.toBeInTheDocument();
      expect(screen.queryByText("Letzte Bewegungen")).not.toBeInTheDocument();
      expect(screen.getByText("Aktive Gruppen")).toBeInTheDocument();
      expect(screen.queryByText("Personal heute")).not.toBeInTheDocument();
      expect(screen.getByTestId("dashboard-info-grid")).toHaveClass(
        "xl:grid-cols-1",
      );
    });
  });

  it("shows error message when fetch fails", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR(undefined, { error: new Error("fetch failed") }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Laden der Dashboard-Daten"),
      ).toBeInTheDocument();
    });
  });

  it("redirects when session error is RefreshTokenExpired", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: { ...mockSession, error: "RefreshTokenExpired" },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(mockRedirect).toHaveBeenCalledWith("/test-tenant/");
    });
  });

  it("redirects when token is missing", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: { ...mockSession, user: { ...mockSession.user, token: undefined } },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(mockRedirect).toHaveBeenCalledWith("/test-tenant/");
    });
  });
});

describe("getTimeBasedGreeting", () => {
  it("returns correct greeting based on hour of day", () => {
    // We test the logic directly since the function is internal
    const getGreeting = (hour: number): string => {
      if (hour < 12) return "Guten Morgen";
      if (hour < 17) return "Guten Tag";
      return "Guten Abend";
    };

    expect(getGreeting(8)).toBe("Guten Morgen");
    expect(getGreeting(11)).toBe("Guten Morgen");
    expect(getGreeting(12)).toBe("Guten Tag");
    expect(getGreeting(16)).toBe("Guten Tag");
    expect(getGreeting(17)).toBe("Guten Abend");
    expect(getGreeting(22)).toBe("Guten Abend");
  });
});

describe("DashboardContent rendering states", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useNFCEnabled).mockReturnValue(true);
  });

  it("shows empty state for recent activity when no data", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({ ...mockDashboardData, recentActivity: [] }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine aktuellen Bewegungen"),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state for current activities when no data", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({ ...mockDashboardData, currentActivities: [] }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine laufenden Aktivitäten"),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state for active groups when no data", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({ ...mockDashboardData, activeGroupsSummary: [] }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine aktiven Gruppen")).toBeInTheDocument();
    });
  });

  it("does not display personnel metrics based on daily supervision", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(mockSWR(mockDashboardData));

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.queryByText("Personal heute")).not.toBeInTheDocument();
      expect(screen.queryByText("Betreuer im Dienst")).not.toBeInTheDocument();
      expect(screen.queryByText("Kinder je Betreuer")).not.toBeInTheDocument();
      expect(screen.queryByText("Betreuungsschlüssel")).not.toBeInTheDocument();
    });
  });

  it("extracts first name from session for greeting", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        ...mockSession,
        user: { ...mockSession.user, name: "John Doe" },
      },
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useSWRAuth).mockReturnValue(mockSWR(mockDashboardData));

    render(<DashboardPage />);

    await waitFor(() => {
      // Should show "John" (first part of "John Doe")
      expect(screen.getByText(/John/)).toBeInTheDocument();
    });
  });

  it("displays multiple recent activities", async () => {
    const multipleActivities = [
      {
        type: "checkin",
        groupName: "Gruppe 1",
        roomName: "Raum 101",
        timestamp: new Date().toISOString(),
        count: 5,
      },
      {
        type: "checkout",
        groupName: "Gruppe 2",
        roomName: "Raum 102",
        timestamp: new Date().toISOString(),
        count: 1,
      },
    ];

    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({ ...mockDashboardData, recentActivity: multipleActivities }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppe 1")).toBeInTheDocument();
      expect(screen.getByText("Gruppe 2")).toBeInTheDocument();
      expect(screen.getByText("5 Kinder")).toBeInTheDocument();
    });
  });

  it("displays multiple current activities with status", async () => {
    const multipleCurrentActivities = [
      {
        id: "201",
        name: "Schach",
        category: "Sport",
        participants: 8,
        maxCapacity: 10,
        status: "active",
      },
      {
        id: "202",
        name: "Kunst",
        category: "Kreativ",
        participants: 12,
        maxCapacity: 15,
        status: "full",
      },
    ];

    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({
        ...mockDashboardData,
        currentActivities: multipleCurrentActivities,
      }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Schach")).toBeInTheDocument();
      expect(screen.getByText("Kunst")).toBeInTheDocument();
      expect(screen.getByText(/Sport • 8\/10 Teilnehmer/)).toBeInTheDocument();
    });
  });

  it("displays multiple active groups with status", async () => {
    const multipleGroups = [
      {
        type: "ogs",
        name: "OGS Gruppe A",
        location: "Raum 101",
        studentCount: 15,
        status: "active",
      },
      {
        type: "activity",
        name: "Schach AG",
        location: "Raum 205",
        studentCount: 8,
        status: "full",
      },
    ];

    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({ ...mockDashboardData, activeGroupsSummary: multipleGroups }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("OGS Gruppe A")).toBeInTheDocument();
      expect(screen.getByText("Schach AG")).toBeInTheDocument();
      expect(screen.getByText(/Raum 101 • 15 Kinder/)).toBeInTheDocument();
    });
  });

  it("defaults to User when session name is missing", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        ...mockSession,
        user: { ...mockSession.user, name: undefined },
      },
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useSWRAuth).mockReturnValue(mockSWR(mockDashboardData));

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText(/User/)).toBeInTheDocument();
    });
  });
});

describe("StatCard component behavior", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useNFCEnabled).mockReturnValue(true);
  });

  it("renders stat cards as links when href is provided", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(mockSWR(mockDashboardData));

    render(<DashboardPage />);

    await waitFor(() => {
      // Students link
      const studentsLink = screen.getByRole("link", {
        name: /Kinder anwesend/i,
      });
      expect(studentsLink).toHaveAttribute("href", "/students/search");
    });
  });

  it("renders website-style Phosphor duotone icons", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(mockSWR(mockDashboardData));

    render(<DashboardPage />);

    const studentsLink = await screen.findByRole("link", {
      name: /Kinder anwesend/i,
    });
    const icon = studentsLink.querySelector("svg");
    const iconWrapper = icon?.parentElement;

    expect(icon).toHaveAttribute("data-moto-duotone-tone", "greenDeep");
    expect(icon).toHaveStyle({ color: "#3F6F12" });
    expect(icon?.querySelector('[opacity="0.2"]')).toBeInTheDocument();
    expect(iconWrapper).not.toHaveClass("bg-gradient-to-br");
  });

  it("uses distinct semantic dashboard colors", async () => {
    render(<DashboardPage />);

    const rooms = await screen.findByRole("link", { name: /In Räumen/i });
    const schoolyard = screen.getByRole("link", { name: /Schulhof/i });
    const sick = screen.getByRole("link", { name: /Krank/i });
    const excused = screen.getByRole("link", { name: /Entschuldigt/i });
    const utilization = screen
      .getByText("Auslastung")
      .closest(".moto-content-surface");

    expect(rooms.querySelector("svg")).toHaveAttribute(
      "data-moto-duotone-tone",
      "navy",
    );
    // Orange, matching LOCATION_COLORS.SCHOOLYARD — the same status renders as
    // an orange pill in the student list.
    expect(schoolyard.querySelector("svg")).toHaveAttribute(
      "data-moto-duotone-tone",
      "orange",
    );
    expect(sick.querySelector("svg")).toHaveAttribute(
      "data-moto-duotone-tone",
      "red",
    );
    expect(excused.querySelector("svg")).toHaveAttribute(
      "data-moto-duotone-tone",
      "purple",
    );
    // Gold: orange belongs to the Schulhof status, and Auslastung is a metric
    // with no badge hue of its own.
    expect(utilization?.querySelector("svg")).toHaveAttribute(
      "data-moto-duotone-tone",
      "gold",
    );
  });

  it("renders stat cards with loading state showing dots", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR(undefined, { isLoading: true }),
    );

    render(<DashboardPage />);

    // During loading, stat cards should show "…" for values
    await waitFor(() => {
      const dots = screen.getAllByText("…");
      expect(dots.length).toBeGreaterThan(0);
    });
  });
});

describe("InfoCard component behavior", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(useSWRAuth).mockReturnValue(mockSWR(mockDashboardData));
    vi.mocked(useNFCEnabled).mockReturnValue(true);
    vi.mocked(usePresenceMode).mockReturnValue("detailed");
  });

  it("renders info cards as links when href is provided", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      const activitiesLink = screen.getByRole("link", {
        name: /Laufende Aktivitäten/i,
      });
      expect(activitiesLink).toHaveAttribute("href", "/activities");
    });
  });

  it("uses Phosphor concept icons for dashboard info cards", async () => {
    render(<DashboardPage />);

    const expectedTones = [
      ["Letzte Bewegungen", "stone"],
      ["Laufende Aktivitäten", "coral"],
      ["Aktive Gruppen", "greenDeep"],
    ] as const;

    for (const [title, tone] of expectedTones) {
      const card = (await screen.findByText(title)).closest(
        ".moto-content-surface",
      );
      expect(card?.querySelector("svg")).toHaveAttribute(
        "data-moto-duotone-tone",
        tone,
      );
    }
  });
});

describe("Activity timestamp key handling", () => {
  it("handles invalid timestamp in activity key", () => {
    // Test the key generation logic
    const activities = [
      {
        type: "checkin",
        groupName: "Group A",
        roomName: "Room 1",
        timestamp: "invalid-date",
        count: 3,
      },
    ];

    const generateKey = (activity: (typeof activities)[0], idx: number) => {
      const ts = new Date(activity.timestamp).getTime();
      const tsKey = Number.isFinite(ts) ? ts : `idx-${idx}`;
      return `${activity.type}-${activity.groupName}-${activity.roomName}-${tsKey}`;
    };

    const key = generateKey(activities[0]!, 0);
    expect(key).toBe("checkin-Group A-Room 1-idx-0");
  });

  it("handles valid timestamp in activity key", () => {
    const validTimestamp = new Date("2024-01-15T10:30:00Z").toISOString();
    const activities = [
      {
        type: "checkout",
        groupName: "Group B",
        roomName: "Room 2",
        timestamp: validTimestamp,
        count: 1,
      },
    ];

    const generateKey = (activity: (typeof activities)[0], idx: number) => {
      const ts = new Date(activity.timestamp).getTime();
      const tsKey = Number.isFinite(ts) ? ts : `idx-${idx}`;
      return `${activity.type}-${activity.groupName}-${activity.roomName}-${tsKey}`;
    };

    const key = generateKey(activities[0]!, 0);
    expect(key).toContain("checkout-Group B-Room 2-");
    expect(key).not.toContain("idx-");
  });
});

// Geburtstagskarte (#1542). The card reads its own SWR key, so these tests
// answer the mock per key instead of returning the analytics payload for both.
describe("DashboardPage Geburtstage", () => {
  function mockKeyedSWR(birthdays: unknown) {
    vi.mocked(useSWRAuth).mockImplementation(((key: string) =>
      key === "birthday-overview"
        ? {
            data: birthdays,
            isLoading: false,
            error: undefined,
            mutate: vi.fn(),
            isValidating: false,
          }
        : mockSWR(mockDashboardData)) as unknown as typeof useSWRAuth);
  }

  beforeEach(() => {
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(isAdmin).mockReturnValue(true);
  });

  it("renders the birthday card with the celebrating names", async () => {
    mockKeyedSWR({
      enabled: true,
      includeStaff: false,
      today: "2026-08-03",
      celebrations: [
        {
          kind: "student",
          id: "1",
          name: "Lina Adler",
          groupName: "Delfine",
          schoolClass: "1a",
          date: "2026-08-03",
          age: 8,
          isToday: true,
        },
      ],
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Geburtstage")).toBeInTheDocument();
    });
    expect(screen.getByText("Lina Adler")).toBeInTheDocument();
  });

  it("hides the card entirely when the school switched the display off", async () => {
    mockKeyedSWR({
      enabled: false,
      includeStaff: false,
      today: "2026-08-03",
      celebrations: [],
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument();
    });
    expect(screen.queryByText("Geburtstage")).not.toBeInTheDocument();
  });

  // A failing birthday fetch must not take the dashboard down with it.
  it("keeps the dashboard usable when the birthday fetch returns nothing", async () => {
    mockKeyedSWR(undefined);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByTestId("user-context-provider")).toBeInTheDocument();
    });
    expect(screen.queryByText("Geburtstage")).not.toBeInTheDocument();
  });
});
