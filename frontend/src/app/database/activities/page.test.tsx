import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ActivitiesPage from "./page";

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

vi.mock("@/components/activities", () => ({
  ActivityCreateModal: ({
    isOpen,
    onClose,
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="activity-create-modal">
        <button
          data-testid="submit-create"
          onClick={() => void onCreate({ name: "New Activity" })}
        >
          Submit
        </button>
        <button data-testid="close-create-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  ActivityDetailModal: ({
    isOpen,
    activity,
    onClose,
    onEdit,
    onDelete,
  }: {
    isOpen: boolean;
    activity: { name: string } | null;
    onClose: () => void;
    onEdit: () => void;
    onDelete: () => void;
  }) =>
    isOpen && activity ? (
      <div data-testid="activity-detail-modal">
        <span data-testid="detail-activity-name">{activity.name}</span>
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
  ActivityEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="activity-edit-modal">
        <button
          data-testid="submit-edit"
          onClick={() => void onSave({ name: "Updated Activity" })}
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

const mockActivities = [
  {
    id: "1",
    name: "Fußball AG",
    category_name: "Sport",
    description: "Fußball für alle",
    max_participants: 20,
  },
  {
    id: "2",
    name: "Chor",
    category_name: "Musik",
    description: "Singen macht Spaß",
    max_participants: 30,
  },
];

describe("ActivitiesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockActivities,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    // Setup getOne to return the selected activity
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockActivities.find((a) => a.id === id)),
    );
  });

  it("renders the page with activities data", async () => {
    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByText("Fußball AG")).toBeInTheDocument();
      expect(screen.getByText("Chor")).toBeInTheDocument();
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

    render(<ActivitiesPage />);

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

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Aktivitäten/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no activities exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Aktivitäten vorhanden"),
      ).toBeInTheDocument();
    });
  });

  it("filters activities by search term", async () => {
    render(<ActivitiesPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Fußball" } });

    await waitFor(() => {
      expect(screen.getByText("Fußball AG")).toBeInTheDocument();
      expect(screen.queryByText("Chor")).not.toBeInTheDocument();
    });
  });

  it("displays category badges for activities", async () => {
    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByText("Sport")).toBeInTheDocument();
      expect(screen.getByText("Musik")).toBeInTheDocument();
    });
  });

  it("opens create modal when create button is clicked", async () => {
    render(<ActivitiesPage />);

    const createButton = screen.getByLabelText("Aktivität erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("activity-create-modal")).toBeInTheDocument();
    });
  });

  it("opens detail modal when activity row is clicked", async () => {
    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByText("Fußball AG")).toBeInTheDocument();
    });

    const activityRow = screen.getByText("Fußball AG").closest("button");
    if (activityRow) {
      fireEvent.click(activityRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-modal")).toBeInTheDocument();
      expect(screen.getByTestId("detail-activity-name")).toHaveTextContent(
        "Fußball AG",
      );
    });
  });

  it("opens edit modal when edit button is clicked in detail modal", async () => {
    render(<ActivitiesPage />);

    await waitFor(() => {
      expect(screen.getByText("Fußball AG")).toBeInTheDocument();
    });

    const activityRow = screen.getByText("Fußball AG").closest("button");
    if (activityRow) {
      fireEvent.click(activityRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-modal")).toBeInTheDocument();
    });

    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("activity-edit-modal")).toBeInTheDocument();
    });
  });

  it("clears all filters when clear button is clicked", async () => {
    render(<ActivitiesPage />);

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
    mockCreate.mockResolvedValueOnce({ id: "3", name: "New Activity" });

    render(<ActivitiesPage />);

    // Open create modal
    const createButton = screen.getByLabelText("Aktivität erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("activity-create-modal")).toBeInTheDocument();
    });

    // Submit the form
    const submitButton = screen.getByTestId("submit-create");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
  });

  it("calls update service when saving edit form", async () => {
    mockUpdate.mockResolvedValueOnce({ id: "1", name: "Updated Activity" });

    render(<ActivitiesPage />);

    // Select an activity to open detail modal
    await waitFor(() => {
      expect(screen.getByText("Fußball AG")).toBeInTheDocument();
    });

    const activityRow = screen.getByText("Fußball AG").closest("button");
    if (activityRow) {
      fireEvent.click(activityRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-modal")).toBeInTheDocument();
    });

    // Click edit button
    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("activity-edit-modal")).toBeInTheDocument();
    });

    // Submit edit form
    const submitButton = screen.getByTestId("submit-edit");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  it("calls delete service when deleting an activity", async () => {
    mockDelete.mockResolvedValueOnce({});

    render(<ActivitiesPage />);

    // Select an activity to open detail modal
    await waitFor(() => {
      expect(screen.getByText("Fußball AG")).toBeInTheDocument();
    });

    const activityRow = screen.getByText("Fußball AG").closest("button");
    if (activityRow) {
      fireEvent.click(activityRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("activity-detail-modal")).toBeInTheDocument();
    });

    // Click delete button
    const deleteButton = screen.getByTestId("delete-button");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalled();
    });
  });
});
