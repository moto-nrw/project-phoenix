import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import TeachersPage from "./page";

const { mockUseSession, mockRedirect, mockTenantMutate, mockLoggerError } =
  vi.hoisted(() => ({
    mockUseSession: vi.fn(() => ({
      data: { user: { id: "1", token: "test-token" }, expires: "2099-01-01" },
      status: "authenticated",
    })),
    mockRedirect: vi.fn(),
    mockTenantMutate: vi.fn(() => Promise.resolve()),
    mockLoggerError: vi.fn(),
  }));

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
  useRouter: vi.fn(() => ({ push: vi.fn() })),
}));

// Mock SWR hooks
vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useTenantMutate: vi.fn(() => mockTenantMutate),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: vi.fn(() => ({
    error: mockLoggerError,
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  })),
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

const mockHandleDeleteClick = vi.fn();
const mockHandleDeleteCancel = vi.fn();
const mockConfirmDelete = vi.fn((callback?: () => void) => callback?.());
vi.mock("~/hooks/useDeleteConfirmation", () => ({
  useDeleteConfirmation: vi.fn(() => ({
    showConfirmModal: false,
    handleDeleteClick: mockHandleDeleteClick,
    handleDeleteCancel: mockHandleDeleteCancel,
    confirmDelete: mockConfirmDelete,
  })),
}));

const mockToastError = vi.fn();
const mockToastSuccess = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
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
    activeFilters,
    onClearAllFilters,
    actionButton,
  }: {
    search: { value: string; onChange: (v: string) => void };
    activeFilters?: Array<{ id: string; label: string; onRemove: () => void }>;
    onClearAllFilters: () => void;
    actionButton?: React.ReactNode;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      {actionButton}
      {activeFilters?.map((filter) => (
        <button
          key={filter.id}
          data-testid={`active-filter-${filter.id}`}
          onClick={filter.onRemove}
        >
          {filter.label}
        </button>
      ))}
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
    </div>
  ),
}));

vi.mock("@/components/teachers", () => ({
  CaregiverCapabilityModal: ({
    onClose,
    onUpdated,
  }: {
    onClose: () => void;
    onUpdated: () => Promise<void>;
  }) => (
    <div data-testid="caregiver-modal">
      <button data-testid="caregiver-close" onClick={onClose}>
        Close Caregiver
      </button>
      <button data-testid="caregiver-update" onClick={() => void onUpdated()}>
        Update Caregiver
      </button>
    </div>
  ),
  TeacherRoleManagementModal: ({
    onClose,
    onUpdate,
  }: {
    onClose: () => void;
    onUpdate: () => void;
  }) => (
    <div data-testid="role-modal">
      <button data-testid="role-close" onClick={onClose}>
        Close Role
      </button>
      <button data-testid="role-update" onClick={onUpdate}>
        Update Role
      </button>
    </div>
  ),
  TeacherPermissionManagementModal: ({
    onClose,
    onUpdate,
  }: {
    onClose: () => void;
    onUpdate: () => void;
  }) => (
    <div data-testid="permission-modal">
      <button data-testid="permission-close" onClick={onClose}>
        Close Permission
      </button>
      <button data-testid="permission-update" onClick={onUpdate}>
        Update Permission
      </button>
    </div>
  ),
}));

vi.mock("@/components/teachers/teacher-detail-modal", () => ({
  TeacherDetailModal: ({
    isOpen,
    teacher,
    onClose,
    onEdit,
    onDelete,
    onDeleteClick,
    onManageCaregiver,
    onUpdateNotes,
  }: {
    isOpen: boolean;
    teacher: {
      first_name: string;
      last_name: string;
      account_id?: string;
    } | null;
    onClose: () => void;
    onEdit: () => void;
    onDelete: () => void;
    onDeleteClick?: () => void;
    onManageCaregiver?: () => void;
    onUpdateNotes?: (notes: string) => Promise<void>;
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
        {onDeleteClick ? (
          <button
            data-testid="open-delete-confirmation"
            onClick={onDeleteClick}
          >
            Open Delete Confirmation
          </button>
        ) : null}
        {onManageCaregiver ? (
          <button
            data-testid="manage-caregiver-button"
            onClick={onManageCaregiver}
          >
            Manage Caregiver
          </button>
        ) : null}
        {onUpdateNotes ? (
          <button
            data-testid="update-notes-button"
            onClick={() => void onUpdateNotes("Neue Notiz")}
          >
            Update Notes
          </button>
        ) : null}
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
    existingPositions,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: { first_name: string; last_name: string }) => Promise<void>;
    existingPositions?: string[];
  }) =>
    isOpen ? (
      <div data-testid="teacher-edit-modal">
        <span data-testid="edit-existing-positions">
          {existingPositions?.join(",") ?? ""}
        </span>
        <button
          data-testid="submit-edit"
          onClick={() =>
            void onSave({
              first_name: "Updated",
              last_name: "Teacher",
            }).catch(() => {})
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
  InvitationForm: ({
    onCreated,
    existingPositions,
  }: {
    onCreated: () => void;
    existingPositions?: string[];
  }) => (
    <div data-testid="invitation-form">
      <span data-testid="invite-existing-positions">
        {existingPositions?.join(",") ?? ""}
      </span>
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
  ConfirmationModal: ({
    onClose,
    onConfirm,
  }: {
    onClose: () => void;
    onConfirm: () => void;
  }) => (
    <div data-testid="confirmation-modal">
      <button data-testid="confirm-delete" onClick={onConfirm}>
        Confirm Delete
      </button>
      <button data-testid="close-confirmation" onClick={onClose}>
        Close Confirmation
      </button>
    </div>
  ),
}));

// Import mocked modules
import { useSWRAuth } from "~/lib/swr";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";

const mockTeachers = [
  {
    id: "1",
    first_name: "Maria",
    last_name: "Müller",
    email: "maria@example.com",
    role: "OGS-Büro",
    account_role: "admin",
    account_id: "11",
    staff_id: "101",
  },
  {
    id: "2",
    first_name: "Thomas",
    last_name: "Schmidt",
    email: "thomas@example.com",
    role: "Pädagogische Fachkraft",
    account_role: "user",
    account_id: "12",
    staff_id: "102",
  },
];

describe("TeachersPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSession.mockImplementation(() => ({
      data: { user: { id: "1", token: "test-token" }, expires: "2099-01-01" },
      status: "authenticated",
    }));
    mockTenantMutate.mockResolvedValue(undefined);

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockTeachers,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    // Setup getOne to return the selected teacher
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(
        mockTeachers.find((t) => t.id === id || t.staff_id === id),
      ),
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

  it("redirects unauthenticated users", () => {
    mockUseSession.mockImplementationOnce(
      (options?: { onUnauthenticated?: () => void }) => {
        options?.onUnauthenticated?.();
        return {
          data: null,
          status: "unauthenticated",
        };
      },
    );

    render(<TeachersPage />);

    expect(mockRedirect).toHaveBeenCalledWith("/");
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

  it("filters teachers by account role", async () => {
    render(<TeachersPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "admin" } });

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
    const addButton = screen.getAllByLabelText("Personal hinzufügen")[0]!;
    fireEvent.click(addButton);

    // Wait for invite modal to appear
    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByTestId("invitation-form")).toBeInTheDocument();
    });
  });

  it("logs an error when loading teacher details fails", async () => {
    mockGetOne.mockRejectedValueOnce(new Error("teacher lookup failed"));

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();

    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        "failed to fetch teacher details",
        { error: "teacher lookup failed" },
      );
    });
  });

  it("opens detail modal when teacher row is clicked", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
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

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
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
      expect(screen.getByTestId("edit-existing-positions")).toHaveTextContent(
        "OGS-Büro,Pädagogische Fachkraft",
      );
    });
  });

  it("passes existing positions into the invitation form", async () => {
    render(<TeachersPage />);

    fireEvent.click(screen.getAllByLabelText("Personal hinzufügen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("invite-existing-positions")).toHaveTextContent(
        "OGS-Büro,Pädagogische Fachkraft",
      );
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
    const addButton = screen.getAllByLabelText("Personal hinzufügen")[0]!;
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

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
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

  it("logs an error when saving the edit form fails", async () => {
    mockUpdate.mockRejectedValueOnce(new Error("update failed"));

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();
    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("edit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("teacher-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith("failed to update teacher", {
        error: "update failed",
      });
    });
  });

  it("calls delete service when deleting a teacher", async () => {
    mockDelete.mockResolvedValueOnce(null);

    render(<TeachersPage />);

    // Select a teacher to open detail modal
    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
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

    const row = screen.getByText("Maria Müller").closest('[role="button"]');
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

  it("confirms deletion through the confirmation modal", async () => {
    vi.mocked(useDeleteConfirmation).mockReturnValueOnce({
      showConfirmModal: true,
      handleDeleteClick: mockHandleDeleteClick,
      handleDeleteCancel: mockHandleDeleteCancel,
      confirmDelete: mockConfirmDelete,
    });
    mockDelete.mockResolvedValueOnce(null);

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();
    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockConfirmDelete).toHaveBeenCalled();
      expect(mockDelete).toHaveBeenCalledWith("1");
    });
  });

  it("closes the confirmation modal through the cancel callback", async () => {
    vi.mocked(useDeleteConfirmation).mockReturnValueOnce({
      showConfirmModal: true,
      handleDeleteClick: mockHandleDeleteClick,
      handleDeleteCancel: mockHandleDeleteCancel,
      confirmDelete: mockConfirmDelete,
    });

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();
    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("close-confirmation"));

    expect(mockHandleDeleteCancel).toHaveBeenCalled();
  });

  it("closes detail modal when close button is clicked", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
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

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
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

  it("opens caregiver modal from the detail modal", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    if (teacherRow) {
      fireEvent.click(teacherRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("manage-caregiver-button"));

    await waitFor(() => {
      expect(screen.getByTestId("caregiver-modal")).toBeInTheDocument();
    });
  });

  it("closes the caregiver modal through its close callback", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();
    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("manage-caregiver-button"));
    fireEvent.click(screen.getByTestId("caregiver-close"));

    expect(screen.getByTestId("caregiver-modal")).toBeInTheDocument();
  });

  it("revalidates teachers after caregiver capability changes", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();
    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("manage-caregiver-button"));
    fireEvent.click(screen.getByTestId("caregiver-update"));

    await waitFor(() => {
      expect(mockTenantMutate).toHaveBeenCalledWith("database-teachers-list");
    });
  });

  it("updates notes from the detail modal", async () => {
    mockUpdate.mockResolvedValueOnce({
      id: "1",
      staff_notes: "Neue Notiz",
    });

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    if (teacherRow) {
      fireEvent.click(teacherRow);
    }

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("update-notes-button"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith("1", {
        staff_notes: "Neue Notiz",
      });
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

  it("clears the active search filter from the chip action", async () => {
    render(<TeachersPage />);

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Maria" } });

    await waitFor(() => {
      expect(screen.getByTestId("active-filter-search")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("active-filter-search"));

    await waitFor(() => {
      expect(searchInput).toHaveValue("");
    });
  });

  it("opens the detail modal from keyboard enter", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();

    fireEvent.keyDown(teacherRow!, { key: "Enter" });

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });
  });

  it("opens the detail modal from keyboard space", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();

    fireEvent.keyDown(teacherRow!, { key: " " });

    await waitFor(() => {
      expect(screen.getByTestId("teacher-detail-modal")).toBeInTheDocument();
    });
  });

  it("copies the teacher email to the clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(window.navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("maria@example.com")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("E-Mail kopieren")[0]!);

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("maria@example.com");
      expect(mockToastSuccess).toHaveBeenCalledWith("E-Mail kopiert");
    });
  });

  it("does not open the detail modal when clicking the mailto link", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("maria@example.com")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("E-Mail senden")[0]!);

    expect(
      screen.queryByTestId("teacher-detail-modal"),
    ).not.toBeInTheDocument();
  });

  it("shows an error toast when copying the email fails", async () => {
    const writeText = vi.fn().mockRejectedValue(new Error("clipboard denied"));
    Object.defineProperty(window.navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("maria@example.com")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("E-Mail kopieren")[0]!);

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith("Kopieren fehlgeschlagen");
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
      const addButton = screen.getAllByLabelText("Personal hinzufügen")[0]!;
      fireEvent.click(addButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
        expect(screen.getByTestId("invitation-form")).toBeInTheDocument();
      });
    });
  });

  it("closes choice modal via close button", async () => {
    render(<TeachersPage />);

    const addButton = screen.getAllByLabelText("Personal hinzufügen")[0]!;
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
    fireEvent.click(screen.getAllByLabelText("Personal hinzufügen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    // Close the invite modal
    fireEvent.click(screen.getByTestId("close-modal"));
    await waitFor(() => {
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });
  });

  it("clears the selected teacher when the role modal closes", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();
    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(screen.getByTestId("role-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("role-close"));

    await waitFor(() => {
      expect(
        screen.queryByTestId("teacher-detail-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("ignores tenant revalidation errors from the role modal", async () => {
    mockTenantMutate.mockRejectedValueOnce(new Error("revalidation failed"));

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();
    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(screen.getByTestId("role-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("role-update"));

    await waitFor(() => {
      expect(mockTenantMutate).toHaveBeenCalledWith("database-teachers-list");
    });
  });

  it("clears the selected teacher when the permission modal closes", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();
    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(screen.getByTestId("permission-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("permission-close"));

    await waitFor(() => {
      expect(
        screen.queryByTestId("teacher-detail-modal"),
      ).not.toBeInTheDocument();
    });
  });

  it("ignores tenant revalidation errors from the permission modal", async () => {
    mockTenantMutate.mockRejectedValueOnce(new Error("revalidation failed"));

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Maria Müller")).toBeInTheDocument();
    });

    const teacherRow = screen
      .getByText("Maria Müller")
      .closest('[role="button"]');
    expect(teacherRow).not.toBeNull();
    fireEvent.click(teacherRow!);

    await waitFor(() => {
      expect(screen.getByTestId("permission-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("permission-update"));

    await waitFor(() => {
      expect(mockTenantMutate).toHaveBeenCalledWith("database-teachers-list");
    });
  });
});
