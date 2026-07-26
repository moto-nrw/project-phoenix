import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PermissionsPage from "./page";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { id: "1", token: "test-token" }, expires: "2099-01-01" },
    status: "authenticated",
  })),
}));

let currentSearch = new URLSearchParams();
const mockReplace = vi.fn((url: string) => {
  const query = url.includes("?") ? (url.split("?")[1] ?? "") : "";
  currentSearch = new URLSearchParams(query);
});
const setSelectedPermission = (id: string | null) => {
  currentSearch = new URLSearchParams();
  if (id) {
    currentSearch.set("permission", id);
  }
};

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: mockReplace })),
  usePathname: vi.fn(() => "/tenant/database/permissions"),
  useSearchParams: () => currentSearch,
}));

const mockGetList = vi.fn();
const mockGetOne = vi.fn();
const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: mockGetList,
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

// Captures the most recent thrown error from onCreate/onSave so tests can
// assert the rethrown message bubbles up to the form layer.
const lastModalError = { message: "" };

vi.mock("~/components/ui/database/database-form-modal", () => ({
  // The page renders the create modal via DatabaseFormModal directly; the
  // edit modal keeps its PermissionEditModal wrapper (mocked below).
  DatabaseFormModal: ({
    isOpen,
    onClose,
    onSubmit,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: { resource: string; action: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="permission-create-modal">
        <button
          type="button"
          data-testid="submit-create"
          onClick={() => {
            lastModalError.message = "";
            onSubmit({ resource: "students", action: "delete" }).catch(
              (err: unknown) => {
                lastModalError.message =
                  err instanceof Error ? err.message : String(err);
              },
            );
          }}
        >
          Submit
        </button>
        <button
          type="button"
          data-testid="close-create-modal"
          onClick={onClose}
        >
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("@/components/permissions/permission-edit-modal", () => ({
  PermissionEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: { description: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="permission-edit-modal">
        <button
          type="button"
          data-testid="submit-edit"
          onClick={() => {
            lastModalError.message = "";
            onSave({ description: "Updated" }).catch((err: unknown) => {
              lastModalError.message =
                err instanceof Error ? err.message : String(err);
            });
          }}
        >
          Save
        </button>
        <button type="button" data-testid="close-edit-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("@/components/permissions/permissions-master-detail", () => ({
  PermissionsMasterDetail: ({
    permissions,
    selectedId,
    selectedPermission,
    onSelect,
    onEditClick,
    onDeleteClick,
  }: {
    permissions: Array<{ id: string; resource: string; action: string }>;
    selectedId: string | null;
    selectedPermission?: { resource: string; action: string } | null;
    onSelect: (id: string | null) => void;
    onEditClick: () => void;
    onDeleteClick: () => void;
  }) => (
    <div data-testid="permissions-master-detail">
      {permissions.map((permission) => (
        <button
          type="button"
          key={permission.id}
          data-testid={`permission-row-${permission.id}`}
          onClick={() => onSelect(permission.id)}
        >
          {permission.resource}:{permission.action}
        </button>
      ))}
      {selectedId ? (
        <div data-testid="permission-detail-panel">
          <span data-testid="detail-selected-id">{selectedId}</span>
          <span data-testid="detail-permission-name">
            {selectedPermission?.resource}:{selectedPermission?.action}
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
        </div>
      ) : null}
    </div>
  ),
}));

vi.mock("~/components/ui/modal", () => ({
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

const mockPermissions = [
  {
    id: "1",
    name: "students:read",
    description: "Kinderdaten lesen",
    resource: "students",
    action: "read",
    createdAt: "2026-01-01",
    updatedAt: "2026-01-02",
  },
  {
    id: "2",
    name: "students:write",
    description: "Kinderdaten bearbeiten",
    resource: "students",
    action: "write",
    createdAt: "2026-01-01",
    updatedAt: "2026-01-02",
  },
];

describe("PermissionsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();

    mockGetList.mockResolvedValue({ data: mockPermissions });
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(
        mockPermissions.find((permission) => permission.id === id) ?? null,
      ),
    );
  });

  it("renders the page with permissions data", async () => {
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("students:read")).toBeInTheDocument();
      expect(screen.getByText("students:write")).toBeInTheDocument();
    });
  });

  it("shows error message when fetch fails", async () => {
    mockGetList.mockRejectedValueOnce(new Error("Failed to fetch"));

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Berechtigungen/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no permissions exist", async () => {
    mockGetList.mockResolvedValueOnce({ data: [] });

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Berechtigungen vorhanden"),
      ).toBeInTheDocument();
    });
  });

  it("filters permissions by search term", async () => {
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("students:read")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "write" },
    });

    await waitFor(() => {
      expect(screen.queryByText("students:read")).not.toBeInTheDocument();
      expect(screen.getByText("students:write")).toBeInTheDocument();
    });
  });

  it("opens create modal when add button is clicked", async () => {
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("students:read")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Berechtigung erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("permission-create-modal")).toBeInTheDocument();
    });
  });

  it("syncs permission selection into the URL when a row is clicked", async () => {
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("students:read")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("permission-row-1"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/tenant/database/permissions?permission=1",
        { scroll: false },
      );
    });
  });

  it("hydrates the detail panel from the permission URL param", async () => {
    setSelectedPermission("1");

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("permission-detail-panel")).toBeInTheDocument();
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("1");
    });
  });

  it("opens the edit modal when the detail panel edit button is clicked", async () => {
    setSelectedPermission("1");

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("permission-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));

    await waitFor(() => {
      expect(screen.getByTestId("permission-edit-modal")).toBeInTheDocument();
    });
  });

  it("calls update service when saving the edit modal", async () => {
    setSelectedPermission("1");
    mockUpdate.mockResolvedValueOnce(undefined);

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("permission-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("permission-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ description: "Updated" }),
      );
    });
  });

  it("calls delete service after confirming deletion from the detail panel", async () => {
    setSelectedPermission("1");
    mockDelete.mockResolvedValueOnce(null);

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("permission-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/permissions", {
        scroll: false,
      });
    });
  });

  it("re-throws German duplicate-key message when create hits 23505", async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    mockCreate.mockRejectedValueOnce(
      new Error('duplicate key value violates unique constraint "..."'),
    );

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("students:read")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Berechtigung erstellen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("permission-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(lastModalError.message).toContain(
        'Die Berechtigung "students:delete" existiert bereits',
      );
    });
    expect(consoleError).toHaveBeenCalledWith("permission_create_failed", {
      error: expect.stringContaining("duplicate key"),
    });
    // Modal stays open on failure so the user can retry.
    expect(screen.getByTestId("permission-create-modal")).toBeInTheDocument();
    consoleError.mockRestore();
  });

  it("re-throws fallback message when create fails for non-duplicate reason", async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    mockCreate.mockRejectedValueOnce(new Error("network unreachable"));

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("students:read")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Berechtigung erstellen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("permission-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(lastModalError.message).toBe(
        "Fehler beim Erstellen der Berechtigung. Bitte versuchen Sie es erneut.",
      );
    });
    expect(consoleError).toHaveBeenCalledWith("permission_create_failed", {
      error: "network unreachable",
    });
    consoleError.mockRestore();
  });

  it("re-throws German duplicate-key message when update hits 23505", async () => {
    setSelectedPermission("1");
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    mockUpdate.mockRejectedValueOnce(
      new Error("constraint violation: 23505 unique"),
    );

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("permission-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("permission-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(lastModalError.message).toContain("existiert bereits");
    });
    expect(consoleError).toHaveBeenCalledWith("permission_update_failed", {
      permission_id: "1",
      error: expect.stringContaining("23505"),
    });
    consoleError.mockRestore();
  });

  it("re-throws fallback message when update fails for non-duplicate reason", async () => {
    setSelectedPermission("1");
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});
    mockUpdate.mockRejectedValueOnce(new Error("server timeout"));

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("permission-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("permission-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(lastModalError.message).toBe(
        "Fehler beim Aktualisieren der Berechtigung. Bitte versuchen Sie es erneut.",
      );
    });
    expect(consoleError).toHaveBeenCalledWith("permission_update_failed", {
      permission_id: "1",
      error: "server timeout",
    });
    consoleError.mockRestore();
  });

  it("shows an error toast when delete returns an error", async () => {
    setSelectedPermission("1");
    mockDelete.mockResolvedValueOnce("Berechtigung kann nicht gelöscht werden");

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("permission-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Berechtigung kann nicht gelöscht werden",
      );
    });
  });
});
