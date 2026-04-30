import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import RolesPage from "./page";

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
const setSelectedRole = (id: string | null) => {
  currentSearch = new URLSearchParams();
  if (id) {
    currentSearch.set("role", id);
  }
};

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: mockReplace })),
  usePathname: vi.fn(() => "/tenant/database/roles"),
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

vi.mock("~/hooks/useIsMobile", () => ({
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

vi.mock("~/components/ui/page-header", () => ({
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
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
      {actionButton}
    </div>
  ),
}));

vi.mock("@/components/roles", () => ({
  RoleCreateModal: ({
    isOpen,
    onClose,
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="role-create-modal">
        <button
          data-testid="submit-create"
          onClick={() => void onCreate({ name: "Neue Rolle" })}
        >
          Submit
        </button>
        <button data-testid="close-create-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  RoleEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSave: (data: { name: string }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="role-edit-modal">
        <button
          data-testid="submit-edit"
          onClick={() => void onSave({ name: "Updated" })}
        >
          Save
        </button>
        <button data-testid="close-edit-modal" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  RolesMasterDetail: ({
    roles,
    selectedId,
    selectedRole,
    onSelect,
    onEditClick,
    onDeleteClick,
    onManagePermissions,
  }: {
    roles: Array<{ id: string; name: string }>;
    selectedId: string | null;
    selectedRole?: { name: string } | null;
    onSelect: (id: string | null) => void;
    onEditClick: () => void;
    onDeleteClick: () => void;
    onManagePermissions: () => void;
  }) => (
    <div data-testid="roles-master-detail">
      {roles.map((role) => (
        <button
          key={role.id}
          data-testid={`role-row-${role.id}`}
          onClick={() => onSelect(role.id)}
        >
          {role.name}
        </button>
      ))}
      {selectedId ? (
        <div data-testid="role-detail-panel">
          <span data-testid="detail-selected-id">{selectedId}</span>
          <span data-testid="detail-role-name">
            {selectedRole?.name ?? "unbekannt"}
          </span>
          <button data-testid="trigger-edit" onClick={onEditClick}>
            Edit
          </button>
          <button data-testid="trigger-delete" onClick={onDeleteClick}>
            Delete
          </button>
          <button
            data-testid="trigger-permissions"
            onClick={onManagePermissions}
          >
            Permissions
          </button>
          <button data-testid="trigger-deselect" onClick={() => onSelect(null)}>
            Close
          </button>
        </div>
      ) : null}
    </div>
  ),
}));

vi.mock("@/components/auth", () => ({
  RolePermissionManagementModal: ({ isOpen }: { isOpen: boolean }) =>
    isOpen ? <div data-testid="role-permission-modal" /> : null,
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
        <button data-testid="confirm-delete" onClick={onConfirm}>
          Confirm
        </button>
        <button data-testid="cancel-delete" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

const mockRoles = [
  {
    id: "1",
    name: "Vertretungslehrkraft",
    description: "Vertritt im Krankheitsfall",
    isSystem: false,
    baseRole: "teacher",
    createdAt: "2026-01-01",
    updatedAt: "2026-01-02",
    permissions: [],
  },
  {
    id: "2",
    name: "admin",
    description: "Administrator",
    isSystem: true,
    baseRole: "admin",
    createdAt: "2026-01-01",
    updatedAt: "2026-01-02",
  },
];

describe("RolesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentSearch = new URLSearchParams();

    mockGetList.mockResolvedValue({ data: mockRoles });
    mockGetOne.mockImplementation((id: string) =>
      Promise.resolve(mockRoles.find((role) => role.id === id) ?? null),
    );
  });

  it("renders the page with roles data", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Vertretungslehrkraft")).toBeInTheDocument();
      // System role label is mapped via getRoleDisplayName; "admin" will map.
    });
  });

  it("shows error message when fetch fails", async () => {
    mockGetList.mockRejectedValueOnce(new Error("Failed to fetch"));

    render(<RolesPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Rollen/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no roles exist", async () => {
    mockGetList.mockResolvedValueOnce({ data: [] });

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Rollen vorhanden")).toBeInTheDocument();
    });
  });

  it("filters roles by search term", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Vertretungslehrkraft")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Vertretung" },
    });

    await waitFor(() => {
      expect(screen.getByText("Vertretungslehrkraft")).toBeInTheDocument();
    });
  });

  it("clears filters when clear button is clicked", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Vertretungslehrkraft")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Vertretung" },
    });
    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByTestId("search-input")).toHaveValue("");
    });
  });

  it("opens create modal when add button is clicked", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Vertretungslehrkraft")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Rolle erstellen")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("role-create-modal")).toBeInTheDocument();
    });
  });

  it("syncs role selection into the URL when a row is clicked", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Vertretungslehrkraft")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("role-row-1"));

    await waitFor(() => {
      expect(mockReplace).toHaveBeenCalledWith(
        "/tenant/database/roles?role=1",
        { scroll: false },
      );
    });
  });

  it("hydrates the detail panel from the role URL param and fetches detail data", async () => {
    setSelectedRole("1");

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
      expect(screen.getByTestId("detail-selected-id")).toHaveTextContent("1");
      expect(mockGetOne).toHaveBeenCalledWith("1");
    });
  });

  it("opens the edit modal when the detail panel edit button is clicked", async () => {
    setSelectedRole("1");

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));

    await waitFor(() => {
      expect(screen.getByTestId("role-edit-modal")).toBeInTheDocument();
    });
  });

  it("opens the permission management modal from the detail panel", async () => {
    setSelectedRole("1");

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-permissions"));

    await waitFor(() => {
      expect(screen.getByTestId("role-permission-modal")).toBeInTheDocument();
    });
  });

  it("calls update service when saving the edit modal", async () => {
    setSelectedRole("1");
    mockUpdate.mockResolvedValueOnce(undefined);

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("role-edit-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        expect.objectContaining({ name: "Updated" }),
      );
    });
  });

  it("calls delete service after confirming deletion from the detail panel", async () => {
    setSelectedRole("1");
    mockDelete.mockResolvedValueOnce(null);

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith("1");
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/roles", {
        scroll: false,
      });
    });
  });

  it("shows an error toast when delete returns an error", async () => {
    setSelectedRole("1");
    mockDelete.mockResolvedValueOnce("Rolle kann nicht gelöscht werden");

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith(
        "Rolle kann nicht gelöscht werden",
      );
    });
  });

  it("renders the unclassified-roles warning banner", async () => {
    mockGetList.mockResolvedValueOnce({
      data: [
        {
          id: "3",
          name: "Helfer",
          description: "",
          isSystem: false,
          createdAt: "2026-01-01",
          updatedAt: "2026-01-02",
        },
      ],
    });

    render(<RolesPage />);

    await waitFor(() => {
      expect(
        screen.getByText("1 Rolle hat keine Systemrollen-Zuordnung"),
      ).toBeInTheDocument();
    });
  });
});
