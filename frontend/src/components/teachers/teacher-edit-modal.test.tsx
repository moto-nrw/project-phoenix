import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { TeacherEditModal } from "./teacher-edit-modal";
import type { Teacher } from "@/lib/teacher-api";

// Mock the Modal component
vi.mock("@/components/ui/modal", () => ({
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
      <div data-testid="modal" role="dialog" aria-label={title}>
        <h2>{title}</h2>
        <button onClick={onClose} data-testid="close-button">
          Close
        </button>
        {children}
      </div>
    ) : null,
}));

// Mock the TeacherForm component
vi.mock("./teacher-form", () => ({
  TeacherForm: ({
    initialData,
    onSubmitAction,
    onCancelAction,
    isLoading,
    formTitle,
    wrapInCard,
    submitLabel,
  }: {
    initialData: Teacher;
    onSubmitAction: (data: Partial<Teacher>) => Promise<void>;
    onCancelAction: () => void;
    isLoading: boolean;
    formTitle: string;
    wrapInCard: boolean;
    submitLabel: string;
  }) => (
    <div data-testid="teacher-form">
      <span data-testid="initial-data">{JSON.stringify(initialData)}</span>
      <span data-testid="form-title">{formTitle}</span>
      <span data-testid="wrap-in-card">{String(wrapInCard)}</span>
      <span data-testid="submit-label">{submitLabel}</span>
      <span data-testid="is-loading">{String(isLoading)}</span>
      <button
        onClick={() => void onSubmitAction({ name: "Updated Teacher" })}
        data-testid="submit-form"
      >
        Submit
      </button>
      <button onClick={onCancelAction} data-testid="cancel-form">
        Cancel
      </button>
    </div>
  ),
}));

describe("TeacherEditModal", () => {
  const mockTeacher: Teacher = {
    id: "1",
    name: "John Doe",
    first_name: "John",
    last_name: "Doe",
    email: "john.doe@example.com",
    account_id: 123,
    staff_id: "456",
    person_id: 789,
  };

  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("when teacher is null", () => {
    it("should render nothing", () => {
      const { container } = render(
        <TeacherEditModal
          isOpen={true}
          onClose={mockOnClose}
          teacher={null}
          onSave={mockOnSave}
        />,
      );

      expect(container.firstChild).toBeNull();
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });
  });

  describe("when teacher is provided", () => {
    it("should render modal with teacher data when open", () => {
      render(
        <TeacherEditModal
          isOpen={true}
          onClose={mockOnClose}
          teacher={mockTeacher}
          onSave={mockOnSave}
        />,
      );

      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByText("Betreuer bearbeiten")).toBeInTheDocument();
    });

    it("should not render modal when closed", () => {
      render(
        <TeacherEditModal
          isOpen={false}
          onClose={mockOnClose}
          teacher={mockTeacher}
          onSave={mockOnSave}
        />,
      );

      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });

    it("should render TeacherForm with correct props when not loading", () => {
      render(
        <TeacherEditModal
          isOpen={true}
          onClose={mockOnClose}
          teacher={mockTeacher}
          onSave={mockOnSave}
          loading={false}
        />,
      );

      expect(screen.getByTestId("teacher-form")).toBeInTheDocument();
      expect(screen.getByTestId("initial-data")).toHaveTextContent(
        JSON.stringify(mockTeacher),
      );
      expect(screen.getByTestId("form-title")).toHaveTextContent("");
      expect(screen.getByTestId("wrap-in-card")).toHaveTextContent("false");
      expect(screen.getByTestId("submit-label")).toHaveTextContent("Speichern");
      expect(screen.getByTestId("is-loading")).toHaveTextContent("false");
    });

    it("should render loading state when loading is true", () => {
      render(
        <TeacherEditModal
          isOpen={true}
          onClose={mockOnClose}
          teacher={mockTeacher}
          onSave={mockOnSave}
          loading={true}
        />,
      );

      expect(screen.queryByTestId("teacher-form")).not.toBeInTheDocument();
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });

    it("should default loading to false", () => {
      render(
        <TeacherEditModal
          isOpen={true}
          onClose={mockOnClose}
          teacher={mockTeacher}
          onSave={mockOnSave}
        />,
      );

      // TeacherForm should be rendered (not loading state)
      expect(screen.getByTestId("teacher-form")).toBeInTheDocument();
      expect(
        screen.queryByText("Daten werden geladen..."),
      ).not.toBeInTheDocument();
    });

    it("should call onSave when form is submitted", () => {
      render(
        <TeacherEditModal
          isOpen={true}
          onClose={mockOnClose}
          teacher={mockTeacher}
          onSave={mockOnSave}
        />,
      );

      const submitButton = screen.getByTestId("submit-form");
      fireEvent.click(submitButton);

      expect(mockOnSave).toHaveBeenCalledWith({ name: "Updated Teacher" });
    });

    it("should call onClose when cancel button is clicked", () => {
      render(
        <TeacherEditModal
          isOpen={true}
          onClose={mockOnClose}
          teacher={mockTeacher}
          onSave={mockOnSave}
        />,
      );

      const cancelButton = screen.getByTestId("cancel-form");
      fireEvent.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalled();
    });

    it("should call onClose when modal close button is clicked", () => {
      render(
        <TeacherEditModal
          isOpen={true}
          onClose={mockOnClose}
          teacher={mockTeacher}
          onSave={mockOnSave}
        />,
      );

      const closeButton = screen.getByTestId("close-button");
      fireEvent.click(closeButton);

      expect(mockOnClose).toHaveBeenCalled();
    });
  });
});
