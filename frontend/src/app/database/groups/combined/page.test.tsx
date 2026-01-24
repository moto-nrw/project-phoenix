import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import CombinedGroupsPage from "./page";

// Mock auth-client
vi.mock("~/lib/auth-client", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "1", name: "Test User" } },
    isPending: false,
  })),
}));

// Mock next/navigation
const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({
    push: mockPush,
    back: vi.fn(),
  })),
}));

// Mock combinedGroupService
const mockGetCombinedGroups = vi.fn();
vi.mock("@/lib/api", () => ({
  combinedGroupService: {
    getCombinedGroups: () => mockGetCombinedGroups(),
  },
}));

// Mock UI components
vi.mock("@/components/dashboard", () => ({
  DataListPage: ({
    title,
    data,
    onSelectEntityAction,
    renderEntity,
  }: {
    title: string;
    data: Array<{
      id: string;
      name: string;
      is_active: boolean;
      is_expired?: boolean;
      access_policy: string;
    }>;
    onSelectEntityAction: (item: {
      id: string;
      name: string;
      is_active: boolean;
      is_expired?: boolean;
      access_policy: string;
    }) => void;
    renderEntity: (item: {
      id: string;
      name: string;
      is_active: boolean;
      is_expired?: boolean;
      access_policy: string;
    }) => React.ReactNode;
  }) => (
    <div data-testid="data-list-page">
      <h1>{title}</h1>
      {data.map((item) => (
        <button
          key={item.id}
          data-testid={`combined-group-${item.id}`}
          onClick={() => onSelectEntityAction(item)}
        >
          {renderEntity(item)}
        </button>
      ))}
    </div>
  ),
  ResponsiveLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="responsive-layout">{children}</div>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

// Import mocked modules
import { useSession } from "~/lib/auth-client";

const mockCombinedGroups = [
  {
    id: "1",
    name: "Combined Group A",
    is_active: true,
    is_expired: false,
    access_policy: "all",
    group_count: 3,
    time_until_expiration: "2 days",
  },
  {
    id: "2",
    name: "Combined Group B",
    is_active: true,
    is_expired: true,
    access_policy: "first",
    group_count: 2,
  },
  {
    id: "3",
    name: "Combined Group C",
    is_active: false,
    is_expired: false,
    access_policy: "specific",
  },
  {
    id: "4",
    name: "Combined Group D",
    is_active: false,
    is_expired: false,
    access_policy: "manual",
  },
];

describe("CombinedGroupsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetCombinedGroups.mockResolvedValue(mockCombinedGroups);
  });

  it("renders the page with combined groups data", async () => {
    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppenkombinationen")).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText("Combined Group A")).toBeInTheDocument();
      expect(screen.getByText("Combined Group B")).toBeInTheDocument();
    });
  });

  it("shows loading state when session is pending", () => {
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1", name: "Test" } },
      isPending: true,
    } as ReturnType<typeof useSession>);

    render(<CombinedGroupsPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("shows loading state while fetching data", async () => {
    // Create a promise that never resolves to keep loading state
    mockGetCombinedGroups.mockImplementation(() => new Promise(() => {}));

    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1", name: "Test" } },
      isPending: false,
    } as ReturnType<typeof useSession>);

    render(<CombinedGroupsPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("shows error message when fetch fails", async () => {
    mockGetCombinedGroups.mockRejectedValueOnce(new Error("Failed to fetch"));

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Gruppenkombinationen/),
      ).toBeInTheDocument();
    });
  });

  it("allows retry when error occurs", async () => {
    mockGetCombinedGroups.mockRejectedValueOnce(new Error("Failed to fetch"));

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Gruppenkombinationen/),
      ).toBeInTheDocument();
    });

    mockGetCombinedGroups.mockResolvedValueOnce(mockCombinedGroups);

    const retryButton = screen.getByText("Erneut versuchen");
    fireEvent.click(retryButton);

    await waitFor(() => {
      expect(mockGetCombinedGroups).toHaveBeenCalledTimes(2);
    });
  });

  it("navigates to detail page when combined group is selected", async () => {
    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Combined Group A")).toBeInTheDocument();
    });

    const combinedGroupButton = screen.getByTestId("combined-group-1");
    fireEvent.click(combinedGroupButton);

    expect(mockPush).toHaveBeenCalledWith("/database/groups/combined/1");
  });

  it("displays access policy labels correctly", async () => {
    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText(/Zugriffsmethode: Alle/)).toBeInTheDocument();
      expect(
        screen.getByText(/Zugriffsmethode: Erste Gruppe/),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Zugriffsmethode: Spezifische Gruppe/),
      ).toBeInTheDocument();
      expect(screen.getByText(/Zugriffsmethode: Manuell/)).toBeInTheDocument();
    });
  });

  it("displays active status badge", async () => {
    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
    });
  });

  it("displays expired status badge", async () => {
    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Abgelaufen")).toBeInTheDocument();
    });
  });

  it("displays group count when available", async () => {
    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText(/Gruppen: 3/)).toBeInTheDocument();
      expect(screen.getByText(/Gruppen: 2/)).toBeInTheDocument();
    });
  });

  it("displays time until expiration when available", async () => {
    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText(/Läuft ab in: 2 days/)).toBeInTheDocument();
    });
  });

  it("handles empty combined groups list", async () => {
    mockGetCombinedGroups.mockResolvedValueOnce([]);

    render(<CombinedGroupsPage />);

    // When there are no groups, the DataListPage still renders but with empty data
    await waitFor(() => {
      expect(screen.getByTestId("data-list-page")).toBeInTheDocument();
    });
  });

  it("redirects when not authenticated", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      isPending: false,
    } as ReturnType<typeof useSession>);

    // The component will call redirect synchronously
    expect(() => render(<CombinedGroupsPage />)).not.toThrow();
  });

  it("logs when empty data is returned from API", async () => {
    const consoleSpy = vi.spyOn(console, "log").mockImplementation(() => {});
    mockGetCombinedGroups.mockResolvedValueOnce([]);

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(consoleSpy).toHaveBeenCalledWith(
        "No combined groups returned from API, checking connection",
      );
    });

    consoleSpy.mockRestore();
  });
});
