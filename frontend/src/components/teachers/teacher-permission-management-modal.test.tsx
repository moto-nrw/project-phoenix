import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// Mock modules that have environment dependencies
vi.mock("~/lib/auth-service", () => ({
  authService: {
    getPermissions: vi.fn(),
    getAccountPermissions: vi.fn(),
    getAccountDirectPermissions: vi.fn(),
    assignPermissionToAccount: vi.fn(),
    removePermissionFromAccount: vi.fn(),
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
  }),
}));

// Import after mocks are set up
import { TeacherPermissionManagementModal } from "./teacher-permission-management-modal";
import { authService } from "~/lib/auth-service";

const mockPermission = {
  id: "1",
  name: "read_users",
  description: "Can read user data",
  resource: "users",
  action: "read",
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

const mockPermission2 = {
  id: "2",
  name: "write_users",
  description: "Can write user data",
  resource: "users",
  action: "write",
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

const mockTeacher = {
  id: "1",
  name: "Max Mustermann",
  account_id: 123,
  staff_id: 1,
  email: "max@example.com",
};

const mockTeacherNoAccount = {
  id: "2",
  name: "No Account Teacher",
  account_id: null,
  staff_id: 2,
  email: "no-account@example.com",
};

describe("TeacherPermissionManagementModal", () => {
  const mockOnClose = vi.fn();
  const mockOnUpdate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(authService.getPermissions).mockResolvedValue([
      mockPermission,
      mockPermission2,
    ]);
    vi.mocked(authService.getAccountPermissions).mockResolvedValue([
      mockPermission,
    ]);
    vi.mocked(authService.getAccountDirectPermissions).mockResolvedValue([
      mockPermission,
    ]);
  });

  it("renders modal title with teacher name", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    expect(
      screen.getByText(`Berechtigungen verwalten - ${mockTeacher.name}`),
    ).toBeInTheDocument();
  });

  it("shows message when teacher has no account", () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacherNoAccount}
        onUpdate={mockOnUpdate}
      />,
    );

    expect(
      screen.getByText("Dieser Lehrer hat kein verknüpftes Konto."),
    ).toBeInTheDocument();
  });

  it("displays loading state while fetching permissions", () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    expect(screen.getByText("Laden...")).toBeInTheDocument();
  });

  it("displays permission counts after loading", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });

    expect(screen.getByText("Gesamte Berechtigungen:")).toBeInTheDocument();
    expect(screen.getByText("Direkte Berechtigungen:")).toBeInTheDocument();
  });

  it("renders tabs for different permission views", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });

    expect(screen.getByText(/Alle Berechtigungen/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Direkte/ })).toBeInTheDocument();
    expect(screen.getByText(/Verfügbar/)).toBeInTheDocument();
  });

  it("filters permissions when searching", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });

    const searchInput = screen.getByPlaceholderText("Berechtigungen suchen...");
    fireEvent.change(searchInput, { target: { value: "nonexistent" } });

    // Should show empty message when no matches
    await waitFor(() => {
      expect(
        screen.getByText("Keine Berechtigungen zugewiesen"),
      ).toBeInTheDocument();
    });
  });

  it("switches to available tab and shows available permissions", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });

    // Click on "Verfügbar" tab
    const availableTab = screen.getByText(/Verfügbar/);
    fireEvent.click(availableTab);

    // Should show the add button
    await waitFor(() => {
      expect(
        screen.getByText("Wählen Sie Berechtigungen aus"),
      ).toBeInTheDocument();
    });
  });

  it("handles error when fetching permissions fails", async () => {
    vi.mocked(authService.getPermissions).mockRejectedValue(
      new Error("Network error"),
    );

    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });
  });

  it("does not fetch permissions when modal is closed", () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={false}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    expect(authService.getPermissions).not.toHaveBeenCalled();
  });

  it("shows direct tab content correctly", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });

    // Click on "Direkte" tab (the button with parentheses count)
    const directTab = screen.getByRole("button", { name: /Direkte \(\d+\)/ });
    fireEvent.click(directTab);

    // Should show direct permissions
    await waitFor(() => {
      expect(screen.getByText("read_users")).toBeInTheDocument();
    });
  });

  it("shows badge for direct permissions", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });

    // Should show "Direkt" badge (green badge) for direct permissions
    const badges = screen.getAllByText("Direkt");
    // There will be the tab button and the badge - badge has specific class
    const directBadge = badges.find((el) =>
      el.className.includes("bg-green-100"),
    );
    expect(directBadge).toBeDefined();
  });

  it("handles assigning permissions", async () => {
    vi.mocked(authService.assignPermissionToAccount).mockResolvedValue(
      undefined,
    );

    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });

    // Click on "Verfügbar" tab
    const availableTab = screen.getByText(/Verfügbar/);
    fireEvent.click(availableTab);

    // Wait for available permissions to load
    await waitFor(() => {
      expect(screen.getByText("write_users")).toBeInTheDocument();
    });

    // Select the checkbox for the available permission
    const checkbox = screen.getByRole("checkbox", {
      name: /Berechtigung write_users aktivieren/i,
    });
    fireEvent.click(checkbox);

    // Click assign button
    const assignButton = screen.getByText(/1 Berechtigungen hinzufügen/);
    fireEvent.click(assignButton);

    await waitFor(() => {
      expect(authService.assignPermissionToAccount).toHaveBeenCalledWith(
        "123",
        "2",
      );
    });
  });

  it("handles removing permissions", async () => {
    vi.mocked(authService.removePermissionFromAccount).mockResolvedValue(
      undefined,
    );

    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });

    // Click on "Direkte" tab (the button with parentheses count)
    const directTab = screen.getByRole("button", { name: /Direkte \(\d+\)/ });
    fireEvent.click(directTab);

    // Wait for direct permissions to show
    await waitFor(() => {
      expect(screen.getByText("Entfernen")).toBeInTheDocument();
    });

    // Click remove button
    const removeButton = screen.getByText("Entfernen");
    fireEvent.click(removeButton);

    await waitFor(() => {
      expect(authService.removePermissionFromAccount).toHaveBeenCalledWith(
        "123",
        "1",
      );
    });
  });

  it("shows warning when no permissions selected for assignment", async () => {
    render(
      <TeacherPermissionManagementModal
        isOpen={true}
        onClose={mockOnClose}
        teacher={mockTeacher}
        onUpdate={mockOnUpdate}
      />,
    );

    await waitFor(() => {
      expect(screen.queryByText("Laden...")).not.toBeInTheDocument();
    });

    // Click on "Verfügbar" tab
    const availableTab = screen.getByText(/Verfügbar/);
    fireEvent.click(availableTab);

    // Click assign button without selecting any permissions
    await waitFor(() => {
      const assignButton = screen.getByText("Wählen Sie Berechtigungen aus");
      expect(assignButton).toBeDisabled();
    });
  });
});
