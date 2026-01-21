import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { DetailModalActions } from "./detail-modal-actions";

// Mock the ConfirmationModal
vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onClose,
    onConfirm,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal">
        <h2>{title}</h2>
        <div>{children}</div>
        <button data-testid="confirm-btn" onClick={onConfirm}>
          Confirm
        </button>
        <button data-testid="cancel-btn" onClick={onClose}>
          Cancel
        </button>
      </div>
    ) : null,
}));

describe("DetailModalActions", () => {
  const defaultProps = {
    onEdit: vi.fn(),
    onDelete: vi.fn(),
    entityName: "Test Entity",
    entityType: "Gruppe",
  };

  it("renders edit and delete buttons", () => {
    render(<DetailModalActions {...defaultProps} />);

    expect(screen.getByText("Bearbeiten")).toBeInTheDocument();
    expect(screen.getByText("Löschen")).toBeInTheDocument();
  });

  it("calls onEdit when edit button is clicked", () => {
    const onEdit = vi.fn();
    render(<DetailModalActions {...defaultProps} onEdit={onEdit} />);

    fireEvent.click(screen.getByText("Bearbeiten"));

    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("opens confirmation modal when delete button is clicked (no custom handler)", async () => {
    render(<DetailModalActions {...defaultProps} />);

    fireEvent.click(screen.getByText("Löschen"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });
  });

  it("shows entity name in confirmation message", async () => {
    render(<DetailModalActions {...defaultProps} />);

    fireEvent.click(screen.getByText("Löschen"));

    await waitFor(() => {
      expect(screen.getByText("Test Entity")).toBeInTheDocument();
    });
  });

  it("calls onDelete when confirmation is confirmed", async () => {
    const onDelete = vi.fn();
    render(<DetailModalActions {...defaultProps} onDelete={onDelete} />);

    fireEvent.click(screen.getByText("Löschen"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("confirm-btn"));

    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it("closes confirmation modal when cancelled", async () => {
    render(<DetailModalActions {...defaultProps} />);

    fireEvent.click(screen.getByText("Löschen"));

    await waitFor(() => {
      expect(screen.getByTestId("confirmation-modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("cancel-btn"));

    await waitFor(() => {
      expect(screen.queryByTestId("confirmation-modal")).not.toBeInTheDocument();
    });
  });

  it("uses custom onDeleteClick handler when provided", () => {
    const onDeleteClick = vi.fn();
    render(
      <DetailModalActions {...defaultProps} onDeleteClick={onDeleteClick} />,
    );

    fireEvent.click(screen.getByText("Löschen"));

    expect(onDeleteClick).toHaveBeenCalledTimes(1);
    // Internal modal should not be shown
    expect(screen.queryByTestId("confirmation-modal")).not.toBeInTheDocument();
  });

  it("uses correct German article for Gerät", async () => {
    render(
      <DetailModalActions
        {...defaultProps}
        entityType="Gerät"
        entityName="RFID Reader"
      />,
    );

    fireEvent.click(screen.getByText("Löschen"));

    await waitFor(() => {
      // "das" is used for Gerät
      expect(screen.getByText(/das Gerät/)).toBeInTheDocument();
    });
  });

  it("uses die as default German article", async () => {
    render(
      <DetailModalActions
        {...defaultProps}
        entityType="Aktivität"
        entityName="Chess Club"
      />,
    );

    fireEvent.click(screen.getByText("Löschen"));

    await waitFor(() => {
      // "die" is used by default
      expect(screen.getByText(/die Aktivität/)).toBeInTheDocument();
    });
  });

  it("renders custom confirmation content when provided", async () => {
    render(
      <DetailModalActions
        {...defaultProps}
        confirmationContent={<p data-testid="custom-content">Custom warning</p>}
      />,
    );

    fireEvent.click(screen.getByText("Löschen"));

    await waitFor(() => {
      expect(screen.getByTestId("custom-content")).toBeInTheDocument();
      expect(screen.getByText("Custom warning")).toBeInTheDocument();
    });
  });

  it("shows confirmation modal title with entity type", async () => {
    render(<DetailModalActions {...defaultProps} entityType="Raum" />);

    fireEvent.click(screen.getByText("Löschen"));

    await waitFor(() => {
      expect(screen.getByText("Raum löschen?")).toBeInTheDocument();
    });
  });
});
