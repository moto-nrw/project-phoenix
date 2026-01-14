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
  StudentDetailModal: () => <div data-testid="student-detail-modal" />,
}));

vi.mock("@/components/students/student-edit-modal", () => ({
  StudentEditModal: () => <div data-testid="student-edit-modal" />,
}));

vi.mock("@/components/students/student-create-modal", () => ({
  StudentCreateModal: () => <div data-testid="student-create-modal" />,
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
});
