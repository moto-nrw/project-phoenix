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
  useTenantMutate: vi.fn(() => vi.fn()),
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

const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: mockToastError,
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
  CaregiverCapabilityModal: () => <div data-testid="caregiver-modal" />,
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

vi.mock("~/components/admin/invitation-form", () => ({
  InvitationForm: ({ onCreated }: { onCreated: () => void }) => (
    <div data-testid="invitation-form">
      <button data-testid="submit-invite" onClick={onCreated}>
        Submit
      </button>
    </div>
  ),
}));

vi.mock("~/components/admin/pending-invitations-list", () => ({
  PendingInvitationsList: ({ refreshKey }: { refreshKey: number }) => (
    <div data-testid="pending-list" data-refresh-key={refreshKey}>
      Pending
    </div>
  ),
}));

vi.mock("~/components/auth/role-guard", () => ({
  RoleGuard: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="role-guard">{children}</div>
  ),
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    children,
    onClose,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    onClose: () => void;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <button data-testid="close-modal" onClick={onClose}>
          Close
        </button>
        {children}
      </div>
    ) : null,
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
        screen.getByText(/Fehler beim Laden des Personals/),
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
      expect(screen.getByText("Kein Personal vorhanden")).toBeInTheDocument();
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

  it("opens invite modal when add button is clicked", async () => {
    render(<TeachersPage />);

    // Click the "Personal hinzufügen" button to open invite modal
    const addButton = screen.getByLabelText("Personal hinzufügen");
    fireEvent.click(addButton);

    // Wait for invite modal to appear
    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByTestId("invitation-form")).toBeInTheDocument();
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

  it("closes invite modal when invitation is created", async () => {
    render(<TeachersPage />);

    // Open invite modal
    const addButton = screen.getByLabelText("Personal hinzufügen");
    fireEvent.click(addButton);

    await waitFor(() => {
      expect(screen.getByTestId("invitation-form")).toBeInTheDocument();
    });

    // Submit the invitation form (triggers onCreated callback)
    const submitButton = screen.getByTestId("submit-invite");
    fireEvent.click(submitButton);

    // Modal should close after invitation is created
    await waitFor(() => {
      expect(screen.queryByTestId("invitation-form")).not.toBeInTheDocument();
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
    mockDelete.mockResolvedValueOnce(null);

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

  it("shows error toast when delete returns error", async () => {
    mockDelete.mockResolvedValueOnce("Personal kann nicht gelöscht werden");

    render(<TeachersPage />);
    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const row = screen.getByText("Maria Müller").closest("button");
    if (row) fireEvent.click(row);
    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("delete-button"));
    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Personal kann nicht gelöscht werden",
      );
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
      expect(screen.getByText("Kein Personal gefunden")).toBeInTheDocument();
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

  describe("Invite modal interaction", () => {
    it("shows invitation form with title in modal", async () => {
      render(<TeachersPage />);

      // Click "Personal hinzufügen" to open invite modal
      const addButton = screen.getByLabelText("Personal hinzufügen");
      fireEvent.click(addButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
        expect(screen.getByTestId("invitation-form")).toBeInTheDocument();
      });
    });
  });

  it("closes choice modal via close button", async () => {
    render(<TeachersPage />);

    const addButton = screen.getByLabelText("Personal hinzufügen");
    fireEvent.click(addButton);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    const closeButton = screen.getByTestId("close-modal");
    fireEvent.click(closeButton);

    await waitFor(() => {
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });
  });

  it("closes invite modal via close button", async () => {
    render(<TeachersPage />);

    // Open invite modal
    fireEvent.click(screen.getByLabelText("Personal hinzufügen"));
    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Close the invite modal
    fireEvent.click(screen.getByTestId("close-modal"));
    await waitFor(() => {
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });
  });
});
