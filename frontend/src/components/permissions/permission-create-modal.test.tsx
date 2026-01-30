/**
 * Tests for PermissionCreateModal
 * Tests the rendering and functionality of the permission creation modal
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PermissionCreateModal } from "./permission-create-modal";

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
  }: {
    onSubmit: () => void;
    onCancel: () => void;
    isLoading: boolean;
    error?: string | null;
    submitLabel: string;
  }) => (
    <div data-testid="database-form">
      {error && <div data-testid="form-error">{error}</div>}
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
      createModalTitle: "Neue Berechtigung",
    },
    theme: {
      primaryColor: "pink",
    },
    form: {
      sections: [],
      defaultValues: {},
    },
  },
}));

describe("PermissionCreateModal", () => {
  const mockOnClose = vi.fn();
  const mockOnCreate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the modal when open", async () => {
    render(
      <PermissionCreateModal
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
      <PermissionCreateModal
        isOpen={false}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("displays the correct title", async () => {
    render(
      <PermissionCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Neue Berechtigung")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <PermissionCreateModal
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
      <PermissionCreateModal
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

  it("displays error message when error prop is provided", async () => {
    const errorMessage = "Test error message";

    render(
      <PermissionCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
        error={errorMessage}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("form-error")).toHaveTextContent(errorMessage);
    });
  });

  it("calls onClose when modal close button is clicked", async () => {
    render(
      <PermissionCreateModal
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
      <PermissionCreateModal
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
