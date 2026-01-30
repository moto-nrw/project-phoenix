/**
 * Tests for StudentDetailModal Component
 * Tests the rendering and display of student information
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { StudentDetailModal } from "./student-detail-modal";
import type { Student } from "@/lib/api";

// Mock UI components
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    children,
    onClose,
  }: {
    isOpen: boolean;
    children: React.ReactNode;
    onClose: () => void;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <button onClick={onClose} data-testid="modal-close">
          Close
        </button>
        {children}
      </div>
    ) : null,
}));

vi.mock("~/components/ui/detail-modal-actions", () => ({
  DetailModalActions: ({
    onEdit,
    onDelete,
    entityName,
  }: {
    onEdit: () => void;
    onDelete: () => void;
    entityName: string;
  }) => (
    <div data-testid="detail-modal-actions">
      <button onClick={onEdit} data-testid="edit-button">
        Edit {entityName}
      </button>
      <button onClick={onDelete} data-testid="delete-button">
        Delete {entityName}
      </button>
    </div>
  ),
}));

vi.mock("~/components/ui/modal-loading-state", () => ({
  ModalLoadingState: () => <div data-testid="loading-state">Loading...</div>,
}));

vi.mock("~/components/ui/detail-modal-components", () => ({
  DataField: ({
    label,
    children,
  }: {
    label: string;
    children: React.ReactNode;
  }) => (
    <div data-testid="data-field">
      <dt>{label}</dt>
      <dd>{children}</dd>
    </div>
  ),
  DataGrid: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="data-grid">{children}</div>
  ),
  InfoSection: ({
    title,
    children,
  }: {
    title: string;
    children: React.ReactNode;
  }) => (
    <div data-testid="info-section">
      <h3>{title}</h3>
      {children}
    </div>
  ),
  InfoText: ({ children }: { children: React.ReactNode }) => (
    <p data-testid="info-text">{children}</p>
  ),
  DetailIcons: {
    person: <svg data-testid="icon-person" />,
    group: <svg data-testid="icon-group" />,
    heart: <svg data-testid="icon-heart" />,
    notes: <svg data-testid="icon-notes" />,
    document: <svg data-testid="icon-document" />,
    home: <svg data-testid="icon-home" />,
    lock: <svg data-testid="icon-lock" />,
    bus: <svg data-testid="icon-bus" />,
    check: <svg data-testid="icon-check" />,
    x: <svg data-testid="icon-x" />,
  },
}));

const mockStudent: Student = {
  id: "1",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "5a",
  group_name: "Gruppe A",
  privacy_consent_accepted: true,
  data_retention_days: 30,
  bus: false,
  health_info: "Keine Allergien",
  supervisor_notes: "Sehr aktiv",
  extra_info: null,
  name_lg: "Maria Mustermann",
  contact_lg: "0123456789",
  guardian_email: "maria@example.com",
  guardian_phone: "0123456789",
  pickup_status: "Abholung durch Eltern",
};

describe("StudentDetailModal", () => {
  const mockOnClose = vi.fn();
  const mockOnEdit = vi.fn();
  const mockOnDelete = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when student is null", () => {
    const { container } = render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={null}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders student name and class", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.getByText("Klasse 5a")).toBeInTheDocument();
    });
  });

  it("renders personal information section", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getAllByTestId("info-section").length,
      ).toBeGreaterThanOrEqual(3);
    });
  });

  it("renders guardian information", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Maria Mustermann")).toBeInTheDocument();
      expect(screen.getByText("maria@example.com")).toBeInTheDocument();
    });
  });

  it("renders health information when available", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Keine Allergien")).toBeInTheDocument();
    });
  });

  it("renders supervisor notes when available", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Sehr aktiv")).toBeInTheDocument();
    });
  });

  it("renders privacy consent status", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Ja")).toBeInTheDocument();
      expect(screen.getByText("30 Tage")).toBeInTheDocument();
    });
  });

  it("shows bus status when student takes bus", async () => {
    const studentWithBus = { ...mockStudent, bus: true };
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={studentWithBus}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Fährt mit dem Bus")).toBeInTheDocument();
    });
  });

  it("renders modal actions", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("detail-modal-actions")).toBeInTheDocument();
    });
  });

  it("shows loading state", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("loading-state")).toBeInTheDocument();
    });
  });

  it("shows error state", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
        error="Failed to load student"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Fehler beim Laden")).toBeInTheDocument();
      expect(screen.getByText("Failed to load student")).toBeInTheDocument();
    });
  });

  it("closes modal when close button is clicked", async () => {
    render(
      <StudentDetailModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    await waitFor(() => {
      const closeButton = screen.getByTestId("modal-close");
      fireEvent.click(closeButton);
    });

    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });
});
