import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StudentsPage from "./page";

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
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}));

// Mock SWR hooks
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
}));

// Mock service factory
const mockGetOne = vi.fn();
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: vi.fn(),
    getOne: mockGetOne,
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
    filters,
    onClearAllFilters,
  }: {
    search: { value: string; onChange: (v: string) => void };
    filters: Array<{
      id: string;
      value: string;
      onChange: (v: string) => void;
    }>;
    onClearAllFilters: () => void;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      {filters.map((f) => (
        <select
          key={f.id}
          data-testid={`filter-${f.id}`}
          value={f.value}
          onChange={(e) => f.onChange(e.target.value)}
        >
          <option value="all">All</option>
        </select>
      ))}
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
    </div>
  ),
}));

vi.mock("@/components/students/student-detail-modal", () => ({
  StudentDetailModal: ({
    isOpen,
    student,
    onClose,
    onEdit,
  }: {
    isOpen: boolean;
    student: { first_name: string; second_name: string } | null;
    onClose: () => void;
    onEdit: () => void;
  }) =>
    isOpen && student ? (
      <div data-testid="student-detail-modal">
        <span data-testid="detail-student-name">
          {student.first_name} {student.second_name}
        </span>
        <button data-testid="edit-button" onClick={onEdit}>
          Edit
        </button>
        <button data-testid="close-detail-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("@/components/students/student-edit-modal", () => ({
  StudentEditModal: ({
    isOpen,
    onClose,
  }: {
    isOpen: boolean;
    onClose: () => void;
  }) =>
    isOpen ? (
      <div data-testid="student-edit-modal">
        <button data-testid="close-edit-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("@/components/students/student-create-modal", () => ({
  StudentCreateModal: ({
    isOpen,
    onClose,
  }: {
    isOpen: boolean;
    onClose: () => void;
  }) =>
    isOpen ? (
      <div data-testid="student-create-modal">
        <button data-testid="close-create-modal" onClick={onClose}>
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

const mockStudents = [
  {
    id: "1",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "1a",
    group_id: "g1",
    group_name: "Gruppe A",
    name_lg: "Hans Mustermann",
  },
  {
    id: "2",
    first_name: "Anna",
    second_name: "Schmidt",
    school_class: "2b",
    group_id: "g2",
    group_name: "Gruppe B",
    name_lg: null,
  },
];

describe("StudentsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Default SWR mock - returns students data
    vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
      if (key === "database-students-list") {
        return {
          data: mockStudents,
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      }
      // Groups dropdown
      return {
        data: [
          { value: "g1", label: "Gruppe A" },
          { value: "g2", label: "Gruppe B" },
        ],
        isLoading: false,
        error: null,
        isValidating: false,
        mutate: vi.fn(),
      } as ReturnType<typeof useSWRAuth>;
    });

    // Setup getOne to return the selected student
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockStudents.find((s) => s.id === id)),
    );
  });

  it("renders the page with students data", async () => {
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
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

    render(<StudentsPage />);

    const layout = screen.getByTestId("database-layout");
    expect(layout).toHaveAttribute("data-loading", "true");
  });

  it("shows error message when fetch fails", async () => {
    vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
      if (key === "database-students-list") {
        return {
          data: undefined,
          isLoading: false,
          error: new Error("Failed to fetch"),
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      }
      return {
        data: [],
        isLoading: false,
        error: null,
        isValidating: false,
        mutate: vi.fn(),
      } as ReturnType<typeof useSWRAuth>;
    });

    render(<StudentsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Schüler/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no students exist", async () => {
    vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
      if (key === "database-students-list") {
        return {
          data: [],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      }
      return {
        data: [],
        isLoading: false,
        error: null,
        isValidating: false,
        mutate: vi.fn(),
      } as ReturnType<typeof useSWRAuth>;
    });

    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Schüler vorhanden")).toBeInTheDocument();
    });
  });

  it("filters students by search term", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Max" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("clears all filters when clear button is clicked", async () => {
    render(<StudentsPage />);

    // Set a search term first
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "test" } });

    // Click clear filters
    const clearButton = screen.getByTestId("clear-filters");
    fireEvent.click(clearButton);

    await waitFor(() => {
      expect(searchInput).toHaveValue("");
    });
  });

  it("displays group badges for students with groups", async () => {
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppe A")).toBeInTheDocument();
      expect(screen.getByText("Gruppe B")).toBeInTheDocument();
    });
  });

  it("displays guardian info when available", async () => {
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText(/Hans Mustermann/)).toBeInTheDocument();
    });
  });

  it("opens create modal when create button is clicked", async () => {
    render(<StudentsPage />);

    const createButton = screen.getByLabelText("Schüler erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("student-create-modal")).toBeInTheDocument();
    });
  });

  it("opens detail modal when student row is clicked", async () => {
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    });

    const studentRow = screen.getByText("Max Mustermann").closest("button");
    if (studentRow) {
      fireEvent.click(studentRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("student-detail-modal")).toBeInTheDocument();
      expect(screen.getByTestId("detail-student-name")).toHaveTextContent(
        "Max Mustermann",
      );
    });
  });

  it("opens edit modal when edit button is clicked in detail modal", async () => {
    render(<StudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
    });

    const studentRow = screen.getByText("Max Mustermann").closest("button");
    if (studentRow) {
      fireEvent.click(studentRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("student-detail-modal")).toBeInTheDocument();
    });

    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("student-edit-modal")).toBeInTheDocument();
    });
  });
});
