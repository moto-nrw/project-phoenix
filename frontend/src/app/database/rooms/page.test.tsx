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
const mockGetOne = vi.fn();
const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: vi.fn(),
    getOne: mockGetOne,
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
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
  RoomCreateModal: ({
    isOpen,
    onClose,
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="room-create-modal">
        <button
          data-testid="submit-create"
          onClick={() => void onCreate({ name: "New Room" })}
        >
          Submit
        </button>
        <button data-testid="close-create-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  RoomDetailModal: ({
    isOpen,
    room,
    onClose,
    onEdit,
    onDelete,
  }: {
    isOpen: boolean;
    room: { name: string } | null;
    onClose: () => void;
    onEdit: () => void;
    onDelete: () => void;
  }) =>
    isOpen && room ? (
      <div data-testid="room-detail-modal">
        <span data-testid="detail-room-name">{room.name}</span>
        <button data-testid="edit-button" onClick={onEdit}>
          Edit
        </button>
        <button data-testid="delete-button" onClick={onDelete}>
          Delete
        </button>
        <button data-testid="close-detail-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  RoomEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="room-edit-modal">
        <button
          data-testid="submit-edit"
          onClick={() => void onSave({ name: "Updated Room" })}
        >
          Save
        </button>
        <button data-testid="close-edit-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
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

    // Setup getOne to return the selected room
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockRooms.find((r) => r.id === id)),
    );
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

  it("opens create modal when create button is clicked", async () => {
    render(<RoomsPage />);

    const createButton = screen.getByLabelText("Raum erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    });
  });

  it("opens detail modal when room row is clicked", async () => {
    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
    });

    const roomRow = screen.getByText("Raum 101").closest("button");
    if (roomRow) {
      fireEvent.click(roomRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-modal")).toBeInTheDocument();
      expect(screen.getByTestId("detail-room-name")).toHaveTextContent(
        "Raum 101",
      );
    });
  });

  it("opens edit modal when edit button is clicked in detail modal", async () => {
    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
    });

    const roomRow = screen.getByText("Raum 101").closest("button");
    if (roomRow) {
      fireEvent.click(roomRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-modal")).toBeInTheDocument();
    });

    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("room-edit-modal")).toBeInTheDocument();
    });
  });

  it("clears all filters when clear button is clicked", async () => {
    render(<RoomsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "test" } });

    expect(searchInput).toHaveValue("test");

    const clearButton = screen.getByTestId("clear-filters");
    fireEvent.click(clearButton);

    await waitFor(() => {
      expect(searchInput).toHaveValue("");
    });
  });

  it("calls create service when submitting create form", async () => {
    mockCreate.mockResolvedValueOnce({ id: "3", name: "New Room" });

    render(<RoomsPage />);

    // Open create modal
    const createButton = screen.getByLabelText("Raum erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("room-create-modal")).toBeInTheDocument();
    });

    // Submit the form
    const submitButton = screen.getByTestId("submit-create");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
  });

  it("calls update service when saving edit form", async () => {
    mockUpdate.mockResolvedValueOnce({ id: "1", name: "Updated Room" });

    render(<RoomsPage />);

    // Select a room to open detail modal
    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
    });

    const roomRow = screen.getByText("Raum 101").closest("button");
    if (roomRow) {
      fireEvent.click(roomRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-modal")).toBeInTheDocument();
    });

    // Click edit button
    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("room-edit-modal")).toBeInTheDocument();
    });

    // Submit edit form
    const submitButton = screen.getByTestId("submit-edit");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  it("calls delete service when deleting a room", async () => {
    mockDelete.mockResolvedValueOnce({});

    render(<RoomsPage />);

    // Select a room to open detail modal
    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
    });

    const roomRow = screen.getByText("Raum 101").closest("button");
    if (roomRow) {
      fireEvent.click(roomRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-modal")).toBeInTheDocument();
    });

    // Click delete button
    const deleteButton = screen.getByTestId("delete-button");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalled();
    });
  });

  it("closes detail modal when close button is clicked", async () => {
    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
    });

    const roomRow = screen.getByText("Raum 101").closest("button");
    if (roomRow) {
      fireEvent.click(roomRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-modal")).toBeInTheDocument();
    });

    const closeButton = screen.getByTestId("close-detail-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(screen.queryByTestId("room-detail-modal")).not.toBeInTheDocument();
    });
  });

  it("closes edit modal when close button is clicked", async () => {
    render(<RoomsPage />);

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
    });

    const roomRow = screen.getByText("Raum 101").closest("button");
    if (roomRow) {
      fireEvent.click(roomRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("room-detail-modal")).toBeInTheDocument();
    });

    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("room-edit-modal")).toBeInTheDocument();
    });

    const closeButton = screen.getByTestId("close-edit-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(screen.queryByTestId("room-edit-modal")).not.toBeInTheDocument();
    });
  });

  it("shows not found message when search has no matches", async () => {
    render(<RoomsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "xyz123" } });

    await waitFor(() => {
      expect(screen.getByText("Keine Räume gefunden")).toBeInTheDocument();
    });
  });
});
