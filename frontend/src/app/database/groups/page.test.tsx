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

vi.mock("@/components/groups", () => ({
  GroupCreateModal: ({
    isOpen,
    onClose,
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="group-create-modal">
        <button
          data-testid="submit-create"
          onClick={() => void onCreate({ name: "New Group" })}
        >
          Submit
        </button>
        <button data-testid="close-create-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  GroupDetailModal: ({
    isOpen,
    group,
    onClose,
    onEdit,
    onDelete,
  }: {
    isOpen: boolean;
    group: { name: string } | null;
    onClose: () => void;
    onEdit: () => void;
    onDelete: () => void;
  }) =>
    isOpen && group ? (
      <div data-testid="group-detail-modal">
        <span data-testid="detail-group-name">{group.name}</span>
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
  GroupEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="group-edit-modal">
        <button
          data-testid="submit-edit"
          onClick={() => void onSave({ name: "Updated Group" })}
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

    // Setup getOne to return the selected group
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockGroups.find((g) => g.id === id)),
    );
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

  it("opens create modal when create button is clicked", async () => {
    render(<GroupsPage />);

    const createButton = screen.getByLabelText("Gruppe erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("group-create-modal")).toBeInTheDocument();
    });
  });

  it("closes create modal when close is clicked", async () => {
    render(<GroupsPage />);

    const createButton = screen.getByLabelText("Gruppe erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("group-create-modal")).toBeInTheDocument();
    });

    const closeButton = screen.getByTestId("close-create-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(
        screen.queryByTestId("group-create-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("opens detail modal when group row is clicked", async () => {
    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppe A")).toBeInTheDocument();
    });

    const groupRow = screen.getByText("Gruppe A").closest("button");
    if (groupRow) {
      fireEvent.click(groupRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("group-detail-modal")).toBeInTheDocument();
      expect(screen.getByTestId("detail-group-name")).toHaveTextContent(
        "Gruppe A",
      );
    });
  });

  it("opens edit modal when edit button is clicked in detail modal", async () => {
    render(<GroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppe A")).toBeInTheDocument();
    });

    const groupRow = screen.getByText("Gruppe A").closest("button");
    if (groupRow) {
      fireEvent.click(groupRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("group-detail-modal")).toBeInTheDocument();
    });

    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("group-edit-modal")).toBeInTheDocument();
    });
  });

  it("clears all filters when clear button is clicked", async () => {
    render(<GroupsPage />);

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
    mockCreate.mockResolvedValueOnce({ id: "3", name: "New Group" });

    render(<GroupsPage />);

    // Open create modal
    const createButton = screen.getByLabelText("Gruppe erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("group-create-modal")).toBeInTheDocument();
    });

    // Submit the form
    const submitButton = screen.getByTestId("submit-create");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
  });

  it("calls update service when saving edit form", async () => {
    mockUpdate.mockResolvedValueOnce({ id: "1", name: "Updated Group" });

    render(<GroupsPage />);

    // Select a group to open detail modal
    await waitFor(() => {
      expect(screen.getByText("Gruppe A")).toBeInTheDocument();
    });

    const groupRow = screen.getByText("Gruppe A").closest("button");
    if (groupRow) {
      fireEvent.click(groupRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("group-detail-modal")).toBeInTheDocument();
    });

    // Click edit button
    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("group-edit-modal")).toBeInTheDocument();
    });

    // Submit edit form
    const submitButton = screen.getByTestId("submit-edit");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  it("calls delete service when deleting a group", async () => {
    mockDelete.mockResolvedValueOnce({});

    render(<GroupsPage />);

    // Select a group to open detail modal
    await waitFor(() => {
      expect(screen.getByText("Gruppe A")).toBeInTheDocument();
    });

    const groupRow = screen.getByText("Gruppe A").closest("button");
    if (groupRow) {
      fireEvent.click(groupRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("group-detail-modal")).toBeInTheDocument();
    });

    // Click delete button
    const deleteButton = screen.getByTestId("delete-button");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalled();
    });
  });
});
