import { render, screen } from "@testing-library/react";
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
      "Neuen Betreuer erstellen",
    );
  });

  it("shows loading state when loading is true", () => {
    render(<TeacherCreateModal {...defaultProps} loading={true} />);

    expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    expect(screen.queryByTestId("teacher-form")).not.toBeInTheDocument();
  });

  it("shows TeacherForm when not loading", () => {
    render(<TeacherCreateModal {...defaultProps} loading={false} />);

    expect(screen.getByTestId("teacher-form")).toBeInTheDocument();
    expect(
      screen.queryByText("Daten werden geladen..."),
    ).not.toBeInTheDocument();
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

  it("displays loading spinner during loading state", () => {
    const { container } = render(
      <TeacherCreateModal {...defaultProps} loading={true} />,
    );

    const spinner = container.querySelector(".animate-spin");
    expect(spinner).toBeInTheDocument();
  });
});
