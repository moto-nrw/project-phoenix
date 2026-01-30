/**
 * Tests for RoleEditModal
 * Tests the rendering and functionality of the role edit modal
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RoleEditModal } from "./role-edit-modal";
import type { Role } from "@/lib/auth-helpers";

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
    submitLabel,
    initialData,
  }: {
    onSubmit: () => void;
    onCancel: () => void;
    isLoading: boolean;
    submitLabel: string;
    initialData?: Record<string, unknown>;
  }) => (
    <div data-testid="database-form">
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

// Mock roles config
vi.mock("@/lib/database/configs/roles.config", () => ({
  rolesConfig: {
    labels: {
      editModalTitle: "Rolle bearbeiten",
    },
    theme: {
      primaryColor: "purple",
    },
    form: {
      sections: [],
    },
  },
}));

describe("RoleEditModal", () => {
  const mockRole: Role = {
    id: "1",
    name: "Test Role",
    description: "Test role description",
    createdAt: "2024-01-01T00:00:00Z",
    updatedAt: "2024-01-01T00:00:00Z",
  };

  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the modal when open with role data", async () => {
    render(
      <RoleEditModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <RoleEditModal
        isOpen={false}
        onClose={mockOnClose}
        role={mockRole}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("returns null when role is null", () => {
    const { container } = render(
      <RoleEditModal
        isOpen={true}
        onClose={mockOnClose}
        role={null}
        onSave={mockOnSave}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("displays the correct title", async () => {
    render(
      <RoleEditModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Rolle bearbeiten")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <RoleEditModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
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
      <RoleEditModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
        onSave={mockOnSave}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
    });
  });

  it("passes role data as initial data to form", async () => {
    render(
      <RoleEditModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
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
      expect(initialData).toHaveProperty("id", "1");
      expect(initialData).toHaveProperty("name", "Test Role");
    });
  });

  it("calls onClose when modal close button is clicked", async () => {
    render(
      <RoleEditModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
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
      <RoleEditModal
        isOpen={true}
        onClose={mockOnClose}
        role={mockRole}
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
