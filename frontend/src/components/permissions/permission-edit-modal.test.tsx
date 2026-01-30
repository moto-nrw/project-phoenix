/**
 * Tests for PermissionEditModal
 * Tests the rendering and functionality of the permission edit modal
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PermissionEditModal } from "./permission-edit-modal";
import type { Permission } from "@/lib/auth-helpers";

// Mock Modal component
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        <button onClick={onClose}>Close</button>
        {children}
      </div>
    ) : null,
}));

// Mock DatabaseForm component
vi.mock("~/components/ui/database/database-form", () => ({
  DatabaseForm: ({
    onSubmit,
    onCancel,
    isLoading,
    error,
    submitLabel,
    initialData,
  }: {
    onSubmit: () => void;
    onCancel: () => void;
    isLoading: boolean;
    error?: string | null;
    submitLabel: string;
    initialData?: Record<string, unknown>;
  }) => (
    <div data-testid="database-form">
      {error && <div data-testid="form-error">{error}</div>}
      <div data-testid="initial-data">{JSON.stringify(initialData)}</div>
      <button
        onClick={onSubmit}
        disabled={isLoading}
        data-testid="submit-button"
      >
        {submitLabel}
      </button>
      <button onClick={onCancel} data-testid="cancel-button">
        Cancel
      </button>
    </div>
  ),
}));

// Mock permissions config
vi.mock("@/lib/database/configs/permissions.config", () => ({
  permissionsConfig: {
    labels: {
      editModalTitle: "Berechtigung bearbeiten",
    },
    theme: {
      primaryColor: "pink",
    },
    form: {
      sections: [],
    },
  },
}));

// Mock configToFormSection
vi.mock("@/lib/database/types", () => ({
  configToFormSection: (section: unknown) => section,
}));

describe("PermissionEditModal", () => {
  const mockPermission: Permission = {
    id: "1",
    resource: "users",
    action: "read",
    name: "Read Users",
    description: "Allows reading user data",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };

  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the modal when open with permission data", async () => {
    render(
      <PermissionEditModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <PermissionEditModal
        isOpen={false}
        onClose={mockOnClose}
        permission={mockPermission}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("returns null when permission is null", () => {
    const { container } = render(
      <PermissionEditModal
        isOpen={true}
        onClose={mockOnClose}
        permission={null}
        onSave={mockOnSave}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("displays the correct title", async () => {
    render(
      <PermissionEditModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Berechtigung bearbeiten")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <PermissionEditModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onSave={mockOnSave}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });
  });

  it("renders the database form when not loading", async () => {
    render(
      <PermissionEditModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onSave={mockOnSave}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
    });
  });

  it("transforms permission data for form with permissionSelector", async () => {
    render(
      <PermissionEditModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      const initialDataText =
        screen.getByTestId("initial-data").textContent ?? "";
      const initialData = JSON.parse(initialDataText) as Record<
        string,
        unknown
      >;
      expect(initialData).toHaveProperty("permissionSelector");
      expect(initialData.permissionSelector).toEqual({
        resource: "users",
        action: "read",
      });
    });
  });

  it("displays error message when error prop is provided", async () => {
    const errorMessage = "Test error message";

    render(
      <PermissionEditModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onSave={mockOnSave}
        error={errorMessage}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("form-error")).toHaveTextContent(errorMessage);
    });
  });

  it("calls onClose when modal close button is clicked", async () => {
    render(
      <PermissionEditModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Close")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByText("Close"));
    });

    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  it("calls onSave when submit button is clicked", async () => {
    mockOnSave.mockResolvedValue(undefined);

    render(
      <PermissionEditModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("submit-button")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("submit-button"));
    });

    expect(mockOnSave).toHaveBeenCalledTimes(1);
  });
});
