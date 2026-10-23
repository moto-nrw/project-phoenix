import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import RolesPage from "./page";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

/** Text of a FormErrorInput, as the shared kit components render it. */
function errorText(error: unknown): string | null {
  if (!error) return null;
  return typeof error === "string"
    ? error
    : (error as { message: string }).message;
}

const mockUseSession = vi.hoisted(() => vi.fn());

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
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
const mockRemove = vi.fn();
vi.mock("@/lib/database/service-factory", () => {
  const service = () => ({
    getList: mockGetList,
    getOne: mockGetOne,
    create: mockCreate,
    update: mockUpdate,
    remove: mockRemove,
  });
  return {
    createCrudService: vi.fn(service),
  };
});

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
    error: mockToastError,
  })),
}));

vi.mock("~/components/ui/confirm-delete-modal", () => ({
  ConfirmDeleteModal: ({
    isOpen,
    onConfirm,
    onClose,
    error,
  }: {
    isOpen: boolean;
    onConfirm?: () => void;
    onClose?: () => void;
    error?: unknown;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        {errorText(error) ? (
          <span data-testid="delete-error">{errorText(error)}</span>
        ) : null}
        <button type="button" data-testid="confirm-delete" onClick={onConfirm}>
          Confirm
        </button>
        <button type="button" data-testid="cancel-delete" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({
    children,
    loading,
    intro,
    search,
    error,
    empty,
    overlays,
  }: {
    children: ReactNode;
    loading: boolean;
    intro?: { title: string; description?: ReactNode; actions?: ReactNode };
    search?: ReactNode;
    error?: unknown;
    empty?: {
      title: string;
      description?: string;
      icon?: ReactNode;
      action?: ReactNode;
    } | null;
    overlays?: ReactNode;
  }) => (
    <div data-testid="database-layout" data-loading={loading}>
      {intro ? (
        <div data-testid="page-intro">
          <h1>{intro.title}</h1>
          {intro.description}
          {intro.actions}
          {search}
        </div>
      ) : null}
      {/* Fehler und Leerzustand liefert das Geruest, nicht die Seite. */}
      {error ? <div data-testid="page-error">{errorText(error)}</div> : null}
      {!error && empty ? (
        <div data-testid="page-empty">
          <p>{empty.title}</p>
          {empty.description ? <p>{empty.description}</p> : null}
          {empty.action}
        </div>
      ) : null}
      {!error && !empty ? children : null}
      {overlays}
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

vi.mock("~/components/ui/database/database-form-modal", () => ({
  // One mock serves both the create and the edit instance; the edit modal is
  // the one that receives initialData. Mirrors DatabaseForm: catches the
  // rejection from onSubmit and hands it to the shared error path, whose
  // catalog text it renders inline.
  DatabaseFormModal: ({
    isOpen,
    onClose,
    onSubmit,
    initialData,
    errorPath,
    errorObject,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (data: { name: string }) => Promise<void>;
    initialData?: unknown;
    errorPath?: {
      error: unknown;
      show: (error: unknown, options: { object: string }) => unknown;
    };
    errorObject?: string;
  }) => {
    const isEdit = initialData !== undefined;
    const submit = (data: { name: string }) => {
      void onSubmit(data).catch((err: unknown) => {
        void errorPath?.show(err, { object: errorObject ?? "" });
      });
    };
    const error = errorText(errorPath?.error);
    if (!isOpen) return null;
    return isEdit ? (
      <div data-testid="role-edit-form">
        {error ? <span data-testid="edit-error">{error}</span> : null}
        <button
          type="button"
          data-testid="submit-edit"
          onClick={() => submit({ name: "Updated" })}
        >
          Save
        </button>
        <button type="button" data-testid="cancel-edit" onClick={onClose}>
          Close
        </button>
      </div>
    ) : (
      <div data-testid="role-create-modal">
        {error ? <span data-testid="create-error">{error}</span> : null}
        <button
          type="button"
          data-testid="submit-create"
          onClick={() => submit({ name: "Neue Rolle" })}
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
    );
  },
}));

vi.mock("@/components/roles/roles-master-detail", () => ({
  // Bearbeitet wird jetzt im Detailbereich (BAUARTEN-SPEC Bauart 2 Regel 3).
  // Der Doppel steht fuer die eingebettete DatabaseForm: sie faengt die
  // Ablehnung von onSaveRole ab und zeigt die Meldung im Formular.
  RolesMasterDetail: ({
    roles,
    selectedId,
    selectedRole,
    onSelect,
    onSaveRole,
    onDeleteClick,
    onPermissionsSaved,
    canManagePermissions,
  }: {
    roles: Array<{ id: string; name: string }>;
    selectedId: string | null;
    selectedRole?: { name: string } | null;
    onSelect: (id: string | null) => void;
    onSaveRole: (data: { name: string }) => Promise<void>;
    onDeleteClick: () => void;
    onPermissionsSaved: () => void | Promise<void>;
    canManagePermissions?: boolean;
  }) => {
    const [editing, setEditing] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const submit = () => {
      setError(null);
      void onSaveRole({ name: "Updated" })
        .then(() => setEditing(false))
        .catch((err: unknown) => {
          setError(err instanceof Error ? err.message : String(err));
        });
    };
    return (
      <div
        data-testid="roles-master-detail"
        data-can-manage-permissions={canManagePermissions}
      >
        {roles.map((role) => (
          <button
            type="button"
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
            <button
              type="button"
              data-testid="trigger-edit"
              onClick={() => setEditing(true)}
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
              data-testid="trigger-permissions-saved"
              onClick={() => void onPermissionsSaved()}
            >
              Permissions saved
            </button>
            <button
              type="button"
              data-testid="trigger-deselect"
              onClick={() => onSelect(null)}
            >
              Close
            </button>
            {editing ? (
              <div data-testid="role-edit-form">
                {error ? <span data-testid="edit-error">{error}</span> : null}
                <button
                  type="button"
                  data-testid="submit-edit"
                  onClick={submit}
                >
                  Save
                </button>
                <button
                  type="button"
                  data-testid="cancel-edit"
                  onClick={() => setEditing(false)}
                >
                  Cancel
                </button>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    );
  },
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
    mockUseSession.mockReturnValue({
      data: { user: { id: "1", token: "test-token" }, expires: "2099-01-01" },
      status: "authenticated",
    });
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

  it("does not enable permission editing with roles:manage alone", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "test-token",
          permissions: ["roles:manage"],
        },
        expires: "2099-01-01",
      },
      status: "authenticated",
    });

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("roles-master-detail")).toHaveAttribute(
        "data-can-manage-permissions",
        "false",
      );
    });
  });

  it("does not enable permission editing without roles:read", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "test-token",
          permissions: ["roles:manage", "permissions:read"],
        },
        expires: "2099-01-01",
      },
      status: "authenticated",
    });

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("roles-master-detail")).toHaveAttribute(
        "data-can-manage-permissions",
        "false",
      );
    });
  });

  it("does not expose permissions to users who only create accounts", async () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "test-token",
          permissions: ["users:create"],
        },
        expires: "2099-01-01",
      },
      status: "authenticated",
    });

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("roles-master-detail")).toHaveAttribute(
        "data-can-manage-permissions",
        "false",
      );
    });
  });

  it("shows error message when fetch fails", async () => {
    mockGetList.mockRejectedValueOnce(
      new ApiError("Failed to fetch", 503, { code: "general.unavailable" }),
    );

    render(<RolesPage />);

    expect(await screen.findByTestId("page-error")).toHaveTextContent(
      catalogText("general.unavailable", "die Liste der Rollen"),
    );
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

  it("shows a taken role name with its own text in the dialog (Issue #1356)", async () => {
    mockCreate.mockRejectedValueOnce(
      new ApiError("role name taken", 409, {
        code: "identity.role_name_taken",
        errors: [{ field: "name", reason: "taken" }],
      }),
    );

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Vertretungslehrkraft")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Rolle erstellen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("role-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        catalogText("identity.role_name_taken", "die Rolle"),
      );
    });
    // The modal must NOT close so the user can correct the name.
    expect(screen.getByTestId("role-create-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("shows the catalog text when create fails for another reason", async () => {
    mockCreate.mockRejectedValueOnce(
      new ApiError("network unreachable", 503, {
        code: "general.unavailable",
      }),
    );

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByText("Vertretungslehrkraft")).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByLabelText("Rolle erstellen")[0]!);
    await waitFor(() => {
      expect(screen.getByTestId("role-create-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-create"));

    await waitFor(() => {
      expect(screen.getByTestId("create-error")).toHaveTextContent(
        catalogText("general.unavailable", "die Rolle"),
      );
    });
    expect(screen.getByTestId("create-error")).not.toHaveTextContent(
      "network unreachable",
    );
    expect(screen.getByTestId("role-create-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  // Die Anzeige übernimmt das Formular im Reiter (errorPath, #2517); die
  // Seite reicht den Fehler unverändert weiter.
  it("passes an update failure unchanged to the detail form", async () => {
    setSelectedRole("1");
    mockUpdate.mockRejectedValueOnce(new Error("server timeout"));

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("role-edit-form")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("submit-edit"));

    await waitFor(() => {
      expect(screen.getByTestId("edit-error")).toHaveTextContent(
        "server timeout",
      );
    });
    expect(screen.getByTestId("role-edit-form")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
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

  it("opens the inline edit form when the detail panel edit button is clicked", async () => {
    setSelectedRole("1");

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));

    await waitFor(() => {
      expect(screen.getByTestId("role-edit-form")).toBeInTheDocument();
    });
  });

  // Berechtigungen werden im Reiter des Detailbereichs bearbeitet (#3116);
  // die Seite lädt danach Liste und Detail neu, damit die Zahl in der Liste
  // stimmt.
  it("reloads the roles and the detail after the permissions were saved", async () => {
    setSelectedRole("1");

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
      expect(mockGetOne).toHaveBeenCalledTimes(1);
    });
    const listCallsBefore = mockGetList.mock.calls.length;
    const detailCallsBefore = mockGetOne.mock.calls.length;

    fireEvent.click(screen.getByTestId("trigger-permissions-saved"));

    await waitFor(() => {
      expect(mockGetList.mock.calls.length).toBe(listCallsBefore + 1);
    });
    await waitFor(() => {
      expect(mockGetOne.mock.calls.length).toBe(detailCallsBefore + 1);
    });
    expect(mockGetOne).toHaveBeenLastCalledWith("1");
  });

  it("calls update service when saving the inline edit form", async () => {
    setSelectedRole("1");
    mockUpdate.mockResolvedValueOnce(undefined);

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-edit"));
    await waitFor(() => {
      expect(screen.getByTestId("role-edit-form")).toBeInTheDocument();
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
    mockRemove.mockResolvedValueOnce(true);

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
      expect(mockRemove).toHaveBeenCalledWith("1");
      expect(mockReplace).toHaveBeenCalledWith("/tenant/database/roles", {
        scroll: false,
      });
    });
  });

  it("keeps a delete error in the confirmation dialog", async () => {
    setSelectedRole("1");
    mockRemove.mockRejectedValueOnce(
      new ApiError("role in use", 409, {
        code: "general.business_rejection",
      }),
    );

    render(<RolesPage />);

    await waitFor(() => {
      expect(screen.getByTestId("role-detail-panel")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("trigger-delete"));
    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-delete"));

    expect(await screen.findByTestId("delete-error")).toHaveTextContent(
      catalogText("general.business_rejection", "das Löschen der Rolle"),
    );
    expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    expect(mockToastSuccess).not.toHaveBeenCalled();
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
