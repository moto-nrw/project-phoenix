import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { TeacherCreateModal } from "./teacher-create-modal";

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
        <span data-testid="modal-title">{title}</span>
        <button data-testid="modal-close" onClick={onClose}>
          Close
        </button>
        {children}
      </div>
    ) : null,
}));

vi.mock("./teacher-form", () => ({
  TeacherForm: ({
    initialData,
    onSubmitAction,
    onCancelAction,
    submitLabel,
    wrapInCard,
  }: {
    initialData: Record<string, unknown>;
    onSubmitAction: () => void;
    onCancelAction: () => void;
    submitLabel: string;
    wrapInCard: boolean;
  }) => (
    <div data-testid="teacher-form">
      <span data-testid="submit-label">{submitLabel}</span>
      <span data-testid="wrap-in-card">{String(wrapInCard)}</span>
      <span data-testid="initial-data">{JSON.stringify(initialData)}</span>
      <button data-testid="submit-btn" onClick={onSubmitAction}>
        Submit
      </button>
      <button data-testid="cancel-btn" onClick={onCancelAction}>
        Cancel
      </button>
    </div>
  ),
}));

describe("TeacherCreateModal", () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    onCreate: vi.fn().mockResolvedValue(undefined),
  };

  it("renders nothing when not open", () => {
    const { container } = render(
      <TeacherCreateModal {...defaultProps} isOpen={false} />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders modal when open", () => {
    render(<TeacherCreateModal {...defaultProps} />);

    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("displays correct modal title", () => {
    render(<TeacherCreateModal {...defaultProps} />);

    expect(screen.getByTestId("modal-title")).toHaveTextContent(
      "Neues Personal anlegen",
    );
  });

  it("shows TeacherForm when open", () => {
    render(<TeacherCreateModal {...defaultProps} />);

    expect(screen.getByTestId("teacher-form")).toBeInTheDocument();
  });

  it("passes correct props to TeacherForm", () => {
    render(<TeacherCreateModal {...defaultProps} />);

    expect(screen.getByTestId("submit-label")).toHaveTextContent("Erstellen");
    expect(screen.getByTestId("wrap-in-card")).toHaveTextContent("false");
    expect(screen.getByTestId("initial-data")).toHaveTextContent("{}");
  });

  it("passes onCreate handler to TeacherForm", async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined);
    render(<TeacherCreateModal {...defaultProps} onCreate={onCreate} />);

    screen.getByTestId("submit-btn").click();

    expect(onCreate).toHaveBeenCalled();
  });

  it("passes onClose handler to TeacherForm cancel", () => {
    const onClose = vi.fn();
    render(<TeacherCreateModal {...defaultProps} onClose={onClose} />);

    screen.getByTestId("cancel-btn").click();

    expect(onClose).toHaveBeenCalled();
  });

  describe("link confirmation flow", () => {
    it("shows link confirmation when onCreate returns account_exists", async () => {
      const onCreate = vi.fn().mockResolvedValue({
        status: "account_exists",
        email: "existing@example.com",
      });
      render(<TeacherCreateModal {...defaultProps} onCreate={onCreate} />);

      // Trigger form submission
      fireEvent.click(screen.getByTestId("submit-btn"));

      // Should now show confirmation dialog
      await waitFor(() => {
        expect(screen.getByText("Konto verknüpfen")).toBeInTheDocument();
      });
      expect(screen.getByText(/existing@example.com/)).toBeInTheDocument();
      expect(
        screen.getByText(/bestehende Passwort bleibt unverändert/),
      ).toBeInTheDocument();
    });

    it("calls onCreate with linkExisting when confirm button clicked", async () => {
      const onCreate = vi
        .fn()
        .mockResolvedValueOnce({
          status: "account_exists",
          email: "existing@example.com",
        })
        .mockResolvedValueOnce(undefined);

      render(<TeacherCreateModal {...defaultProps} onCreate={onCreate} />);

      // Trigger first submission to show confirmation
      fireEvent.click(screen.getByTestId("submit-btn"));

      await waitFor(() => {
        expect(screen.getByText("Konto verknüpfen")).toBeInTheDocument();
      });

      // Click the confirm link button
      fireEvent.click(screen.getByText("Verknüpfen"));

      await waitFor(() => {
        expect(onCreate).toHaveBeenCalledTimes(2);
      });

      // Second call should have linkExisting: true
      const secondCall = onCreate.mock.calls[1]?.[0] as Record<string, unknown>;
      expect(secondCall.linkExisting).toBe(true);
    });

    it("closes modal and resets state when cancel clicked during confirmation", async () => {
      const onClose = vi.fn();
      const onCreate = vi.fn().mockResolvedValue({
        status: "account_exists",
        email: "existing@example.com",
      });

      render(
        <TeacherCreateModal
          {...defaultProps}
          onClose={onClose}
          onCreate={onCreate}
        />,
      );

      // Trigger confirmation flow
      fireEvent.click(screen.getByTestId("submit-btn"));

      await waitFor(() => {
        expect(screen.getByText("Konto verknüpfen")).toBeInTheDocument();
      });

      // Click cancel / Abbrechen
      fireEvent.click(screen.getByText("Abbrechen"));

      expect(onClose).toHaveBeenCalled();
    });
  });
});
