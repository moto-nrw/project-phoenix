/**
 * Tests for TeacherPermissionManagementModal Component
 * Tests the rendering and basic functionality of teacher permission management
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { TeacherPermissionManagementModal } from "./teacher-permission-management-modal";
import type { Teacher } from "~/lib/teacher-api";
import type { Permission } from "~/lib/auth-helpers";

// Mock functions using vi.hoisted to avoid hoisting issues
const {
  mockToastSuccess,
  mockGetPermissions,
  mockGetAccountPermissions,
  mockGetAccountDirectPermissions,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockGetPermissions: vi.fn(() => Promise.resolve([] as Permission[])),
  mockGetAccountPermissions: vi.fn(() => Promise.resolve([] as Permission[])),
  mockGetAccountDirectPermissions: vi.fn(() =>
    Promise.resolve([] as Permission[]),
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
    getAccountPermissions: mockGetAccountPermissions,
    getAccountDirectPermissions: mockGetAccountDirectPermissions,
    assignPermissionToAccount: vi.fn(() => Promise.resolve()),
    removePermissionFromAccount: vi.fn(() => Promise.resolve()),
  },
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

describe("TeacherPermissionManagementModal", () => {
  const mockTeacher = {
    id: "1",
    name: "John Doe",
    first_name: "John",
    last_name: "Doe",
    email: "john@example.com",
    account_id: 123,
  } as Teacher;

  const mockPermissions: Permission[] = [
    {
      id: "1",
      name: "Read Students",
      description: "Can read students",
      resource: "students",
      action: "read",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-01T00:00:00Z",
    },
    {
      id: "2",
      name: "Write Students",
      description: "Can write students",
      resource: "students",
      action: "write",
      createdAt: "2024-01-01T00:00:00Z",
      updatedAt: "2024-01-01T00:00:00Z",
    },
  ];

  const mockOnClose = vi.fn();
  const mockOnUpdate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetPermissions.mockResolvedValue(mockPermissions);
    mockGetAccountPermissions.mockResolvedValue([]);
    mockGetAccountDirectPermissions.mockResolvedValue([]);
  });

  it("renders the modal when open", async () => {
    render(
      <TeacherPermissionManagementModal
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
      <TeacherPermissionManagementModal
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
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Berechtigungen verwalten - John Doe/i),
      ).toBeInTheDocument();
    });
  });

  it("displays loading state initially", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    expect(screen.getByText(/Laden.../i)).toBeInTheDocument();
  });

  it("loads permissions when opened", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(mockGetPermissions).toHaveBeenCalled();
      expect(mockGetAccountPermissions).toHaveBeenCalledWith("123");
      expect(mockGetAccountDirectPermissions).toHaveBeenCalledWith("123");
    });
  });

  it("displays stats cards", async () => {
    mockGetAccountPermissions.mockResolvedValue([mockPermissions[0]!]);
    mockGetAccountDirectPermissions.mockResolvedValue([mockPermissions[0]!]);

    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/Gesamte Berechtigungen:/i)).toBeInTheDocument();
      expect(screen.getByText(/Direkte Berechtigungen:/i)).toBeInTheDocument();
    });
  });

  it("displays all three tabs", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /Alle Berechtigungen/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Direkte \(/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Verfügbar/i }),
      ).toBeInTheDocument();
    });
  });

  it("displays search input", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByPlaceholderText(/Berechtigungen suchen.../i),
      ).toBeInTheDocument();
    });
  });

  it("switches tabs when clicked", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      const availableTab = screen.getByRole("button", { name: /Verfügbar/i });
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
      <TeacherPermissionManagementModal
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

  it("displays empty state for no permissions", async () => {
    mockGetAccountPermissions.mockResolvedValue([]);

    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Keine Berechtigungen zugewiesen/i),
      ).toBeInTheDocument();
    });
  });

  it("displays add button in available tab", async () => {
    mockGetPermissions.mockResolvedValue(mockPermissions);
    mockGetAccountDirectPermissions.mockResolvedValue([]);

    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      const availableTab = screen.getByRole("button", { name: /Verfügbar/i });
      fireEvent.click(availableTab);
    });

    await waitFor(() => {
      expect(
        screen.getByText(/Wählen Sie Berechtigungen aus/i),
      ).toBeInTheDocument();
    });
  });
});
