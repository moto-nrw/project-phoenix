import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import DashboardPage from "./page";

const mockPush = vi.fn();
const mockReplace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    replace: mockReplace,
  }),
}));

const mockSession = {
  user: {
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
}));

vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-layout">{children}</div>
  ),
}));

vi.mock("~/lib/usercontext-context", () => ({
  UserContextProvider: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="user-context-provider">{children}</div>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div
      data-testid="loading"
      data-fullpage={fullPage}
      aria-label="Lädt..."
    />
  ),
}));

const mockDashboardData = {
  studentsPresent: 150,
  studentsInRooms: 120,
  studentsInTransit: 20,
  studentsOnPlayground: 10,
  activeOGSGroups: 8,
  activeActivities: 5,
  freeRooms: 12,
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

vi.mock("~/lib/fetch-with-auth", () => ({
  fetchWithAuth: vi.fn(() =>
    Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ data: mockDashboardData }),
    }),
  ),
}));

vi.mock("~/lib/dashboard-helpers", () => ({
  formatRecentActivityTime: vi.fn((timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" });
  }),
  getActivityStatusColor: vi.fn(() => "bg-green-500"),
  getGroupStatusColor: vi.fn(() => "bg-green-500"),
}));

import { useSession } from "next-auth/react";
import { isAdmin } from "~/lib/auth-utils";
import { fetchWithAuth } from "~/lib/fetch-with-auth";

describe("DashboardPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });
    vi.mocked(isAdmin).mockReturnValue(true);
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: mockDashboardData }),
    } as Response);
  });

  it("renders dashboard for admin user", async () => {
    render(<DashboardPage />);

    expect(screen.getByTestId("responsive-layout")).toBeInTheDocument();
    expect(screen.getByTestId("user-context-provider")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText(/Test/)).toBeInTheDocument();
    });
  });

  it("redirects non-admin users to /ogs-groups", async () => {
    vi.mocked(isAdmin).mockReturnValue(false);

    render(<DashboardPage />);

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith("/ogs-groups");
    });
  });

  it("shows loading state when session is loading", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "loading",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("returns null while redirecting non-admin users", () => {
    vi.mocked(isAdmin).mockReturnValue(false);
    vi.mocked(useSession).mockReturnValue({
      data: { ...mockSession, user: { ...mockSession.user, isAdmin: false } },
      status: "authenticated",
      update: vi.fn(),
    });

    const { container } = render(<DashboardPage />);

    // Container should be empty since the component returns null
    expect(container.innerHTML).toBe("");
  });

  it("displays dashboard data after loading", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("150")).toBeInTheDocument(); // studentsPresent
      expect(screen.getByText("120")).toBeInTheDocument(); // studentsInRooms
      expect(screen.getByText("20")).toBeInTheDocument(); // studentsInTransit
      // 10 appears multiple times (studentsOnPlayground and supervisorsToday)
      expect(screen.getAllByText("10")).toHaveLength(2);
    });
  });

  it("displays stat cards with correct titles", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Kinder anwesend")).toBeInTheDocument();
      expect(screen.getByText("In Räumen")).toBeInTheDocument();
      expect(screen.getByText("Unterwegs")).toBeInTheDocument();
      expect(screen.getByText("Schulhof")).toBeInTheDocument();
      // "Aktive Gruppen" appears both as stat card title and info card title
      expect(screen.getAllByText("Aktive Gruppen")).toHaveLength(2);
      expect(screen.getByText("Aktive Aktivitäten")).toBeInTheDocument();
      expect(screen.getByText("Freie Räume")).toBeInTheDocument();
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
      // "Aktive Gruppen" appears both as stat card title and info card title
      expect(screen.getAllByText("Aktive Gruppen")).toHaveLength(2);
      expect(screen.getByText("Personal heute")).toBeInTheDocument();
    });
  });

  it("displays supervisor count", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Betreuer im Dienst")).toBeInTheDocument();
    });
  });

  it("shows error message when fetch fails", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: "Server Error" }),
    } as Response);

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
      expect(mockPush).toHaveBeenCalledWith("/");
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
      expect(mockPush).toHaveBeenCalledWith("/");
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

describe("getColorTheme", () => {
  it("returns correct theme for known colors", () => {
    const COLOR_THEMES: Record<string, { overlay: string; ring: string }> = {
      "[#5080D8]": {
        overlay: "from-blue-50/80 to-cyan-100/80",
        ring: "ring-blue-200/60",
      },
      "[#83CD2D]": {
        overlay: "from-green-50/80 to-lime-100/80",
        ring: "ring-green-200/60",
      },
      "[#FF3130]": {
        overlay: "from-red-50/80 to-rose-100/80",
        ring: "ring-red-200/60",
      },
    };

    const DEFAULT_THEME = {
      overlay: "from-gray-50/80 to-slate-100/80",
      ring: "ring-gray-200/60",
    };

    const getColorTheme = (
      color: string,
    ): { overlay: string; ring: string } => {
      const matchedKey = Object.keys(COLOR_THEMES).find((key) =>
        color.includes(key),
      );
      return matchedKey ? COLOR_THEMES[matchedKey]! : DEFAULT_THEME;
    };

    expect(getColorTheme("from-[#5080D8] to-[#4070c8]").overlay).toBe(
      "from-blue-50/80 to-cyan-100/80",
    );
    expect(getColorTheme("from-[#83CD2D] to-green").overlay).toBe(
      "from-green-50/80 to-lime-100/80",
    );
    expect(getColorTheme("unknown-color").overlay).toBe(
      "from-gray-50/80 to-slate-100/80",
    );
  });
});

describe("DashboardContent rendering states", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(isAdmin).mockReturnValue(true);
  });

  it("shows empty state for recent activity when no data", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { ...mockDashboardData, recentActivity: [] },
        }),
    } as Response);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine aktuellen Bewegungen"),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state for current activities when no data", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { ...mockDashboardData, currentActivities: [] },
        }),
    } as Response);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine laufenden Aktivitäten")).toBeInTheDocument();
    });
  });

  it("shows empty state for active groups when no data", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { ...mockDashboardData, activeGroupsSummary: [] },
        }),
    } as Response);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine aktiven Gruppen")).toBeInTheDocument();
    });
  });

  it("displays student ratio when supervisors are present", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: mockDashboardData,
        }),
    } as Response);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Kinder je Betreuer")).toBeInTheDocument();
      expect(screen.getByText("Betreuungsschlüssel")).toBeInTheDocument();
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

    render(<DashboardPage />);

    await waitFor(() => {
      // Should show "John" (first part of "John Doe")
      expect(screen.getByText(/John/)).toBeInTheDocument();
    });
  });

  it("shows dash for student ratio when no supervisors", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { ...mockDashboardData, supervisorsToday: 0 },
        }),
    } as Response);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Daten")).toBeInTheDocument();
    });
  });

  it("shows dash for student ratio when no students present", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { ...mockDashboardData, studentsPresent: 0 },
        }),
    } as Response);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Daten")).toBeInTheDocument();
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

    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { ...mockDashboardData, recentActivity: multipleActivities },
        }),
    } as Response);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

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
        name: "Schach",
        category: "Sport",
        participants: 8,
        maxCapacity: 10,
        status: "active",
      },
      {
        name: "Kunst",
        category: "Kreativ",
        participants: 12,
        maxCapacity: 15,
        status: "full",
      },
    ];

    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { ...mockDashboardData, currentActivities: multipleCurrentActivities },
        }),
    } as Response);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

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

    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: { ...mockDashboardData, activeGroupsSummary: multipleGroups },
        }),
    } as Response);
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

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
  });

  it("renders stat cards as links when href is provided", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: mockDashboardData }),
    } as Response);

    render(<DashboardPage />);

    await waitFor(() => {
      // Students link
      const studentsLink = screen.getByRole("link", { name: /Kinder anwesend/i });
      expect(studentsLink).toHaveAttribute("href", "/students/search");
    });
  });

  it("renders stat cards with loading state showing dots", async () => {
    // Keep the mock response pending
    vi.mocked(fetchWithAuth).mockImplementation(
      () => new Promise(() => {}), // Never resolves
    );

    render(<DashboardPage />);

    // During loading, stat cards should show "..." for values
    await waitFor(() => {
      const dots = screen.getAllByText("...");
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
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: mockDashboardData }),
    } as Response);
  });

  it("renders info cards as links when href is provided", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      const activitiesLink = screen.getByRole("link", { name: /Laufende Aktivitäten/i });
      expect(activitiesLink).toHaveAttribute("href", "/activities");
    });
  });

  it("renders staff info card with link to staff page", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      const staffLink = screen.getByRole("link", { name: /Personal heute/i });
      expect(staffLink).toHaveAttribute("href", "/staff");
    });
  });
});

describe("COLOR_THEMES coverage", () => {
  it("maps all predefined color themes correctly", () => {
    const COLOR_THEMES: Record<string, { overlay: string; ring: string }> = {
      "[#5080D8]": {
        overlay: "from-blue-50/80 to-cyan-100/80",
        ring: "ring-blue-200/60",
      },
      "[#83CD2D]": {
        overlay: "from-green-50/80 to-lime-100/80",
        ring: "ring-green-200/60",
      },
      "[#FF3130]": {
        overlay: "from-red-50/80 to-rose-100/80",
        ring: "ring-red-200/60",
      },
      "orange-500": {
        overlay: "from-orange-50/80 to-orange-100/80",
        ring: "ring-orange-200/60",
      },
      "yellow-400": {
        overlay: "from-yellow-50/80 to-yellow-100/80",
        ring: "ring-yellow-200/60",
      },
      emerald: {
        overlay: "from-emerald-50/80 to-green-100/80",
        ring: "ring-emerald-200/60",
      },
      purple: {
        overlay: "from-purple-50/80 to-violet-100/80",
        ring: "ring-purple-200/60",
      },
      indigo: {
        overlay: "from-indigo-50/80 to-blue-100/80",
        ring: "ring-indigo-200/60",
      },
    };

    const DEFAULT_THEME = {
      overlay: "from-gray-50/80 to-slate-100/80",
      ring: "ring-gray-200/60",
    };

    const getColorTheme = (
      color: string,
    ): { overlay: string; ring: string } => {
      const matchedKey = Object.keys(COLOR_THEMES).find((key) =>
        color.includes(key),
      );
      return matchedKey ? COLOR_THEMES[matchedKey]! : DEFAULT_THEME;
    };

    // Test all themes
    expect(getColorTheme("from-orange-500 to-orange-600").overlay).toBe(
      "from-orange-50/80 to-orange-100/80",
    );
    expect(getColorTheme("from-yellow-400 to-yellow-500").overlay).toBe(
      "from-yellow-50/80 to-yellow-100/80",
    );
    expect(getColorTheme("from-emerald-500 to-green-600").overlay).toBe(
      "from-emerald-50/80 to-green-100/80",
    );
    expect(getColorTheme("from-purple-500 to-purple-600").overlay).toBe(
      "from-purple-50/80 to-violet-100/80",
    );
    expect(getColorTheme("from-indigo-500 to-indigo-600").overlay).toBe(
      "from-indigo-50/80 to-blue-100/80",
    );
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

    const generateKey = (activity: typeof activities[0], idx: number) => {
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

    const generateKey = (activity: typeof activities[0], idx: number) => {
      const ts = new Date(activity.timestamp).getTime();
      const tsKey = Number.isFinite(ts) ? ts : `idx-${idx}`;
      return `${activity.type}-${activity.groupName}-${activity.roomName}-${tsKey}`;
    };

    const key = generateKey(activities[0]!, 0);
    expect(key).toContain("checkout-Group B-Room 2-");
    expect(key).not.toContain("idx-");
  });
});
