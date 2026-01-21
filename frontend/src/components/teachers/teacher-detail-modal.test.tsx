import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { TeacherDetailModal } from "./teacher-detail-modal";

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
        <button data-testid="modal-close" onClick={onClose}>
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
    onDeleteClick,
    entityName,
    entityType,
  }: {
    onEdit: () => void;
    onDelete: () => void;
    onDeleteClick?: () => void;
    entityName: string;
    entityType: string;
  }) => (
    <div data-testid="detail-actions">
      <span data-testid="entity-name">{entityName}</span>
      <span data-testid="entity-type">{entityType}</span>
      <button data-testid="edit-btn" onClick={onEdit}>
        Edit
      </button>
      <button data-testid="delete-btn" onClick={onDeleteClick ?? onDelete}>
        Delete
      </button>
    </div>
  ),
}));

vi.mock("~/components/ui/modal-loading-state", () => ({
  ModalLoadingState: ({ accentColor }: { accentColor: string }) => (
    <div data-testid="loading-state">
      <span data-testid="accent-color">{accentColor}</span>
      Loading...
    </div>
  ),
}));

vi.mock("~/components/ui/detail-modal-components", () => ({
  DataField: ({
    label,
    children,
    fullWidth,
    mono,
  }: {
    label: string;
    children: React.ReactNode;
    fullWidth?: boolean;
    mono?: boolean;
  }) => (
    <div data-testid={`field-${label.toLowerCase().replace(/\s/g, "-")}`}>
      <span data-testid="label">{label}</span>
      <span
        data-testid="value"
        data-fullwidth={fullWidth}
        data-mono={mono}
      >
        {children}
      </span>
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
    icon: React.ReactNode;
    accentColor: string;
    children: React.ReactNode;
  }) => (
    <div data-testid={`section-${title.toLowerCase().replace(/\s/g, "-")}`}>
      <span data-testid="section-title">{title}</span>
      {children}
    </div>
  ),
  DetailIcons: {
    person: <span>PersonIcon</span>,
    briefcase: <span>BriefcaseIcon</span>,
    notes: <span>NotesIcon</span>,
    document: <span>DocumentIcon</span>,
  },
}));

const mockTeacher = {
  id: "1",
  person_id: "10",
  staff_id: "100",
  first_name: "John",
  last_name: "Doe",
  email: "john@example.com",
  role: "Pädagogische Fachkraft",
  tag_id: "RFID123",
  qualifications: "Bachelor in Education",
  staff_notes: "Some important notes",
  created_at: new Date("2024-01-15T10:00:00"),
  updated_at: new Date("2024-06-20T14:30:00"),
  is_teacher: true,
};

describe("TeacherDetailModal", () => {
  const defaultProps = {
    isOpen: true,
    onClose: vi.fn(),
    teacher: mockTeacher,
    onEdit: vi.fn(),
    onDelete: vi.fn(),
  };

  it("renders nothing when teacher is null", () => {
    const { container } = render(
      <TeacherDetailModal {...defaultProps} teacher={null} />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("renders modal when open with teacher", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("displays teacher name in header", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getAllByText("John Doe").length).toBeGreaterThan(0);
  });

  it("displays teacher initials in avatar", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByText("JD")).toBeInTheDocument();
  });

  it("shows loading state when loading is true", () => {
    render(<TeacherDetailModal {...defaultProps} loading={true} />);

    expect(screen.getByTestId("loading-state")).toBeInTheDocument();
    expect(screen.getByTestId("accent-color")).toHaveTextContent("orange");
  });

  it("displays personal information section", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(
      screen.getByTestId("section-persönliche-daten"),
    ).toBeInTheDocument();
  });

  it("displays first and last name fields", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-vorname")).toBeInTheDocument();
    expect(screen.getByTestId("field-nachname")).toBeInTheDocument();
  });

  it("displays email when provided", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-e-mail")).toBeInTheDocument();
    expect(screen.getByText("john@example.com")).toBeInTheDocument();
  });

  it("displays RFID tag when provided", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-rfid-karte")).toBeInTheDocument();
    expect(screen.getByText("RFID123")).toBeInTheDocument();
  });

  it("displays professional information section when role exists", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(
      screen.getByTestId("section-berufliche-informationen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Pädagogische Fachkraft")).toBeInTheDocument();
  });

  it("hides professional section when no role or qualifications", () => {
    const teacherNoRole = { ...mockTeacher, role: "", qualifications: "" };
    render(<TeacherDetailModal {...defaultProps} teacher={teacherNoRole} />);

    expect(
      screen.queryByTestId("section-berufliche-informationen"),
    ).not.toBeInTheDocument();
  });

  it("displays notes section when staff_notes exists", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("section-notizen")).toBeInTheDocument();
    expect(screen.getByText("Some important notes")).toBeInTheDocument();
  });

  it("hides notes section when no staff_notes", () => {
    const teacherNoNotes = { ...mockTeacher, staff_notes: "" };
    render(<TeacherDetailModal {...defaultProps} teacher={teacherNoNotes} />);

    expect(screen.queryByTestId("section-notizen")).not.toBeInTheDocument();
  });

  it("displays timestamps section", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("section-zeitstempel")).toBeInTheDocument();
  });

  it("calls onEdit when edit button clicked", () => {
    const onEdit = vi.fn();
    render(<TeacherDetailModal {...defaultProps} onEdit={onEdit} />);

    fireEvent.click(screen.getByTestId("edit-btn"));

    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("calls onDeleteClick when provided", () => {
    const onDeleteClick = vi.fn();
    render(
      <TeacherDetailModal {...defaultProps} onDeleteClick={onDeleteClick} />,
    );

    fireEvent.click(screen.getByTestId("delete-btn"));

    expect(onDeleteClick).toHaveBeenCalledTimes(1);
  });

  it("passes correct entity info to DetailModalActions", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("entity-name")).toHaveTextContent("John Doe");
    expect(screen.getByTestId("entity-type")).toHaveTextContent("Betreuer");
  });

  it("displays teacher ID when provided", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-betreuer-id")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
  });
});
