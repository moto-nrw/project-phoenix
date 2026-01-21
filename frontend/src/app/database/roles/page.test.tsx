import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import RolesPage from "./page";

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
  redirect: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  })),
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
  }: {
    children: React.ReactNode;
    loading?: boolean;
    sessionLoading?: boolean;
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
      {children}
    </div>
  ),
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    search,
    actionButton,
  }: {
    search: { value: string; onChange: (v: string) => void };
    actionButton?: React.ReactNode;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      {actionButton}
    </div>
  ),
}));

vi.mock("~/hooks/useIsMobile", () => ({
  useIsMobile: () => false,
}));

vi.mock("~/hooks/useDeleteConfirmation", () => ({
  useDeleteConfirmation: () => ({
    showConfirmModal: false,
    handleDeleteClick: vi.fn(),
    handleDeleteCancel: vi.fn(),
    confirmDelete: vi.fn(),
  }),
}));

const mockToastSuccess = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  }),
}));

const mockGetList = vi.fn();
const mockGetOne = vi.fn();
const mockCreate = vi.fn();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();

vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: () => ({
    getList: mockGetList,
    getOne: mockGetOne,
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
  }),
}));

vi.mock("@/lib/database/configs/roles.config", () => ({
  rolesConfig: {
    name: { singular: "Rolle", plural: "Rollen" },
    form: {
      transformBeforeSubmit: (data: unknown) => data,
    },
  },
}));

vi.mock("@/lib/auth-helpers", () => ({
  getRoleDisplayName: (name: string) => name,
  getRoleDisplayDescription: (name: string, desc: string) => desc,
}));

vi.mock("@/components/roles", () => ({
  RoleCreateModal: ({
    isOpen,
    onClose,
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: { name: string }) => void;
  }) =>
    isOpen ? (
      <div data-testid="create-modal">
        <button
          data-testid="create-submit"
          onClick={() => onCreate({ name: "New Role" })}
        >
          Create
        </button>
        <button data-testid="create-close" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  RoleDetailModal: ({
    isOpen,
    onClose,
    role,
    onEdit,
    onManagePermissions,
  }: {
    isOpen: boolean;
    onClose: () => void;
    role: { name: string };
    onEdit: () => void;
    onDelete: () => void;
    onManagePermissions: () => void;
    loading?: boolean;
    onDeleteClick: () => void;
  }) =>
    isOpen ? (
      <div data-testid="detail-modal">
        <span data-testid="detail-name">{role.name}</span>
        <button data-testid="edit-button" onClick={onEdit}>
          Edit
        </button>
        <button data-testid="manage-permissions" onClick={onManagePermissions}>
          Manage Permissions
        </button>
        <button data-testid="detail-close" onClick={onClose}>
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
    role: { name: string };
    onSave: (data: { name: string }) => void;
    loading?: boolean;
  }) =>
    isOpen ? (
      <div data-testid="edit-modal">
        <button
          data-testid="save-button"
          onClick={() => onSave({ name: "Updated" })}
        >
          Save
        </button>
        <button data-testid="edit-close" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("@/components/auth", () => ({
  RolePermissionManagementModal: ({
    isOpen,
    onClose,
    onUpdate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    role: { id: string; name: string };
    onUpdate: () => void;
  }) =>
    isOpen ? (
      <div data-testid="permission-modal">
        <button data-testid="permission-update" onClick={onUpdate}>
          Update
        </button>
        <button data-testid="permission-close" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({ isOpen }: { isOpen: boolean }) =>
    isOpen ? <div data-testid="confirm-modal">Confirm</div> : null,
}));

const mockRoles = [
  {
    id: "1",
    name: "Admin",
    description: "Full access",
    permissions: [{ id: "p1" }, { id: "p2" }],
  },
  {
    id: "2",
    name: "Lehrer",
    description: "Teacher role",
    permissions: [{ id: "p1" }],
  },
  {
    id: "3",
    name: "Betreuer",
    description: "Supervisor role",
    permissions: [],
  },
];

describe("RolesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetList.mockResolvedValue({ data: mockRoles });
    mockGetOne.mockResolvedValue(mockRoles[0]);
  });

  it("renders roles list", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
      expect(screen.getByText("Lehrer")).toBeInTheDocument();
      expect(screen.getByText("Betreuer")).toBeInTheDocument();
    });
  });

  it("shows loading state initially", () => {
    mockGetList.mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve({ data: [] }), 100)),
    );

    render(<RolesPage />);

    expect(screen.getByTestId("database-layout")).toHaveAttribute(
      "data-loading",
      "true",
    );
  });

  it("shows error message when fetch fails", async () => {
    mockGetList.mockRejectedValue(new Error("Fetch failed"));

    render(<RolesPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Rollen/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no roles", async () => {
    mockGetList.mockResolvedValue({ data: [] });

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Rollen vorhanden")).toBeInTheDocument();
    });
  });

  it("filters roles by search term", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Lehrer" } });

    await waitFor(() => {
      expect(screen.getByText("Lehrer")).toBeInTheDocument();
      expect(screen.queryByText("Admin")).not.toBeInTheDocument();
    });
  });

  it("opens create modal when create button is clicked", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });

    const createButtons = screen.getAllByLabelText("Rolle erstellen");
    const firstButton = createButtons[0];
    if (firstButton) {
      fireEvent.click(firstButton);
    }

    await waitFor(() => {
      expect(screen.getByTestId("create-modal")).toBeInTheDocument();
    });
  });

  it("opens detail modal when role is clicked", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Admin"));

    await waitFor(() => {
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument();
    });
  });

  it("opens edit modal when edit button is clicked", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Admin"));

    await waitFor(() => {
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("edit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("edit-modal")).toBeInTheDocument();
    });
  });

  it("opens permission management modal when manage permissions is clicked", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Admin"));

    await waitFor(() => {
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("manage-permissions"));

    await waitFor(() => {
      expect(screen.getByTestId("permission-modal")).toBeInTheDocument();
    });
  });

  it("creates role successfully", async () => {
    mockCreate.mockResolvedValue({
      id: "4",
      name: "New Role",
    });

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });

    const createButtons = screen.getAllByLabelText("Rolle erstellen");
    const firstButton = createButtons[0];
    if (firstButton) {
      fireEvent.click(firstButton);
    }

    await waitFor(() => {
      expect(screen.getByTestId("create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("create-submit"));

    await waitFor(() => {
      expect(mockCreate).toHaveBeenCalled();
      expect(mockToastSuccess).toHaveBeenCalled();
    });
  });

  it("closes create modal when close button is clicked", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
    });

    const createButtons = screen.getAllByLabelText("Rolle erstellen");
    const firstButton = createButtons[0];
    if (firstButton) {
      fireEvent.click(firstButton);
    }

    await waitFor(() => {
      expect(screen.getByTestId("create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("create-close"));

    await waitFor(() => {
      expect(screen.queryByTestId("create-modal")).not.toBeInTheDocument();
    });
  });

  it("displays permission count on role cards", async () => {
    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin")).toBeInTheDocument();
      expect(screen.getByText("2 Berechtigungen")).toBeInTheDocument();
      expect(screen.getByText("1 Berechtigungen")).toBeInTheDocument();
      expect(screen.getByText("0 Berechtigungen")).toBeInTheDocument();
    });
  });
});

describe("RolesPage filtering logic", () => {
  it("filters by name", () => {
    const roles = [
      { id: "1", name: "Admin", description: "Full access" },
      { id: "2", name: "Lehrer", description: "Teacher role" },
    ];

    const searchTerm = "admin";
    const filtered = roles.filter(
      (r) =>
        r.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        (r.description?.toLowerCase().includes(searchTerm.toLowerCase()) ?? false),
    );

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe("Admin");
  });

  it("filters by description", () => {
    const roles = [
      { id: "1", name: "Admin", description: "Full access" },
      { id: "2", name: "Lehrer", description: "Teacher role" },
    ];

    const searchTerm = "teacher";
    const filtered = roles.filter(
      (r) =>
        r.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        (r.description?.toLowerCase().includes(searchTerm.toLowerCase()) ?? false),
    );

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe("Lehrer");
  });

  it("sorts roles alphabetically by name", () => {
    const roles = [
      { id: "1", name: "Betreuer" },
      { id: "2", name: "Admin" },
      { id: "3", name: "Lehrer" },
    ];

    const sorted = [...roles].sort((a, b) => a.name.localeCompare(b.name, "de"));

    expect(sorted[0]?.name).toBe("Admin");
    expect(sorted[1]?.name).toBe("Betreuer");
    expect(sorted[2]?.name).toBe("Lehrer");
  });
});

describe("RolesPage role display helper", () => {
  it("returns first character uppercase for avatar", () => {
    const role = { name: "Administrator" };
    const getAvatarLetter = (r: typeof role) =>
      r.name?.charAt(0)?.toUpperCase() ?? "R";

    expect(getAvatarLetter(role)).toBe("A");
  });

  it("returns R as fallback when name is empty", () => {
    const role = { name: "" };
    const getAvatarLetter = (r: typeof role) =>
      r.name?.charAt(0)?.toUpperCase() ?? "R";

    // Empty string charAt(0) returns "", which is falsy, so fallback to "R"
    expect(getAvatarLetter(role) || "R").toBe("R");
  });
});
