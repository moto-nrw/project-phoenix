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
vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: mockGetList,
  })),
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
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

vi.mock("@/components/permissions/permissions-master-detail", () => ({
  PermissionsMasterDetail: ({
    permissions,
    selectedId,
    selectedPermission,
    onSelect,
  }: {
    permissions: Array<{ id: string; resource: string; action: string }>;
    selectedId: string | null;
    selectedPermission?: { resource: string; action: string } | null;
    onSelect: (id: string | null) => void;
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

  it("shows that the catalog is read-only without mutation controls", async () => {
    render(<PermissionsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Sie können Berechtigungen ansehen. Nur das moto-Team kann sie ändern.",
        ),
      ).toBeInTheDocument();
    });
    expect(
      screen.queryByLabelText("Berechtigung erstellen"),
    ).not.toBeInTheDocument();
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
});
