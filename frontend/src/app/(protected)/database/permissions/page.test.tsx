import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import PermissionsPage from "./page";

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

vi.mock("@/lib/database/configs/permissions.config", () => ({
  permissionsConfig: {
    name: { singular: "Berechtigung", plural: "Berechtigungen" },
    form: {
      transformBeforeSubmit: (data: unknown) => data,
    },
  },
}));

vi.mock("@/lib/permission-labels", () => ({
  formatPermissionDisplay: (resource: string, action: string) =>
    `${resource}:${action}`,
  localizeAction: (action: string) => action,
  localizeResource: (resource: string) => resource,
}));

vi.mock("@/components/permissions", () => ({
  PermissionCreateModal: ({
    isOpen,
    onClose,
    onCreate,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onCreate: (data: { resource: string; action: string }) => void;
  }) =>
    isOpen ? (
      <div data-testid="create-modal">
        <button
          data-testid="create-submit"
          onClick={() => onCreate({ resource: "test", action: "read" })}
        >
          Create
        </button>
        <button data-testid="create-close" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  PermissionDetailModal: ({
    isOpen,
    onClose,
    permission,
    onEdit,
  }: {
    isOpen: boolean;
    onClose: () => void;
    permission: { name: string };
    onEdit: () => void;
    onDelete: () => void;
    loading?: boolean;
    onDeleteClick: () => void;
  }) =>
    isOpen ? (
      <div data-testid="detail-modal">
        <span data-testid="detail-name">{permission.name}</span>
        <button data-testid="edit-button" onClick={onEdit}>
          Edit
        </button>
        <button data-testid="detail-close" onClick={onClose}>
          Close
        </button>
      </div>
    ) : null,
  PermissionEditModal: ({
    isOpen,
    onClose,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    permission: { name: string };
    onSave: (data: { name: string }) => void;
    loading?: boolean;
    error?: string | null;
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

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({ isOpen }: { isOpen: boolean }) =>
    isOpen ? <div data-testid="confirm-modal">Confirm</div> : null,
}));

const mockPermissions = [
  {
    id: "1",
    name: "Read Students",
    description: "Can read student data",
    resource: "students",
    action: "read",
  },
  {
    id: "2",
    name: "Write Students",
    description: "Can write student data",
    resource: "students",
    action: "write",
  },
  {
    id: "3",
    name: "Admin Access",
    description: "Full admin access",
    resource: "admin",
    action: "all",
  },
];

describe("PermissionsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetList.mockResolvedValue({ data: mockPermissions });
    mockGetOne.mockResolvedValue(mockPermissions[0]);
  });

  it("renders permissions list", async () => {
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Read Students")).toBeInTheDocument();
      expect(screen.getByText("Write Students")).toBeInTheDocument();
      expect(screen.getByText("Admin Access")).toBeInTheDocument();
    });
  });

  it("shows loading state initially", () => {
    mockGetList.mockImplementation(
      () =>
        new Promise((resolve) => setTimeout(() => resolve({ data: [] }), 100)),
    );

    render(<PermissionsPage />);

    expect(screen.getByTestId("database-layout")).toHaveAttribute(
      "data-loading",
      "true",
    );
  });

  it("shows error message when fetch fails", async () => {
    mockGetList.mockRejectedValue(new Error("Fetch failed"));

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Laden der Berechtigungen/),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no permissions", async () => {
    mockGetList.mockResolvedValue({ data: [] });

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
      expect(screen.getByText("Read Students")).toBeInTheDocument();
    });

    const searchInput = screen.getByTestId("search-input");
    fireEvent.change(searchInput, { target: { value: "Admin" } });

    await waitFor(() => {
      expect(screen.getByText("Admin Access")).toBeInTheDocument();
      expect(screen.queryByText("Read Students")).not.toBeInTheDocument();
    });
  });

  it("opens create modal when create button is clicked", async () => {
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Read Students")).toBeInTheDocument();
    });

    const createButtons = screen.getAllByLabelText("Berechtigung erstellen");
    const firstButton = createButtons[0];
    if (firstButton) {
      fireEvent.click(firstButton);
    }

    await waitFor(() => {
      expect(screen.getByTestId("create-modal")).toBeInTheDocument();
    });
  });

  it("opens detail modal when permission is clicked", async () => {
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Read Students")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Read Students"));

    await waitFor(() => {
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument();
    });
  });

  it("opens edit modal when edit button is clicked", async () => {
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Read Students")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Read Students"));

    await waitFor(() => {
      expect(screen.getByTestId("detail-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("edit-button"));

    await waitFor(() => {
      expect(screen.getByTestId("edit-modal")).toBeInTheDocument();
    });
  });

  it("creates permission successfully", async () => {
    mockCreate.mockResolvedValue({
      id: "4",
      resource: "test",
      action: "read",
    });

    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Read Students")).toBeInTheDocument();
    });

    const createButtons = screen.getAllByLabelText("Berechtigung erstellen");
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
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(screen.getByText("Read Students")).toBeInTheDocument();
    });

    const createButtons = screen.getAllByLabelText("Berechtigung erstellen");
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
});

describe("PermissionsPage filtering logic", () => {
  it("filters by name", () => {
    const permissions = [
      { id: "1", name: "Read Users", resource: "users", action: "read" },
      { id: "2", name: "Write Data", resource: "data", action: "write" },
    ];

    const searchTerm = "read";
    const filtered = permissions.filter(
      (p) =>
        p.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        p.resource.toLowerCase().includes(searchTerm.toLowerCase()) ||
        p.action.toLowerCase().includes(searchTerm.toLowerCase()),
    );

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe("Read Users");
  });

  it("filters by resource", () => {
    const permissions = [
      { id: "1", name: "Read Users", resource: "users", action: "read" },
      { id: "2", name: "Write Data", resource: "data", action: "write" },
    ];

    const searchTerm = "data";
    const filtered = permissions.filter(
      (p) =>
        p.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        p.resource.toLowerCase().includes(searchTerm.toLowerCase()) ||
        p.action.toLowerCase().includes(searchTerm.toLowerCase()),
    );

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe("Write Data");
  });

  it("sorts permissions by resource then action", () => {
    const permissions = [
      { id: "1", name: "B", resource: "users", action: "write" },
      { id: "2", name: "A", resource: "admin", action: "read" },
      { id: "3", name: "C", resource: "users", action: "read" },
    ];

    const sorted = [...permissions].sort((a, b) => {
      const r = a.resource.localeCompare(b.resource, "de");
      if (r !== 0) return r;
      return a.action.localeCompare(b.action, "de");
    });

    expect(sorted[0]?.resource).toBe("admin");
    expect(sorted[1]?.resource).toBe("users");
    expect(sorted[1]?.action).toBe("read");
    expect(sorted[2]?.resource).toBe("users");
    expect(sorted[2]?.action).toBe("write");
  });
});

describe("displayTitle helper logic", () => {
  it("returns name when name is present", () => {
    const perm = { name: "Test Permission", resource: "test", action: "read" };
    const displayTitle = (p: typeof perm) =>
      p.name?.trim() ? p.name : `${p.resource}:${p.action}`;

    expect(displayTitle(perm)).toBe("Test Permission");
  });

  it("returns resource:action when name is empty", () => {
    const perm = { name: "", resource: "test", action: "read" };
    const displayTitle = (p: typeof perm) =>
      p.name?.trim() ? p.name : `${p.resource}:${p.action}`;

    expect(displayTitle(perm)).toBe("test:read");
  });

  it("returns resource:action when name is whitespace", () => {
    const perm = { name: "   ", resource: "test", action: "read" };
    const displayTitle = (p: typeof perm) =>
      p.name?.trim() ? p.name : `${p.resource}:${p.action}`;

    expect(displayTitle(perm)).toBe("test:read");
  });
});
