import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import RoomsPage from "./page";

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

vi.mock("@/components/rooms", () => ({
  RoomCreateModal: () => <div data-testid="room-create-modal" />,
  RoomDetailModal: () => <div data-testid="room-detail-modal" />,
  RoomEditModal: () => <div data-testid="room-edit-modal" />,
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: () => <div data-testid="confirmation-modal" />,
}));

// Import mocked modules
import { useSWRAuth } from "~/lib/swr";

const mockRooms = [
  {
    id: "1",
    name: "Raum 101",
    category: "Klassenzimmer",
    capacity: 30,
    building: "Hauptgebäude",
  },
  {
    id: "2",
    name: "Turnhalle",
    category: "Sport",
    capacity: 100,
    building: "Sporthalle",
  },
];

describe("RoomsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);
  });

  it("renders the page with rooms data", async () => {
    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.getByText("Turnhalle")).toBeInTheDocument();
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

    render(<RoomsPage />);

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

    render(<RoomsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Räume/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no rooms exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Räume vorhanden")).toBeInTheDocument();
    });
  });

  it("filters rooms by search term", async () => {
    render(<RoomsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "101" } });

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.queryByText("Turnhalle")).not.toBeInTheDocument();
    });
  });

  it("displays building info for rooms", async () => {
    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByText("Hauptgebäude")).toBeInTheDocument();
      expect(screen.getByText("Sporthalle")).toBeInTheDocument();
    });
  });
});
