/**
 * Tests for GuardianDeleteModal
 * Tests the rendering and functionality of the guardian delete confirmation modal
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { GuardianDeleteModal } from "./guardian-delete-modal";

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

describe("GuardianDeleteModal", () => {
  const mockOnClose = vi.fn();
  const mockOnConfirm = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the modal when open", async () => {
    render(
      <GuardianDeleteModal
        isOpen={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        guardianName="John Doe"
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <GuardianDeleteModal
        isOpen={false}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        guardianName="John Doe"
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("displays the guardian name", async () => {
    render(
      <GuardianDeleteModal
        isOpen={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        guardianName="John Doe"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText(/John Doe/)).toBeInTheDocument();
    });
  });

  it("displays warning message", async () => {
    render(
      <GuardianDeleteModal
        isOpen={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        guardianName="John Doe"
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Diese Aktion kann nicht rückgängig gemacht werden/i),
      ).toBeInTheDocument();
    });
  });

  it("displays delete confirmation heading", async () => {
    render(
      <GuardianDeleteModal
        isOpen={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        guardianName="John Doe"
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Erziehungsberechtigte\/n entfernen\?/i),
      ).toBeInTheDocument();
    });
  });

  it("calls onClose when cancel button is clicked", async () => {
    render(
      <GuardianDeleteModal
        isOpen={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        guardianName="John Doe"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Abbrechen")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByText("Abbrechen"));
    });

    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  it("calls onConfirm when delete button is clicked", async () => {
    render(
      <GuardianDeleteModal
        isOpen={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        guardianName="John Doe"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Entfernen")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByText("Entfernen"));
    });

    expect(mockOnConfirm).toHaveBeenCalledTimes(1);
  });

  it("disables buttons when loading", async () => {
    render(
      <GuardianDeleteModal
        isOpen={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        guardianName="John Doe"
        isLoading={true}
      />,
    );

    await waitFor(() => {
      const cancelButton = screen.getByText("Abbrechen");
      expect(cancelButton).toBeDisabled();
      // The "Wird entfernt..." text is inside a span, check the parent button
      const confirmText = screen.getByText("Wird entfernt...");
      const confirmButton = confirmText.closest("button");
      expect(confirmButton).toBeDisabled();
    });
  });

  it("shows loading text when loading", async () => {
    render(
      <GuardianDeleteModal
        isOpen={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        guardianName="John Doe"
        isLoading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Wird entfernt...")).toBeInTheDocument();
    });
  });
});
