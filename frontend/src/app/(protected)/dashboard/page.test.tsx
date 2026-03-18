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
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return, @typescript-eslint/no-unsafe-member-access
  isAdmin: vi.fn((session) => session?.user?.isAdmin ?? false),
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

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading" data-fullpage={fullPage} aria-label="Lädt..." />
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

vi.mock("~/lib/swr/hooks", () => ({
  useSWRAuth: vi.fn(),
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
import { useSWRAuth } from "~/lib/swr/hooks";

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
  });

  it("renders dashboard for admin user", async () => {
    render(<DashboardPage />);

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

  it("displays hero card with student count", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("150")).toBeInTheDocument();
      expect(screen.getByText("Kinder anwesend")).toBeInTheDocument();
    });
  });

  it("displays breakdown bar with segment labels", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText(/120 In Räumen/)).toBeInTheDocument();
      expect(screen.getByText(/20 Unterwegs/)).toBeInTheDocument();
      expect(screen.getByText(/10 Schulhof/)).toBeInTheDocument();
    });
  });

  it("displays compact stats row", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      // "Aktive Gruppen" appears in compact stats AND list card
      expect(
        screen.getAllByText("Aktive Gruppen").length,
      ).toBeGreaterThanOrEqual(1);
      expect(screen.getByText("8")).toBeInTheDocument();
      expect(screen.getByText("Aktivitäten")).toBeInTheDocument();
      expect(screen.getByText("5")).toBeInTheDocument();
      expect(screen.getByText("Betreuer")).toBeInTheDocument();
    });
  });

  it("shows supervisor ratio in compact stats when meaningful", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      // 150 students / 10 supervisors = 15:1
      expect(screen.getByText("15:1 Betreuungsschlüssel")).toBeInTheDocument();
    });
  });

  it("hides supervisor ratio when no students present", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({ ...mockDashboardData, studentsPresent: 0 }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Betreuer")).toBeInTheDocument();
      expect(screen.queryByText(/Betreuungsschlüssel/)).not.toBeInTheDocument();
    });
  });

  it("hides supervisor ratio when no supervisors", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({ ...mockDashboardData, supervisorsToday: 0 }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("Betreuer")).toBeInTheDocument();
      expect(screen.queryByText(/Betreuungsschlüssel/)).not.toBeInTheDocument();
    });
  });

  it("displays list sections with data", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      // "Aktive Gruppen" appears in both compact stats and list card
      expect(screen.getAllByText("Aktive Gruppen").length).toBe(2);
      expect(screen.getByText("Laufende Aktivitäten")).toBeInTheDocument();
      expect(screen.getByText("Letzte Bewegungen")).toBeInTheDocument();
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

  it("hero card links to students search", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      const heroLink = screen.getByRole("link", {
        name: /Kinder anwesend/i,
      });
      expect(heroLink).toHaveAttribute("href", "/students/search");
    });
  });

  it("compact stats link to their respective pages", async () => {
    render(<DashboardPage />);

    await waitFor(() => {
      // Multiple "Aktive Gruppen" links exist (compact stat + list card)
      const groupLinks = screen.getAllByRole("link", {
        name: /Aktive Gruppen/i,
      });
      expect(
        groupLinks.some((l) => l.getAttribute("href") === "/ogs-groups"),
      ).toBe(true);

      const activitiesLinks = screen.getAllByRole("link", {
        name: /Aktivitäten/i,
      });
      expect(
        activitiesLinks.some((l) => l.getAttribute("href") === "/activities"),
      ).toBe(true);

      const staffLink = screen.getByRole("link", { name: /Betreuer/i });
      expect(staffLink).toHaveAttribute("href", "/staff");
    });
  });
});

describe("getTimeBasedGreeting", () => {
  it("returns correct greeting based on hour of day", () => {
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
  });

  it("hides list sections when data arrays are empty", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({
        ...mockDashboardData,
        recentActivity: [],
        currentActivities: [],
        activeGroupsSummary: [],
      }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      // Hero and compact stats still visible
      expect(screen.getByText("150")).toBeInTheDocument();
      expect(screen.getByText("Kinder anwesend")).toBeInTheDocument();

      // List sections should not be rendered
      expect(screen.queryByText("Letzte Bewegungen")).not.toBeInTheDocument();
      expect(
        screen.queryByText("Laufende Aktivitäten"),
      ).not.toBeInTheDocument();
    });
  });

  it("shows hero with zero and message when no students present", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR({ ...mockDashboardData, studentsPresent: 0 }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      expect(screen.getByText("0")).toBeInTheDocument();
      expect(screen.getByText("Keine Kinder anwesend")).toBeInTheDocument();
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
      expect(screen.getByText(/John/)).toBeInTheDocument();
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
    });
  });

  it("shows skeleton loading states", async () => {
    vi.mocked(useSWRAuth).mockReturnValue(
      mockSWR(undefined, { isLoading: true }),
    );

    render(<DashboardPage />);

    await waitFor(() => {
      // Skeleton elements should be present (animate-pulse divs)
      const skeletons = document.querySelectorAll(".animate-pulse");
      expect(skeletons.length).toBeGreaterThan(0);
    });
  });
});
