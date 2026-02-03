/**
 * Tests for TeacherRoleManagementModal Component
 * Tests the rendering and basic functionality of teacher role management
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  TeacherRoleManagementModal,
  RoleInfo,
} from "./teacher-role-management-modal";
import type { Teacher } from "~/lib/teacher-api";
import type { Role } from "~/lib/auth-helpers";

// Mock functions using vi.hoisted to avoid hoisting issues
const { mockToastSuccess, mockGetRoles, mockGetAccountRoles } = vi.hoisted(
  () => ({
    mockToastSuccess: vi.fn(),
    mockGetRoles: vi.fn(() => Promise.resolve([] as Role[])),
    mockGetAccountRoles: vi.fn(() => Promise.resolve([] as Role[])),
  }),
);

// Mock ToastContext
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

// Mock auth-service
vi.mock("~/lib/auth-service", () => ({
  authService: {
    getRoles: mockGetRoles,
    getAccountRoles: mockGetAccountRoles,
    assignRoleToAccount: vi.fn(() => Promise.resolve()),
    removeRoleFromAccount: vi.fn(() => Promise.resolve()),
  },
}));

// Mock auth-helpers
vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (name: string) => `Display ${name}`,
  getRoleDisplayDescription: (name: string, description: string) =>
    description || name,
}));

// Mock UI components
vi.mock("~/components/ui", () => ({
  FormModal: ({
    isOpen,
    children,
    title,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    title: string;
  }) =>
    isOpen ? (
      <div data-testid="form-modal">
        <h1>{title}</h1>
        {children}
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

describe("RoleInfo", () => {
  it("renders role name", () => {
    const mockRole: Role = {
      id: "1",
      name: "teacher",
      description: "Teacher role",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-01T00:00:00Z",
    };

    render(<RoleInfo role={mockRole} />);

    expect(screen.getByText(/Display teacher/i)).toBeInTheDocument();
  });

  it("renders role description", () => {
    const mockRole: Role = {
      id: "1",
      name: "teacher",
      description: "Teacher role",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-01T00:00:00Z",
    };

    render(<RoleInfo role={mockRole} />);

    expect(screen.getByText(/Teacher role/i)).toBeInTheDocument();
  });

  it("renders permission count when permissions exist", () => {
    const mockRole: Role = {
      id: "1",
      name: "teacher",
      description: "Teacher role",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-01T00:00:00Z",
      permissions: [
        {
          id: "1",
          name: "Read",
          description: "Read permission",
          resource: "students",
          action: "read",
          createdAt: "2024-01-01T00:00:00Z",
          updatedAt: "2024-01-01T00:00:00Z",
        },
        {
          id: "2",
          name: "Write",
          description: "Write permission",
          resource: "students",
          action: "write",
          createdAt: "2024-01-01T00:00:00Z",
          updatedAt: "2024-01-01T00:00:00Z",
        },
      ],
    };

    render(<RoleInfo role={mockRole} />);

    expect(screen.getByText(/2 Berechtigungen/i)).toBeInTheDocument();
  });
});

describe("TeacherRoleManagementModal", () => {
  const mockTeacher = {
    id: "1",
    name: "John Doe",
    first_name: "John",
    last_name: "Doe",
    email: "john@example.com",
    account_id: 123,
  } as Teacher;

  const mockRoles: Role[] = [
    {
      id: "1",
      name: "teacher",
      description: "Teacher role",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-01T00:00:00Z",
    },
    {
      id: "2",
      name: "admin",
      description: "Admin role",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-01T00:00:00Z",
    },
  ];

  const mockOnClose = vi.fn();
  const mockOnUpdate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetRoles.mockResolvedValue(mockRoles);
    mockGetAccountRoles.mockResolvedValue([]);
  });

  it("renders the modal when open", async () => {
    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("form-modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <TeacherRoleManagementModal
        isOpen={false}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    expect(screen.queryByTestId("form-modal")).not.toBeInTheDocument();
  });

  it("displays the teacher name in title", async () => {
    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Rollen verwalten - John Doe/i),
      ).toBeInTheDocument();
    });
  });

  it("displays loading state initially", async () => {
    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    expect(screen.getByText(/Laden.../i)).toBeInTheDocument();
  });

  it("loads roles when opened", async () => {
    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(mockGetRoles).toHaveBeenCalled();
      expect(mockGetAccountRoles).toHaveBeenCalledWith("123");
    });
  });

  it("displays role statistics", async () => {
    mockGetAccountRoles.mockResolvedValue([mockRoles[0]!]);

    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Zugewiesene Rollen:/i)).toBeInTheDocument();
    });
  });

  it("displays both tabs", async () => {
    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Zugewiesen/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Verfügbare Rollen/i }),
      ).toBeInTheDocument();
    });
  });

  it("displays search input", async () => {
    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByPlaceholderText(/Rollen suchen.../i),
      ).toBeInTheDocument();
    });
  });

  it("switches tabs when clicked", async () => {
    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      const availableTab = screen.getByRole("button", {
        name: /Verfügbare Rollen/i,
      });
      fireEvent.click(availableTab);
    });

    // Modal should still be visible
    expect(screen.getByTestId("form-modal")).toBeInTheDocument();
  });

  it("handles teacher without account_id", async () => {
    const teacherWithoutAccount: Teacher = {
      ...mockTeacher,
      account_id: undefined,
    };

    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={teacherWithoutAccount}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/kein verknüpftes Konto/i)).toBeInTheDocument();
    });
  });

  it("displays empty state for no roles", async () => {
    mockGetAccountRoles.mockResolvedValue([]);

    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Keine Rollen zugewiesen/i)).toBeInTheDocument();
    });
  });

  it("displays add button in available tab", async () => {
    mockGetRoles.mockResolvedValue(mockRoles);
    mockGetAccountRoles.mockResolvedValue([]);

    render(
      <TeacherRoleManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      const availableTab = screen.getByRole("button", {
        name: /Verfügbare Rollen/i,
      });
      fireEvent.click(availableTab);
    });

    await waitFor(() => {
      expect(screen.getByText(/Wählen Sie Rollen aus/i)).toBeInTheDocument();
    });
  });
});
