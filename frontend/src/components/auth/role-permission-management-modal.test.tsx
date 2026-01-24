/**
 * Tests for RolePermissionManagementModal component
 *
 * This file tests:
 * - Modal rendering
 * - Permission loading and display
 * - Permission filtering (search)
 * - Permission toggling
 * - Save functionality
 * - Error handling
 */

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, type Mock } from "vitest";
import { RolePermissionManagementModal } from "./role-permission-management-modal";
import { authService } from "~/lib/auth-service";
import type { Role, Permission } from "~/lib/auth-helpers";

// Mock toast context
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
  }),
}));

// Mock auth service
vi.mock("~/lib/auth-service", () => ({
  authService: {
    getPermissions: vi.fn(),
    getRolePermissions: vi.fn(),
    assignPermissionToRole: vi.fn(),
    removePermissionFromRole: vi.fn(),
  },
}));

// Mock permission labels - return plain values since component adds prefixes
vi.mock("~/lib/permission-labels", () => ({
  localizeAction: (action: string) => action,
  localizeResource: (resource: string) => resource,
  formatPermissionDisplay: (resource: string, action: string) =>
    `${resource}:${action}`,
  getPermissionDisplayName: (permission: {
    resource: string;
    action: string;
  }) => `${permission.resource}:${permission.action}`,
}));

// Mock auth helpers
vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (name: string) => `Role: ${name}`,
}));

const mockRole: Role = {
  id: "role-1",
  name: "teacher",
  createdAt: new Date(),
  updatedAt: new Date(),
  organizationId: "org-1",
};

const mockAllPermissions: Permission[] = [
  {
    id: "perm-1",
    name: "View Students",
    description: "Can view student data",
    resource: "students",
    action: "read",
    createdAt: new Date(),
    updatedAt: new Date(),
  },
  {
    id: "perm-2",
    name: "Edit Students",
    description: "Can edit student data",
    resource: "students",
    action: "write",
    createdAt: new Date(),
    updatedAt: new Date(),
  },
  {
    id: "perm-3",
    name: "View Rooms",
    description: "Can view room data",
    resource: "rooms",
    action: "read",
    createdAt: new Date(),
    updatedAt: new Date(),
  },
];

const mockRolePermissions: Permission[] = [mockAllPermissions[0]!];

const defaultProps = {
  isOpen: true,
  onClose: vi.fn(),
  role: mockRole,
  onUpdate: vi.fn(),
};

describe("RolePermissionManagementModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (authService.getPermissions as Mock).mockResolvedValue(mockAllPermissions);
    (authService.getRolePermissions as Mock).mockResolvedValue(
      mockRolePermissions,
    );
    (authService.assignPermissionToRole as Mock).mockResolvedValue({});
    (authService.removePermissionFromRole as Mock).mockResolvedValue({});
  });

  // =============================================================================
  // Rendering Tests
  // =============================================================================

  describe("rendering", () => {
    it("renders modal when isOpen is true", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByText("Berechtigungen verwalten - Role: teacher"),
        ).toBeInTheDocument();
      });
    });

    it("shows loading state while fetching permissions", async () => {
      let resolvePromise: ((value: unknown) => void) | undefined;
      (authService.getPermissions as Mock).mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvePromise = resolve;
          }),
      );

      render(<RolePermissionManagementModal {...defaultProps} />);

      expect(screen.getByText("Laden...")).toBeInTheDocument();

      // Cleanup
      resolvePromise?.(mockAllPermissions);
    });

    it("displays assigned permissions count", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByText("Zugewiesene Berechtigungen"),
        ).toBeInTheDocument();
        // The count "1" should be displayed
        expect(screen.getAllByText("1").length).toBeGreaterThanOrEqual(1);
      });
    });

    it("displays hint text", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByText(/Aktiviere oder deaktiviere Berechtigungen/),
        ).toBeInTheDocument();
      });
    });

    it("renders search input", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByPlaceholderText("Berechtigungen suchen..."),
        ).toBeInTheDocument();
      });
    });

    it("renders cancel and save buttons", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Abbrechen" }),
        ).toBeInTheDocument();
        expect(
          screen.getByRole("button", { name: "Speichern" }),
        ).toBeInTheDocument();
      });
    });

    it("disables save button when no changes made", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Speichern" }),
        ).toBeDisabled();
      });
    });
  });

  // =============================================================================
  // Permission Display Tests
  // =============================================================================

  describe("permission display", () => {
    it("displays all permissions with formatted names", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText("students:read")).toBeInTheDocument();
        expect(screen.getByText("students:write")).toBeInTheDocument();
        expect(screen.getByText("rooms:read")).toBeInTheDocument();
      });
    });

    it("displays permission details", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        // The text is split across multiple nodes, so search for each part separately
        // Find the resource labels
        expect(screen.getAllByText(/Ressource:/).length).toBeGreaterThan(0);
        expect(screen.getAllByText(/Aktion:/).length).toBeGreaterThan(0);
      });
    });

    it("shows correct toggle state for assigned permissions", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        // First permission should be toggled on (assigned)
        const switches = screen.getAllByRole("switch");
        expect(switches[0]).toHaveAttribute("aria-checked", "true");
        // Second and third should be off
        expect(switches[1]).toHaveAttribute("aria-checked", "false");
        expect(switches[2]).toHaveAttribute("aria-checked", "false");
      });
    });
  });

  // =============================================================================
  // Search/Filter Tests
  // =============================================================================

  describe("search functionality", () => {
    it("filters permissions by name", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText("students:read")).toBeInTheDocument();
      });

      const searchInput = screen.getByPlaceholderText(
        "Berechtigungen suchen...",
      );
      fireEvent.change(searchInput, { target: { value: "rooms" } });

      // Only rooms permission should be visible
      expect(screen.getByText("rooms:read")).toBeInTheDocument();
      expect(screen.queryByText("students:read")).not.toBeInTheDocument();
      expect(screen.queryByText("students:write")).not.toBeInTheDocument();
    });

    it("filters permissions by description", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText("students:read")).toBeInTheDocument();
      });

      const searchInput = screen.getByPlaceholderText(
        "Berechtigungen suchen...",
      );
      fireEvent.change(searchInput, { target: { value: "room data" } });

      // Only rooms permission should match the description
      expect(screen.getByText("rooms:read")).toBeInTheDocument();
    });

    it("filters permissions by resource", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText("students:read")).toBeInTheDocument();
      });

      const searchInput = screen.getByPlaceholderText(
        "Berechtigungen suchen...",
      );
      fireEvent.change(searchInput, { target: { value: "students" } });

      // Both student permissions should be visible
      expect(screen.getByText("students:read")).toBeInTheDocument();
      expect(screen.getByText("students:write")).toBeInTheDocument();
      expect(screen.queryByText("rooms:read")).not.toBeInTheDocument();
    });

    it("filters permissions by action", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText("students:read")).toBeInTheDocument();
      });

      const searchInput = screen.getByPlaceholderText(
        "Berechtigungen suchen...",
      );
      fireEvent.change(searchInput, { target: { value: "write" } });

      // Only write permission should be visible
      expect(screen.getByText("students:write")).toBeInTheDocument();
      expect(screen.queryByText("students:read")).not.toBeInTheDocument();
    });

    it("shows no results message when filter returns empty", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText("students:read")).toBeInTheDocument();
      });

      const searchInput = screen.getByPlaceholderText(
        "Berechtigungen suchen...",
      );
      fireEvent.change(searchInput, { target: { value: "nonexistent" } });

      expect(
        screen.getByText("Keine Berechtigungen gefunden"),
      ).toBeInTheDocument();
    });

    it("is case insensitive", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getByText("students:read")).toBeInTheDocument();
      });

      const searchInput = screen.getByPlaceholderText(
        "Berechtigungen suchen...",
      );
      fireEvent.change(searchInput, { target: { value: "STUDENTS" } });

      // Both student permissions should be visible
      expect(screen.getByText("students:read")).toBeInTheDocument();
      expect(screen.getByText("students:write")).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Permission Toggling Tests
  // =============================================================================

  describe("permission toggling", () => {
    it("toggles permission on click", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getAllByRole("switch").length).toBe(3);
      });

      const switches = screen.getAllByRole("switch");
      // Toggle off the first permission (currently on)
      fireEvent.click(switches[0]!);

      expect(switches[0]).toHaveAttribute("aria-checked", "false");
    });

    it("enables save button after toggling", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Speichern" }),
        ).toBeDisabled();
      });

      const switches = screen.getAllByRole("switch");
      fireEvent.click(switches[1]!); // Toggle on second permission

      expect(
        screen.getByRole("button", { name: "Speichern" }),
      ).not.toBeDisabled();
    });

    it("disables save button when toggled back to original state", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Speichern" }),
        ).toBeDisabled();
      });

      const switches = screen.getAllByRole("switch");

      // Toggle off
      fireEvent.click(switches[0]!);
      expect(
        screen.getByRole("button", { name: "Speichern" }),
      ).not.toBeDisabled();

      // Toggle back on
      fireEvent.click(switches[0]!);
      expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    });
  });

  // =============================================================================
  // Save Functionality Tests
  // =============================================================================

  describe("save functionality", () => {
    it("calls assignPermissionToRole for newly assigned permissions", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getAllByRole("switch").length).toBe(3);
      });

      const switches = screen.getAllByRole("switch");
      // Toggle on second permission (currently off)
      fireEvent.click(switches[1]!);

      // Click save
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(authService.assignPermissionToRole).toHaveBeenCalledWith(
          "role-1",
          "perm-2",
        );
      });
    });

    it("calls removePermissionFromRole for removed permissions", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getAllByRole("switch").length).toBe(3);
      });

      const switches = screen.getAllByRole("switch");
      // Toggle off first permission (currently on)
      fireEvent.click(switches[0]!);

      // Click save
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(authService.removePermissionFromRole).toHaveBeenCalledWith(
          "role-1",
          "perm-1",
        );
      });
    });

    it("shows success toast and calls callbacks on successful save", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getAllByRole("switch").length).toBe(3);
      });

      const switches = screen.getAllByRole("switch");
      fireEvent.click(switches[1]!);

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Berechtigungen aktualisiert",
        );
      });

      await waitFor(() => {
        expect(defaultProps.onUpdate).toHaveBeenCalled();
        expect(defaultProps.onClose).toHaveBeenCalled();
      });
    });

    it("shows saving state during save", async () => {
      let resolvePromise: ((value: unknown) => void) | undefined;
      (authService.assignPermissionToRole as Mock).mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvePromise = resolve;
          }),
      );

      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getAllByRole("switch").length).toBe(3);
      });

      const switches = screen.getAllByRole("switch");
      fireEvent.click(switches[1]!);

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Wird gespeichert..." }),
        ).toBeInTheDocument();
      });

      // Cleanup
      resolvePromise?.({});
    });

    it("disables buttons during save", async () => {
      let resolvePromise: ((value: unknown) => void) | undefined;
      (authService.assignPermissionToRole as Mock).mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvePromise = resolve;
          }),
      );

      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getAllByRole("switch").length).toBe(3);
      });

      const switches = screen.getAllByRole("switch");
      fireEvent.click(switches[1]!);

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Abbrechen" }),
        ).toBeDisabled();
        expect(
          screen.getByRole("button", { name: "Wird gespeichert..." }),
        ).toBeDisabled();
      });

      // Cleanup
      resolvePromise?.({});
    });
  });

  // =============================================================================
  // Error Handling Tests
  // =============================================================================

  describe("error handling", () => {
    it("shows error alert when fetching permissions fails", async () => {
      const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {
        /* suppress */
      });
      (authService.getPermissions as Mock).mockRejectedValue(
        new Error("Fetch error"),
      );

      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByText("Fehler beim Laden der Berechtigungen"),
        ).toBeInTheDocument();
      });

      consoleSpy.mockRestore();
    });

    it("shows error alert when saving fails", async () => {
      const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {
        /* suppress */
      });
      (authService.assignPermissionToRole as Mock).mockRejectedValue(
        new Error("Save error"),
      );

      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(screen.getAllByRole("switch").length).toBe(3);
      });

      const switches = screen.getAllByRole("switch");
      fireEvent.click(switches[1]!);

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(
          screen.getByText("Fehler beim Aktualisieren der Berechtigungen"),
        ).toBeInTheDocument();
      });

      consoleSpy.mockRestore();
    });

    it("allows closing error alert", async () => {
      const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {
        /* suppress */
      });
      (authService.getPermissions as Mock).mockRejectedValue(
        new Error("Fetch error"),
      );

      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByText("Fehler beim Laden der Berechtigungen"),
        ).toBeInTheDocument();
      });

      // Find close button in alert - the button is inside the fixed alert container
      // SimpleAlert's close button doesn't have an accessible name, so find it by structure
      const alertText = screen.getByText(
        "Fehler beim Laden der Berechtigungen",
      );
      const alertContainer = alertText.closest("div[class*='fixed']");
      const closeButton = alertContainer?.querySelector("button");
      expect(closeButton).toBeTruthy();
      fireEvent.click(closeButton!);

      await waitFor(() => {
        expect(
          screen.queryByText("Fehler beim Laden der Berechtigungen"),
        ).not.toBeInTheDocument();
      });

      consoleSpy.mockRestore();
    });
  });

  // =============================================================================
  // Modal Control Tests
  // =============================================================================

  describe("modal controls", () => {
    it("calls onClose when cancel button is clicked", async () => {
      render(<RolePermissionManagementModal {...defaultProps} />);

      await waitFor(() => {
        expect(
          screen.getByRole("button", { name: "Abbrechen" }),
        ).toBeInTheDocument();
      });

      fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

      expect(defaultProps.onClose).toHaveBeenCalled();
    });

    it("fetches permissions when modal opens", async () => {
      const { rerender } = render(
        <RolePermissionManagementModal {...defaultProps} isOpen={false} />,
      );

      expect(authService.getPermissions).not.toHaveBeenCalled();

      rerender(
        <RolePermissionManagementModal {...defaultProps} isOpen={true} />,
      );

      await waitFor(() => {
        expect(authService.getPermissions).toHaveBeenCalled();
        expect(authService.getRolePermissions).toHaveBeenCalledWith("role-1");
      });
    });

    it("refetches permissions when role changes", async () => {
      const { rerender } = render(
        <RolePermissionManagementModal {...defaultProps} />,
      );

      await waitFor(() => {
        expect(authService.getRolePermissions).toHaveBeenCalledWith("role-1");
      });

      const newRole: Role = {
        ...mockRole,
        id: "role-2",
        name: "admin",
      };

      rerender(
        <RolePermissionManagementModal {...defaultProps} role={newRole} />,
      );

      await waitFor(() => {
        expect(authService.getRolePermissions).toHaveBeenCalledWith("role-2");
      });
    });
  });
});
