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
    filters,
    onClearAllFilters,
  }: {
    search: { value: string; onChange: (v: string) => void };
    filters: Array<{
      id: string;
      value: string;
      onChange: (v: string) => void;
      options?: Array<{ value: string; label: string }>;
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
          {f.options?.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          )) ?? <option value="all">All</option>}
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
    onDelete,
  }: {
    isOpen: boolean;
    student: { first_name: string; second_name: string } | null;
    onClose: () => void;
    onEdit: () => void;
    onDelete: () => void;
  }) =>
    isOpen && student ? (
      <div data-testid="student-detail-modal">
        <span data-testid="detail-student-name">
          {student.first_name} {student.second_name}
        </span>
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
}));

vi.mock("@/components/students/student-edit-modal", () => ({
  StudentEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: {
      first_name: string;
      second_name: string;
    }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="student-edit-modal">
        <button
          data-testid="submit-edit"
          onClick={() =>
            void onSave({ first_name: "Updated", second_name: "Student" })
          }
        >
          Save
        </button>
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
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: {
      first_name: string;
      second_name: string;
    }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="student-create-modal">
        <button
          data-testid="submit-create"
          onClick={() =>
            void onCreate({ first_name: "New", second_name: "Student" })
          }
        >
          Submit
        </button>
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

  it("filters students by guardian name (name_lg)", async () => {
    render(<StudentsPage />);

    // name_lg is "Hans Mustermann" for student with id "1"
    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Hans" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      // Anna has null name_lg, so should be filtered out
      expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
    });
  });

  it("filters students by school class", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "1a" } });

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      // Anna is in 2b, so should be filtered out
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
      // Use getAllByText since "Gruppe A/B" appears in both dropdown and badges
      expect(screen.getAllByText("Gruppe A").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Gruppe B").length).toBeGreaterThan(0);
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

  it("calls create service when submitting create form", async () => {
    mockCreate.mockResolvedValueOnce({
      id: "3",
      first_name: "New",
      second_name: "Student",
    });

    render(<StudentsPage />);

    // Open create modal
    const createButton = screen.getByLabelText("Schüler erstellen");
    fireEvent.click(createButton);

    await waitFor(() => {
      expect(screen.getByTestId("student-create-modal")).toBeInTheDocument();
    });

    // Submit the form
    const submitButton = screen.getByTestId("submit-create");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
    });
  });

  it("calls update service when saving edit form", async () => {
    mockUpdate.mockResolvedValueOnce({
      id: "1",
      first_name: "Updated",
      second_name: "Student",
    });

    render(<StudentsPage />);

    // Select a student to open detail modal
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

    // Click edit button
    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("student-edit-modal")).toBeInTheDocument();
    });

    // Submit edit form
    const submitButton = screen.getByTestId("submit-edit");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  it("calls delete service when deleting a student", async () => {
    mockDelete.mockResolvedValueOnce({});

    render(<StudentsPage />);

    // Select a student to open detail modal
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

    // Click delete button
    const deleteButton = screen.getByTestId("delete-button");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalled();
    });
  });

  it("closes detail modal when close button is clicked", async () => {
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

    const closeButton = screen.getByTestId("close-detail-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(
        screen.queryByTestId("student-detail-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("closes edit modal when close button is clicked", async () => {
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

    const closeButton = screen.getByTestId("close-edit-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(
        screen.queryByTestId("student-edit-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("shows not found message when search has no matches", async () => {
    render(<StudentsPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "xyz123" } });

    await waitFor(() => {
      expect(screen.getByText("Keine Schüler gefunden")).toBeInTheDocument();
    });
  });

  // Tests for SWR fetcher execution (new code coverage)
  describe("SWR fetcher execution", () => {
    it("executes the students SWR fetcher and handles array response", async () => {
      const mockGetList = vi.fn().mockResolvedValue({
        data: [{ id: "1", first_name: "Test", second_name: "Student" }],
      });

      const serviceFactory = await import("@/lib/database/service-factory");
      vi.mocked(serviceFactory.createCrudService).mockReturnValue({
        getList: mockGetList,
        getOne: mockGetOne,
        create: mockCreate,
        update: mockUpdate,
        delete: mockDelete,
      });

      let capturedStudentsFetcher: (() => Promise<unknown>) | null = null;

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-students-list" && fetcher) {
          capturedStudentsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: [
              {
                id: "1",
                first_name: "Test",
                second_name: "Student",
                school_class: "1a",
              },
            ],
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

      // Execute the captured fetcher to cover lines 75-78
      expect(capturedStudentsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedStudentsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([
        { id: "1", first_name: "Test", second_name: "Student" },
      ]);
      expect(mockGetList).toHaveBeenCalledWith({ page: 1, pageSize: 1000 });
    });

    it("handles non-array response from getList", async () => {
      const mockGetList = vi.fn().mockResolvedValue({
        data: "not an array",
      });

      const serviceFactory = await import("@/lib/database/service-factory");
      vi.mocked(serviceFactory.createCrudService).mockReturnValue({
        getList: mockGetList,
        getOne: mockGetOne,
        create: mockCreate,
        update: mockUpdate,
        delete: mockDelete,
      });

      let capturedStudentsFetcher: (() => Promise<unknown>) | null = null;

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-students-list" && fetcher) {
          capturedStudentsFetcher = fetcher as () => Promise<unknown>;
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

      expect(capturedStudentsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedStudentsFetcher as unknown as () => Promise<unknown>
      )();
      // Should return empty array when data is not an array
      expect(result).toEqual([]);
    });

    it("executes the groups dropdown SWR fetcher with array response", async () => {
      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      // Mock fetch for groups API
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve([
            { id: 1, name: "Group Alpha" },
            { id: 2, name: "Group Beta" },
          ]),
      });
      vi.stubGlobal("fetch", mockFetch);

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-groups-dropdown" && fetcher) {
          capturedGroupsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: mockStudents,
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

      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedGroupsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([
        { value: "1", label: "Group Alpha" },
        { value: "2", label: "Group Beta" },
      ]);

      vi.unstubAllGlobals();
    });

    it("handles wrapped groups response with data property", async () => {
      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      // Mock fetch with wrapped response { data: [...] }
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: [
              { id: 3, name: "Wrapped Group A" },
              { id: 4, name: "Wrapped Group B" },
            ],
          }),
      });
      vi.stubGlobal("fetch", mockFetch);

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-groups-dropdown" && fetcher) {
          capturedGroupsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: mockStudents,
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

      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedGroupsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([
        { value: "3", label: "Wrapped Group A" },
        { value: "4", label: "Wrapped Group B" },
      ]);

      vi.unstubAllGlobals();
    });

    it("handles failed fetch for groups", async () => {
      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      // Mock fetch with error response
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
      });
      vi.stubGlobal("fetch", mockFetch);

      // Spy on console.error
      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-groups-dropdown" && fetcher) {
          capturedGroupsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: mockStudents,
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

      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedGroupsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Failed to fetch groups:",
        500,
      );

      consoleErrorSpy.mockRestore();
      vi.unstubAllGlobals();
    });

    it("handles unexpected groups response format", async () => {
      let capturedGroupsFetcher: (() => Promise<unknown>) | null = null;

      // Mock fetch with unexpected format
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: () => Promise.resolve("unexpected string"),
      });
      vi.stubGlobal("fetch", mockFetch);

      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-groups-dropdown" && fetcher) {
          capturedGroupsFetcher = fetcher as () => Promise<unknown>;
        }
        if (key === "database-students-list") {
          return {
            data: mockStudents,
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

      expect(capturedGroupsFetcher).not.toBeNull();
      const result: unknown = await (
        capturedGroupsFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([]);
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Unexpected groups response format:",
        "unexpected string",
      );

      consoleErrorSpy.mockRestore();
      vi.unstubAllGlobals();
    });
  });

  // Tests for group filter coverage (line 138-140)
  describe("Group Filter", () => {
    it("filters students by group selection", async () => {
      const studentsWithGroups = [
        {
          id: "1",
          first_name: "Max",
          second_name: "Mustermann",
          school_class: "1a",
          group_id: "group-1",
          group_name: "Gruppe A",
        },
        {
          id: "2",
          first_name: "Anna",
          second_name: "Schmidt",
          school_class: "2b",
          group_id: "group-2",
          group_name: "Gruppe B",
        },
      ];

      // Mock useSWRAuth to return different data based on cache key
      vi.mocked(useSWRAuth).mockImplementation((key) => {
        if (key === "database-students-list") {
          return {
            data: studentsWithGroups,
            isLoading: false,
            error: null,
            isValidating: false,
            mutate: vi.fn(),
          } as ReturnType<typeof useSWRAuth>;
        }
        // For groups dropdown
        return {
          data: [
            { value: "group-1", label: "Gruppe A" },
            { value: "group-2", label: "Gruppe B" },
          ],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      await waitFor(() => {
        expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
        expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
      });

      // Change group filter to "group-1"
      const groupFilter = screen.getByTestId("filter-group");
      fireEvent.change(groupFilter, { target: { value: "group-1" } });

      await waitFor(() => {
        // Max (group-1) should be visible
        expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
        // Anna (group-2) should be filtered out
        expect(screen.queryByText("Anna Schmidt")).not.toBeInTheDocument();
      });
    });

    it("shows empty state when group filter has no matches", async () => {
      // Mock useSWRAuth to return different data based on cache key
      vi.mocked(useSWRAuth).mockImplementation((key) => {
        if (key === "database-students-list") {
          return {
            data: [
              {
                id: "1",
                first_name: "Max",
                second_name: "Mustermann",
                school_class: "1a",
                group_id: "group-1",
                group_name: "Gruppe A",
              },
            ],
            isLoading: false,
            error: null,
            isValidating: false,
            mutate: vi.fn(),
          } as ReturnType<typeof useSWRAuth>;
        }
        // For groups dropdown
        return {
          data: [{ value: "group-1", label: "Gruppe A" }],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<StudentsPage />);

      await waitFor(() => {
        expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      });

      // Change group filter to a non-existing group
      const groupFilter = screen.getByTestId("filter-group");
      fireEvent.change(groupFilter, {
        target: { value: "non-existing-group" },
      });

      await waitFor(() => {
        expect(screen.getByText("Keine Schüler gefunden")).toBeInTheDocument();
      });
    });
  });
});
