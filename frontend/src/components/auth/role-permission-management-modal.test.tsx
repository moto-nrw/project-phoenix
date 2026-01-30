/**
 * Tests for RolePermissionManagementModal Component
 * Tests the rendering and basic functionality of role permission management
 */
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RolePermissionManagementModal } from "./role-permission-management-modal";
import type { Role, Permission } from "~/lib/auth-helpers";

// Mock functions using vi.hoisted to avoid hoisting issues
const { mockToastSuccess, mockGetPermissions, mockGetRolePermissions } =
  vi.hoisted(() => ({
    mockToastSuccess: vi.fn(),
    mockGetPermissions: vi.fn((): Promise<unknown[]> => Promise.resolve([])),
    mockGetRolePermissions: vi.fn(
      (): Promise<unknown[]> => Promise.resolve([]),
    ),
  }));

// Mock ToastContext
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

// Mock auth-service
vi.mock("~/lib/auth-service", () => ({
  authService: {
    getPermissions: mockGetPermissions,
    getRolePermissions: mockGetRolePermissions,
    assignPermissionToRole: vi.fn(() => Promise.resolve()),
    removePermissionFromRole: vi.fn(() => Promise.resolve()),
  },
}));

// Mock auth-helpers
vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (name: string) => name,
}));

// Mock permission-labels
vi.mock("~/lib/permission-labels", () => ({
  localizeAction: (action: string) => action,
  localizeResource: (resource: string) => resource,
  formatPermissionDisplay: (resource: string, action: string) =>
    `${resource}:${action}`,
}));

// Mock UI components
vi.mock("~/components/ui", () => ({
  FormModal: ({
    isOpen,
    children,
    title,
    footer,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    title: string;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="form-modal">
        <h1>{title}</h1>
        {children}
        {footer}
      </div>
    ) : null,
}));

vi.mock("~/components/simple/SimpleAlert", () => ({
  SimpleAlert: ({
    type,
    message,
    onClose,
  }: {
    type: string;
    message: string;
    onClose: () => void;
  }) => (
    <div data-testid={`alert-${type}`}>
      {message}
      <button onClick={onClose}>Close</button>
    </div>
  ),
}));

describe("RolePermissionManagementModal", () => {
  const mockRole: Role = {
    id: "1",
    name: "teacher",
    description: "Teacher role",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };

  const mockPermissions: Permission[] = [
    {
      id: "1",
      name: "Read Students",
      description: "Can read students",
      resource: "students",
      action: "read",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
    {
      id: "2",
      name: "Write Students",
      description: "Can write students",
      resource: "students",
      action: "write",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    },
  ];

  const mockOnClose = vi.fn();
  const mockOnUpdate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetPermissions.mockResolvedValue(mockPermissions);
    mockGetRolePermissions.mockResolvedValue([]);
  });

  it("renders the modal when open", async () => {
    render(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("form-modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <RolePermissionManagementModal
        isOpen={false}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    expect(screen.queryByTestId("form-modal")).not.toBeInTheDocument();
  });

  it("displays the role name in title", async () => {
    render(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Berechtigungen verwalten - teacher/i),
      ).toBeInTheDocument();
    });
  });

  it("displays loading state initially", async () => {
    render(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    expect(screen.getByText(/Laden.../i)).toBeInTheDocument();
  });

  it("loads permissions when opened", async () => {
    render(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(mockGetPermissions).toHaveBeenCalled();
      expect(mockGetRolePermissions).toHaveBeenCalledWith(mockRole.id);
    });
  });

  it("displays search input", async () => {
    render(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByPlaceholderText(/Berechtigungen suchen.../i),
      ).toBeInTheDocument();
    });
  });

  it("displays assigned permissions count", async () => {
    mockGetRolePermissions.mockResolvedValue([mockPermissions[0]]);

    render(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Zugewiesene Berechtigungen/i),
      ).toBeInTheDocument();
    });
  });

  it("renders save and cancel buttons", async () => {
    render(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Abbrechen/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Speichern/i }),
      ).toBeInTheDocument();
    });
  });

  it("displays permissions after loading", async () => {
    render(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/students:read/i)).toBeInTheDocument();
      expect(screen.getByText(/students:write/i)).toBeInTheDocument();
    });
  });

  it("handles empty permissions list", async () => {
    mockGetPermissions.mockResolvedValue([]);

    render(
      <RolePermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Keine Berechtigungen gefunden/i),
      ).toBeInTheDocument();
    });
  });
});
