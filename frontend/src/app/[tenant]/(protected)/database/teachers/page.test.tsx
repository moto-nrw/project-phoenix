import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import TeachersPage from "./page";

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

vi.mock("@/components/teachers", () => ({
  TeacherRoleManagementModal: () => <div data-testid="role-modal" />,
  TeacherPermissionManagementModal: () => (
    <div data-testid="permission-modal" />
  ),
}));

vi.mock("@/components/teachers/teacher-detail-modal", () => ({
  TeacherDetailModal: ({
    isOpen,
    teacher,
    onClose,
    onEdit,
    onDelete,
  }: {
    isOpen: boolean;
    teacher: { first_name: string; last_name: string } | null;
    onClose: () => void;
    onEdit: () => void;
    onDelete: () => void;
  }) =>
    isOpen && teacher ? (
      <div data-testid="teacher-detail-modal">
        <span data-testid="detail-teacher-name">
          {teacher.first_name} {teacher.last_name}
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

vi.mock("@/components/teachers/teacher-edit-modal", () => ({
  TeacherEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: { first_name: string; last_name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="teacher-edit-modal">
        <button
          data-testid="submit-edit"
          onClick={() =>
            void onSave({ first_name: "Updated", last_name: "Teacher" })
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

vi.mock("@/components/teachers/teacher-create-modal", () => ({
  TeacherCreateModal: ({
    isOpen,
    onClose,
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: {
      first_name: string;
      last_name: string;
    }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="teacher-create-modal">
        <button
          data-testid="submit-create"
          onClick={() =>
            void onCreate({ first_name: "New", last_name: "Teacher" })
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
  Modal: ({
    isOpen,
    children,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
  }) => (isOpen ? <div data-testid="modal">{children}</div> : null),
  ConfirmationModal: () => <div data-testid="confirmation-modal" />,
}));

// Import mocked modules
import { useSWRAuth } from "~/lib/swr";

const mockTeachers = [
  {
    id: "1",
    first_name: "Maria",
    last_name: "Müller",
    email: "maria@example.com",
    roles: ["teacher"],
  },
  {
    id: "2",
    first_name: "Thomas",
    last_name: "Schmidt",
    email: "thomas@example.com",
    roles: ["admin", "teacher"],
  },
];

describe("TeachersPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockTeachers,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    // Setup getOne to return the selected teacher
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockTeachers.find((t) => t.id === id)),
    );
  });

  it("renders the page with teachers data", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
      expect(screen.getByText("Thomas Schmidt")).toBeInTheDocument();
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

    render(<TeachersPage />);

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

    render(<TeachersPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Betreuer/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no teachers exist", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Betreuer vorhanden")).toBeInTheDocument();
    });
  });

  it("filters teachers by search term", async () => {
    render(<TeachersPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Maria" } });

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
      expect(screen.queryByText("Thomas Schmidt")).not.toBeInTheDocument();
    });
  });

  it("displays email for teachers", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("maria@example.com")).toBeInTheDocument();
    });
  });

  it("opens create modal when create button is clicked", async () => {
    render(<TeachersPage />);

    // Click the "Betreuer hinzufügen" button to open choice modal
    const addButton = screen.getByLabelText("Betreuer hinzufügen");
    fireEvent.click(addButton);

    // Wait for choice modal to appear
    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Click on "Manuell erstellen" option in the choice modal
    const createOption = screen.getByText("Manuell erstellen");
    fireEvent.click(createOption);

    // Now the create modal should open
    await waitFor(() => {
      expect(screen.getByTestId("teacher-create-modal")).toBeInTheDocument();
    });
  });

  it("opens detail modal when teacher row is clicked", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen.getByText("Maria Müller").closest("button");
    if (teacherRow) {
      fireEvent.click(teacherRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
      expect(screen.getByTestId("detail-teacher-name")).toHaveTextContent(
        "Maria Müller",
      );
    });
  });

  it("opens edit modal when edit button is clicked in detail modal", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen.getByText("Maria Müller").closest("button");
    if (teacherRow) {
      fireEvent.click(teacherRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("teacher-edit-modal")).toBeInTheDocument();
    });
  });

  it("clears all filters when clear button is clicked", async () => {
    render(<TeachersPage />);

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
    mockCreate.mockResolvedValueOnce({
      id: "3",
      first_name: "New",
      last_name: "Teacher",
    });

    render(<TeachersPage />);

    // Click "Betreuer hinzufügen" to open choice modal
    const addButton = screen.getByLabelText("Betreuer hinzufügen");
    fireEvent.click(addButton);

    // Wait for choice modal
    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Click on "Manuell erstellen" option
    const createOption = screen.getByText("Manuell erstellen");
    fireEvent.click(createOption);

    // Now the create modal should open
    await waitFor(() => {
      expect(screen.getByTestId("teacher-create-modal")).toBeInTheDocument();
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
      last_name: "Teacher",
    });

    render(<TeachersPage />);

    // Select a teacher to open detail modal
    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen.getByText("Maria Müller").closest("button");
    if (teacherRow) {
      fireEvent.click(teacherRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    // Click edit button
    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("teacher-edit-modal")).toBeInTheDocument();
    });

    // Submit edit form
    const submitButton = screen.getByTestId("submit-edit");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalled();
    });
  });

  it("calls delete service when deleting a teacher", async () => {
    mockDelete.mockResolvedValueOnce({});

    render(<TeachersPage />);

    // Select a teacher to open detail modal
    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen.getByText("Maria Müller").closest("button");
    if (teacherRow) {
      fireEvent.click(teacherRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    // Click delete button
    const deleteButton = screen.getByTestId("delete-button");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalled();
    });
  });

  it("closes detail modal when close button is clicked", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen.getByText("Maria Müller").closest("button");
    if (teacherRow) {
      fireEvent.click(teacherRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    const closeButton = screen.getByTestId("close-detail-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(
        screen.queryByTestId("teacher-detail-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("closes edit modal when close button is clicked", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen.getByText("Maria Müller").closest("button");
    if (teacherRow) {
      fireEvent.click(teacherRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    const editButton = screen.getByTestId("edit-button");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("teacher-edit-modal")).toBeInTheDocument();
    });

    const closeButton = screen.getByTestId("close-edit-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(
        screen.queryByTestId("teacher-edit-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("shows not found message when search has no matches", async () => {
    render(<TeachersPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "xyz123" } });

    await waitFor(() => {
      expect(screen.getByText("Keine Betreuer gefunden")).toBeInTheDocument();
    });
  });

  // Tests for getTeacherInitials helper function coverage
  describe("getTeacherInitials coverage", () => {
    it("displays initials from fullName when first_name and last_name are missing", async () => {
      vi.mocked(useSWRAuth).mockReturnValue({
        data: [
          {
            id: "3",
            name: "Max Mustermann", // Only name field, no first_name/last_name
            first_name: undefined,
            last_name: undefined,
            email: "max@example.com",
          },
        ],
        isLoading: false,
        error: null,
        isValidating: false,
        mutate: vi.fn(),
      } as ReturnType<typeof useSWRAuth>);

      render(<TeachersPage />);

      await waitFor(() => {
        // Should display "MM" (from "Max Mustermann")
        expect(screen.getByText("MM")).toBeInTheDocument();
      });
    });

    it("displays XX when no name data is available", async () => {
      vi.mocked(useSWRAuth).mockReturnValue({
        data: [
          {
            id: "4",
            name: undefined,
            first_name: undefined,
            last_name: undefined,
            email: "unknown@example.com",
          },
        ],
        isLoading: false,
        error: null,
        isValidating: false,
        mutate: vi.fn(),
      } as ReturnType<typeof useSWRAuth>);

      render(<TeachersPage />);

      await waitFor(() => {
        // Should display "XX" as fallback
        expect(screen.getByText("XX")).toBeInTheDocument();
      });
    });
  });

  describe("SWR fetcher execution", () => {
    it("executes the SWR fetcher and handles array response", async () => {
      const mockGetList = vi.fn().mockResolvedValue({
        data: [{ id: "1", name: "Test Teacher" }],
      });

      // Re-mock createCrudService to track getList calls
      const serviceFactory = await import("@/lib/database/service-factory");
      vi.mocked(serviceFactory.createCrudService).mockReturnValue({
        getList: mockGetList,
        getOne: mockGetOne,
        create: mockCreate,
        update: mockUpdate,
        delete: mockDelete,
      });

      // Mock useSWRAuth to actually execute the fetcher
      let capturedFetcher: (() => Promise<unknown>) | null = null;
      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-teachers-list" && fetcher) {
          capturedFetcher = fetcher as () => Promise<unknown>;
        }
        return {
          data: [
            {
              id: "1",
              name: "Test Teacher",
              first_name: "Test",
              last_name: "Teacher",
            },
          ],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<TeachersPage />);

      // Execute the captured fetcher to cover the fetcher code path
      expect(capturedFetcher).not.toBeNull();
      const result: unknown = await (
        capturedFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([{ id: "1", name: "Test Teacher" }]);
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

      let capturedFetcher: (() => Promise<unknown>) | null = null;
      vi.mocked(useSWRAuth).mockImplementation((key, fetcher) => {
        if (key === "database-teachers-list" && fetcher) {
          capturedFetcher = fetcher as () => Promise<unknown>;
        }
        return {
          data: [],
          isLoading: false,
          error: null,
          isValidating: false,
          mutate: vi.fn(),
        } as ReturnType<typeof useSWRAuth>;
      });

      render(<TeachersPage />);

      expect(capturedFetcher).not.toBeNull();
      const result: unknown = await (
        capturedFetcher as unknown as () => Promise<unknown>
      )();
      expect(result).toEqual([]); // Should return empty array for non-array data
    });
  });

  describe("Email invite navigation", () => {
    it("navigates to invitations when email invite option is clicked", async () => {
      const mockPush = vi.fn();
      const useRouter = await import("next/navigation");
      vi.mocked(useRouter.useRouter).mockReturnValue({
        push: mockPush,
        replace: vi.fn(),
        refresh: vi.fn(),
        back: vi.fn(),
        forward: vi.fn(),
        prefetch: vi.fn(),
      } as ReturnType<typeof useRouter.useRouter>);

      render(<TeachersPage />);

      // Click the "Betreuer hinzufügen" button to open choice modal
      const addButton = screen.getByLabelText("Betreuer hinzufügen");
      fireEvent.click(addButton);

      // Wait for choice modal to appear
      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });

      // Click on "Per E-Mail einladen" option
      const emailOption = screen.getByText("Per E-Mail einladen");
      fireEvent.click(emailOption);

      // Verify navigation to /invitations
      await waitFor(() => {
        expect(mockPush).toHaveBeenCalledWith("/invitations");
      });
    });
  });
});
