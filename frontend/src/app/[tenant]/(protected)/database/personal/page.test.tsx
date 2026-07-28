import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useSession } from "next-auth/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import TeachersPage from "./page";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: {
      user: {
        id: "1",
        token: "test-token",
        permissions: ["users:manage"],
      },
      expires: "2099-01-01",
    },
    status: "authenticated",
  })),
}));

let currentSearch = new URLSearchParams();
const mockReplace = vi.fn((url: string) => {
  const query = url.includes("?") ? (url.split("?")[1] ?? "") : "";
  currentSearch = new URLSearchParams(query);
});
const setSelectedStaff = (id: string | null) => {
  currentSearch = new URLSearchParams();
  if (id) {
    currentSearch.set("staff", id);
  }
};

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: mockReplace })),
  usePathname: vi.fn(() => "/tenant/database/personal"),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

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

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
    error: mockToastError,
  })),
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
  }: {
    children: ReactNode;
    loading: boolean;
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
      {children}
    </div>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({
    search,
    onClearAllFilters,
    actionButton,
  }: {
    search: { value: string; onChange: (value: string) => void };
    onClearAllFilters: () => void;
    actionButton?: ReactNode;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(event) => search.onChange(event.target.value)}
      />
      <button
        type="button"
        data-testid="clear-filters"
        onClick={onClearAllFilters}
      >
        Clear
      </button>
      {actionButton}
    </div>
  ),
}));

vi.mock("@/components/teachers/staff-master-detail", () => ({
  StaffMasterDetail: ({
    groupDefinitions,
    selectedId,
    selectedTeacher,
    onSelect,
    onEditClick,
    onDeleteClick,
    onUpdateNotes,
    onManageCaregiver,
  }: {
    groupDefinitions: Array<{
      id: string;
      title: string;
      items: Array<{ id: string; name: string }>;
    }>;
    selectedId: string | null;
    selectedTeacher?: { name: string } | null;
    onSelect: (id: string | null) => void;
    onEditClick: () => void;
    onDeleteClick: () => void;
    onUpdateNotes: (notes: string) => Promise<void>;
    onManageCaregiver?: () => void;
  }) => (
    <div data-testid="staff-master-detail">
      {groupDefinitions.map((group) => (
        <div key={group.id} data-testid={`group-${group.id}`}>
          <span data-testid={`group-title-${group.id}`}>{group.title}</span>
          {group.items.map((teacher) => (
            <button
              type="button"
              key={teacher.id}
              data-testid={`staff-row-${teacher.id}`}
              onClick={() => onSelect(teacher.id)}
            >
              {teacher.name}
            </button>
          ))}
        </div>
      ))}
      {selectedId ? (
        <div data-testid="staff-detail-panel">
          <span data-testid="detail-selected-id">{selectedId}</span>
          <span data-testid="detail-staff-name">
            {selectedTeacher?.name ?? "unbekannt"}
          </span>
          <button
            type="button"
            data-testid="trigger-edit"
            onClick={onEditClick}
          >
            Edit
          </button>
          <button
            type="button"
            data-testid="trigger-delete"
            onClick={onDeleteClick}
          >
            Delete
          </button>
          <button
            type="button"
            data-testid="trigger-deselect"
            onClick={() => onSelect(null)}
          >
            Close
          </button>
          <button
            type="button"
            data-testid="trigger-notes"
            onClick={() => void onUpdateNotes("Updated note")}
          >
            Save Notes
          </button>
          {onManageCaregiver ? (
            <button
              type="button"
              data-testid="trigger-caregiver"
              onClick={onManageCaregiver}
            >
              Caregiver
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  ),
}));

vi.mock("@/components/teachers/caregiver-capability-modal", () => ({
  CaregiverCapabilityModal: ({ isOpen }: { isOpen: boolean }) =>
    isOpen ? <div data-testid="caregiver-modal" /> : null,
}));

vi.mock("@/components/teachers/teacher-edit-modal", () => ({
  TeacherEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: { first_name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="teacher-edit-modal">
        <button
          type="button"
          data-testid="submit-edit"
          onClick={() => void onSave({ first_name: "Updated" })}
        >
          Save
        </button>
        <button type="button" data-testid="close-edit" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/admin/invitation-form", () => ({
  InvitationForm: ({ onCreated }: { onCreated: () => void }) => (
    <div data-testid="invitation-form">
      <button
        type="button"
        data-testid="invitation-created"
        onClick={onCreated}
      >
        Submit Invitation
      </button>
    </div>
  ),
}));

vi.mock("~/components/admin/pending-invitations-list", () => ({
  PendingInvitationsList: () => <div data-testid="pending-invitations-list" />,
}));

vi.mock("~/components/auth/role-guard", () => ({
  RoleGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title?: string;
    children?: ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal" data-title={title}>
        <button type="button" data-testid="close-modal" onClick={onClose}>
          Close
        </button>
        {children}
      </div>
    ) : null,
  ConfirmationModal: ({
    isOpen,
    onConfirm,
    onClose,
  }: {
    isOpen: boolean;
    onConfirm?: () => void;
    onClose?: () => void;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <button type="button" data-testid="confirm-delete" onClick={onConfirm}>
          Confirm
        </button>
        <button type="button" data-testid="cancel-delete" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

import { useSWRAuth } from "~/lib/swr";

const mockTeachers = [
  {
    id: "1",
    name: "Anna Müller",
    first_name: "Anna",
    last_name: "Müller",
    email: "anna@example.com",
    role: "Lehrerin",
    account_role: "teacher",
    account_id: 99,
    staff_id: "11",
  },
  {
    id: "2",
    name: "Ben Schmidt",
    first_name: "Ben",
    last_name: "Schmidt",
    email: "ben@example.com",
    role: "Erzieher",
    account_role: "teacher",
    staff_id: "12",
  },
];

describe("TeachersPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();

    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "test-token",
          permissions: ["users:manage"],
        },
        expires: "2099-01-01",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockTeachers,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>);

    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(
        mockTeachers.find(
          (teacher) => teacher.id === id || teacher.staff_id === id,
        ) ?? null,
      ),
    );
  });

  it("renders the page with teachers data", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByText("Anna Müller")).toBeInTheDocument();
      expect(screen.getByText("Ben Schmidt")).toBeInTheDocument();
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

    expect(screen.getByTestId("database-layout")).toHaveAttribute(
      "data-loading",
      "true",
    );
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

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Anna" },
    });

    await waitFor(() => {
      expect(screen.getByText("Anna Müller")).toBeInTheDocument();
      expect(screen.queryByText("Ben Schmidt")).not.toBeInTheDocument();
    });
  });

  it("filters teachers by email in search", async () => {
    render(<TeachersPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "ben@example" },
    });

    await waitFor(() => {
      expect(screen.queryByText("Anna Müller")).not.toBeInTheDocument();
      expect(screen.getByText("Ben Schmidt")).toBeInTheDocument();
    });
  });

  it("clears filters when clear button is clicked", async () => {
    render(<TeachersPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Anna" },
    });

    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByTestId("search-input")).toHaveValue("");
    });
  });

  it("opens invite modal when add button is clicked", async () => {
    render(<TeachersPage />);

    fireEvent.click(screen.getAllByLabelText("Personal hinzufügen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("invitation-form")).toBeInTheDocument();
    });
  });

  it("hides invitation controls without users:manage", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "test-token",
          permissions: ["users:create"],
        },
        expires: "2099-01-01",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<TeachersPage />);

    expect(
      screen.queryByLabelText("Personal hinzufügen"),
    ).not.toBeInTheDocument();
    expect(screen.queryByTestId("invitation-form")).not.toBeInTheDocument();
  });

  it("renders the pending invitations list above the master-detail", async () => {
    render(<TeachersPage />);

    await waitFor(() => {
      expect(
        screen.getByTestId("pending-invitations-list"),
      ).toBeInTheDocument();
    });
  });

  it("syncs staff selection into the URL when a row is clicked", async () => {
    render(<TeachersPage />);

    fireEvent.click(screen.getByTestId("staff-row-1"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/tenant/database/personal?staff=1",
        { scroll: false },
      );
    });
  });

  it("hydrates the detail panel from the staff URL param using the cached list", async () => {
    setSelectedStaff("1");

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByTestId("staff-detail-panel")).toBeInTheDocument();
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("1");
      expect(screen.getByTestId("detail-staff-name")).toHaveTextContent(
        "Anna Müller",
      );
    });
    // No per-selection refetch — list DTO already carries every detail field.
    expect(mockGetOne).not.toHaveBeenCalled();
  });

  it("removes the staff URL param when the detail panel is closed", async () => {
    setSelectedStaff("1");

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByTestId("staff-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-deselect"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/personal", {
        scroll: false,
      });
    });
  });

  it("opens the edit modal when the detail panel edit button is clicked", async () => {
    setSelectedStaff("1");

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByTestId("staff-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));

    await waitFor(() => {
      expect(screen.getByTestId("teacher-edit-modal")).toBeInTheDocument();
    });
  });

  it("calls update service when saving from the edit modal", async () => {
    setSelectedStaff("1");
    mockUpdate.mockResolvedValueOnce(undefined);

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByTestId("staff-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("teacher-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ first_name: "Updated" }),
      );
    });
  });

  it("calls update service when notes are saved from the detail panel", async () => {
    setSelectedStaff("1");
    mockUpdate.mockResolvedValueOnce(undefined);

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByTestId("staff-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-notes"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith("1", {
        staff_notes: "Updated note",
      });
    });
  });

  it("calls delete service after confirming deletion from the detail panel", async () => {
    setSelectedStaff("1");
    mockDelete.mockResolvedValueOnce(null);

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByTestId("staff-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/personal", {
        scroll: false,
      });
    });
  });

  it("shows an error toast when delete returns an error", async () => {
    setSelectedStaff("1");
    mockDelete.mockResolvedValueOnce("Personal kann nicht gelöscht werden");

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByTestId("staff-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Personal kann nicht gelöscht werden",
      );
    });
  });

  it("opens the caregiver modal when the detail panel surfaces it", async () => {
    setSelectedStaff("1");

    render(<TeachersPage />);

    await waitFor(() => {
      expect(screen.getByTestId("staff-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-caregiver"));

    await waitFor(() => {
      expect(screen.getByTestId("caregiver-modal")).toBeInTheDocument();
    });
  });
});
