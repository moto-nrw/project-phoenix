import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import CombinedGroupsPage from "./page";

const mockRouterPush = vi.fn();

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: {
      user: {
        id: "1",
        name: "Test User",
        email: "test@test.com",
        token: "tok",
        roles: ["admin"],
      },
    },
    status: "authenticated",
  })),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockRouterPush,
    replace: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    refresh: vi.fn(),
    prefetch: vi.fn(),
  }),
  redirect: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    className,
    "aria-label": ariaLabel,
  }: {
    children: React.ReactNode;
    href: string;
    className?: string;
    "aria-label"?: string;
  }) => (
    <a href={href} className={className} aria-label={ariaLabel}>
      {children}
    </a>
  ),
}));

const mockGetCombinedGroups = vi.fn();
vi.mock("@/lib/api", () => ({
  combinedGroupService: {
    getCombinedGroups: () => mockGetCombinedGroups(),
  },
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

describe("CombinedGroupsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    mockGetCombinedGroups.mockReturnValue(new Promise(() => {}));
    render(<CombinedGroupsPage />);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders combined groups list after loading", async () => {
    mockGetCombinedGroups.mockResolvedValue([
      {
        id: "1",
        name: "Kombination A",
        is_active: true,
        is_expired: false,
        access_policy: "all",
        group_count: 3,
      },
    ]);

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppenkombinationen")).toBeInTheDocument();
      expect(screen.getByText("Kombination A")).toBeInTheDocument();
    });
  });

  it("shows active badge for active groups", async () => {
    mockGetCombinedGroups.mockResolvedValue([
      {
        id: "1",
        name: "Active Group",
        is_active: true,
        is_expired: false,
        access_policy: "all",
      },
    ]);

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
    });
  });

  it("shows expired badge for expired groups", async () => {
    mockGetCombinedGroups.mockResolvedValue([
      {
        id: "1",
        name: "Expired Group",
        is_active: true,
        is_expired: true,
        access_policy: "all",
      },
    ]);

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Abgelaufen")).toBeInTheDocument();
    });
  });

  it("filters combined groups by search term", async () => {
    mockGetCombinedGroups.mockResolvedValue([
      {
        id: "1",
        name: "Kombination A",
        is_active: true,
        is_expired: false,
        access_policy: "all",
      },
      {
        id: "2",
        name: "Feriengruppe",
        is_active: true,
        is_expired: false,
        access_policy: "specific",
      },
    ]);

    render(<CombinedGroupsPage />);

    await screen.findByText("Kombination A");

    const searchInput = screen.getAllByPlaceholderText("Suchen...")[0];
    if (!searchInput) throw new Error("Search input was not rendered");

    fireEvent.change(searchInput, { target: { value: "Ferien" } });

    expect(screen.getByText("Feriengruppe")).toBeInTheDocument();
    expect(screen.queryByText("Kombination A")).not.toBeInTheDocument();
  });

  it("shows no results message when search has no matches", async () => {
    mockGetCombinedGroups.mockResolvedValue([
      {
        id: "1",
        name: "Kombination A",
        is_active: true,
        is_expired: false,
        access_policy: "all",
      },
    ]);

    render(<CombinedGroupsPage />);

    await screen.findByText("Kombination A");

    const searchInput = screen.getAllByPlaceholderText("Suchen...")[0];
    if (!searchInput) throw new Error("Search input was not rendered");

    fireEvent.change(searchInput, { target: { value: "Nichts" } });

    expect(screen.getByText("Keine Ergebnisse gefunden.")).toBeInTheDocument();
  });

  it("links to the create page", async () => {
    mockGetCombinedGroups.mockResolvedValue([]);

    render(<CombinedGroupsPage />);

    await screen.findByText("Keine Einträge vorhanden.");

    const createLinks = screen.getAllByRole("link", {
      name: "Neue Kombination erstellen",
    });
    const createLink = createLinks[0];
    if (!createLink) throw new Error("Create link was not rendered");

    expect(createLink).toHaveAttribute("href", "/database/groups/combined/new");
  });

  it("navigates to the detail page when a group is selected", async () => {
    mockGetCombinedGroups.mockResolvedValue([
      {
        id: "1",
        name: "Kombination A",
        is_active: true,
        is_expired: false,
        access_policy: "all",
      },
    ]);

    render(<CombinedGroupsPage />);

    fireEvent.click(
      await screen.findByRole("button", { name: /Kombination A/ }),
    );

    expect(mockRouterPush).toHaveBeenCalledWith(
      "/test-tenant/database/groups/combined/1",
    );
  });

  it("shows error state on API failure", async () => {
    mockGetCombinedGroups.mockRejectedValue(new Error("API Error"));

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Fehler")).toBeInTheDocument();
      expect(screen.getByText("Erneut versuchen")).toBeInTheDocument();
    });
  });
});
