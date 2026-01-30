/**
 * Tests for PermissionDetailModal
 * Tests the rendering and functionality of the permission detail view modal
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PermissionDetailModal } from "./permission-detail-modal";
import type { Permission } from "@/lib/auth-helpers";

// Mock Modal component
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <button onClick={onClose}>Close</button>
        {children}
      </div>
    ) : null,
}));

// Mock DetailModalActions component
vi.mock("~/components/ui/detail-modal-actions", () => ({
  DetailModalActions: ({
    onEdit,
    onDelete,
    onDeleteClick,
  }: {
    onEdit: () => void;
    onDelete: () => void;
    onDeleteClick?: () => void;
    entityName: string;
    entityType: string;
    confirmationContent: React.ReactNode;
  }) => (
    <div data-testid="detail-modal-actions">
      <button onClick={onEdit} data-testid="edit-button">
        Edit
      </button>
      <button onClick={onDeleteClick ?? onDelete} data-testid="delete-button">
        Delete
      </button>
    </div>
  ),
}));

// Mock permission labels
vi.mock("@/lib/permission-labels", () => ({
  formatPermissionDisplay: (resource: string, action: string) =>
    `${resource}: ${action}`,
}));

describe("PermissionDetailModal", () => {
  const mockPermission: Permission = {
    id: "1",
    resource: "users",
    action: "read",
    name: "Read Users",
    description: "Allows reading user data",
  };

  const mockOnClose = vi.fn();
  const mockOnEdit = vi.fn();
  const mockOnDelete = vi.fn();
  const mockOnDeleteClick = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the modal when open with permission data", async () => {
    render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <PermissionDetailModal
        isOpen={false}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("returns null when permission is null", () => {
    const { container } = render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={null}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("displays permission resource", async () => {
    render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("users")).toBeInTheDocument();
    });
  });

  it("displays permission action", async () => {
    render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("read")).toBeInTheDocument();
    });
  });

  it("displays permission name", async () => {
    render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getAllByText("Read Users").length).toBeGreaterThan(0);
    });
  });

  it("displays permission description", async () => {
    render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Allows reading user data")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });
  });

  it("calls onEdit when edit button is clicked", async () => {
    render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("edit-button")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("edit-button"));
    });

    expect(mockOnEdit).toHaveBeenCalledTimes(1);
  });

  it("calls onDeleteClick when provided and delete button is clicked", async () => {
    render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
        onDeleteClick={mockOnDeleteClick}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("delete-button")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("delete-button"));
    });

    expect(mockOnDeleteClick).toHaveBeenCalledTimes(1);
    expect(mockOnDelete).not.toHaveBeenCalled();
  });

  it("displays formatted permission title", async () => {
    render(
      <PermissionDetailModal
        isOpen={true}
        onClose={mockOnClose}
        permission={mockPermission}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("users: read")).toBeInTheDocument();
    });
  });
});
