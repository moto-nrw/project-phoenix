import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import GroupsPage from "./page";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "1", token: "test-token" }, expires: "2099-01-01" },
    status: "authenticated",
  })),
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

// Mock SWR hooks
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
}));

// Mock service factory
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: vi.fn(),
    getOne: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
  })),
}));

// Mock hooks
vi.mock("~/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock("~/hooks/useDeleteConfirmation", () => ({
  useDeleteConfirmation: vi.fn(() => ({
    showConfirmModal: false,
    handleDeleteClick: vi.fn(),
    handleDeleteCancel: vi.fn(),
    confirmDelete: vi.fn(),
  })),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: vi.fn(),
  })),
}));

// Mock UI components
vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
  }: {
    children: React.ReactNode;
    loading: boolean;
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
      {children}
    </div>
  ),
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    search,
    onClearAllFilters,
  }: {
    search: { value: string; onChange: (v: string) => void };
    onClearAllFilters: () => void;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
    </div>
  ),
}));

vi.mock("@/components/groups", () => ({
  GroupCreateModal: () => <div data-testid="group-create-modal" />,
  GroupDetailModal: () => <div data-testid="group-detail-modal" />,
  GroupEditModal: () => <div data-testid="group-edit-modal" />,
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: () => <div data-testid="confirmation-modal" />,
}));

// Import mocked modules
import { useSWRAuth } from "~/lib/swr";

const mockGroups = [
  {
    id: "1",
    name: "Gruppe A",
    room_name: "Raum 101",
    student_count: 15,
  },
  {
    id: "2",
    name: "Gruppe B",
    room_name: "Raum 102",
    student_count: 20,
  },
];

describe("GroupsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockGroups,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);
  });

  it("renders the page with groups data", async () => {
    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppe A")).toBeInTheDocument();
      expect(screen.getByText("Gruppe B")).toBeInTheDocument();
    });
  });

  it("shows loading state when data is loading", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<GroupsPage />);

    const layout = screen.getByTestId("database-layout");
    expect(layout).toHaveAttribute("data-loading", "true");
  });

  it("shows error message when fetch fails", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("Failed to fetch"),
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<GroupsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Gruppen/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no groups exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Gruppen vorhanden")).toBeInTheDocument();
    });
  });

  it("filters groups by search term", async () => {
    render(<GroupsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Gruppe A" } });

    await waitFor(() => {
      expect(screen.getByText("Gruppe A")).toBeInTheDocument();
      expect(screen.queryByText("Gruppe B")).not.toBeInTheDocument();
    });
  });

  it("displays room info for groups", async () => {
    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.getByText("Raum 102")).toBeInTheDocument();
    });
  });
});
