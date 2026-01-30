/**
 * Tests for RoleCreateModal
 * Tests the rendering and functionality of the role creation modal
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RoleCreateModal } from "./role-create-modal";

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
  }: {
    onSubmit: () => void;
    onCancel: () => void;
    isLoading: boolean;
    submitLabel: string;
  }) => (
    <div data-testid="database-form">
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
      createModalTitle: "Neue Rolle",
    },
    theme: {
      primaryColor: "purple",
    },
    form: {
      sections: [],
      defaultValues: {},
    },
  },
}));

describe("RoleCreateModal", () => {
  const mockOnClose = vi.fn();
  const mockOnCreate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the modal when open", async () => {
    render(
      <RoleCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <RoleCreateModal
        isOpen={false}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("displays the correct title", async () => {
    render(
      <RoleCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Neue Rolle")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <RoleCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });
  });

  it("renders the database form when not loading", async () => {
    render(
      <RoleCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
        loading={false}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("database-form")).toBeInTheDocument();
    });
  });

  it("calls onClose when modal close button is clicked", async () => {
    render(
      <RoleCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
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

  it("calls onCreate when submit button is clicked", async () => {
    mockOnCreate.mockResolvedValue(undefined);

    render(
      <RoleCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("submit-button")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("submit-button"));
    });

    expect(mockOnCreate).toHaveBeenCalledTimes(1);
  });
});
