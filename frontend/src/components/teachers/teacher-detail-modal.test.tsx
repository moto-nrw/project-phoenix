import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { createElement } from "react";
import { describe, it, expect, vi } from "vitest";
import { TeacherDetailModal } from "./teacher-detail-modal";

const { mockLoggerError } = vi.hoisted(() => ({
  mockLoggerError: vi.fn(),
}));

vi.mock("next/image", () => ({
  default: (props: React.ImgHTMLAttributes<HTMLImageElement>) =>
    createElement("img", {
      ...props,
      alt: props.alt ?? "",
    }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: mockLoggerError,
  }),
}));

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
      <span data-testid="value" data-fullwidth={fullWidth} data-mono={mono}>
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
  name: "John Doe",
  person_id: 10,
  staff_id: "100",
  first_name: "John",
  last_name: "Doe",
  email: "john@example.com",
  role: "Pädagogische Fachkraft",
  tag_id: "RFID123",
  qualifications: "Bachelor in Education",
  staff_notes: "Some important notes",
  created_at: "2024-01-15T10:00:00",
  updated_at: "2024-06-20T14:30:00",
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

  it("renders the avatar image when the teacher has an avatar", () => {
    render(
      <TeacherDetailModal
        {...defaultProps}
        teacher={{ ...mockTeacher, avatar: "/uploads/avatars/global/john.jpg" }}
      />,
    );

    expect(screen.getByAltText("John Doe")).toHaveAttribute(
      "src",
      "/api/staff/100/avatar",
    );
  });

  it("shows loading state when loading is true", () => {
    render(<TeacherDetailModal {...defaultProps} loading={true} />);

    expect(screen.getByTestId("loading-state")).toBeInTheDocument();
    expect(screen.getByTestId("accent-color")).toHaveTextContent("orange");
  });

  it("displays personal information section", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("section-persönliche-daten")).toBeInTheDocument();
  });

  it("displays first and last name fields", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-vorname")).toBeInTheDocument();
    expect(screen.getByTestId("field-nachname")).toBeInTheDocument();
  });

  it("displays email when provided", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("section-e-mail")).toBeInTheDocument();
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

  it("hides professional section when only a role is present", () => {
    const teacherRoleOnly = {
      ...mockTeacher,
      role: "Gruppenleitung",
      qualifications: "",
    };
    render(<TeacherDetailModal {...defaultProps} teacher={teacherRoleOnly} />);

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

  it("shows the caregiver management action when provided", () => {
    const onManageCaregiver = vi.fn();

    render(
      <TeacherDetailModal
        {...defaultProps}
        onManageCaregiver={onManageCaregiver}
      />,
    );

    fireEvent.click(screen.getByText("Betreuung verwalten"));
    expect(onManageCaregiver).toHaveBeenCalledTimes(1);
  });

  it("supports inline note editing when onUpdateNotes is provided", async () => {
    const onUpdateNotes = vi.fn().mockResolvedValue(undefined);

    render(
      <TeacherDetailModal
        {...defaultProps}
        onUpdateNotes={onUpdateNotes}
        teacher={{ ...mockTeacher, staff_notes: "Existing notes" }}
      />,
    );

    fireEvent.click(screen.getByText("Existing notes"));
    fireEvent.change(screen.getByPlaceholderText("Notizen hinzufügen..."), {
      target: { value: "Updated notes" },
    });
    fireEvent.click(screen.getByText("Speichern"));

    expect(onUpdateNotes).toHaveBeenCalledWith("Updated notes");
  });

  it("logs note save failures without closing the editor", async () => {
    const onUpdateNotes = vi.fn().mockRejectedValue(new Error("save failed"));

    render(
      <TeacherDetailModal
        {...defaultProps}
        onUpdateNotes={onUpdateNotes}
        teacher={{ ...mockTeacher, staff_notes: "Existing notes" }}
      />,
    );

    fireEvent.click(screen.getByText("Existing notes"));
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        "notes_save_failed",
        expect.objectContaining({ error: "save failed" }),
      );
    });
    expect(
      screen.getByPlaceholderText("Notizen hinzufügen..."),
    ).toBeInTheDocument();
  });

  it("resyncs inline notes when parent props change after cancel", () => {
    const onUpdateNotes = vi.fn().mockResolvedValue(undefined);
    const { rerender } = render(
      <TeacherDetailModal
        {...defaultProps}
        onUpdateNotes={onUpdateNotes}
        teacher={{ ...mockTeacher, staff_notes: "Initial notes" }}
      />,
    );

    fireEvent.click(screen.getByText("Initial notes"));
    fireEvent.change(screen.getByPlaceholderText("Notizen hinzufügen..."), {
      target: { value: "Unsaved local edit" },
    });
    fireEvent.click(screen.getByText("Abbrechen"));

    rerender(
      <TeacherDetailModal
        {...defaultProps}
        onUpdateNotes={onUpdateNotes}
        teacher={{ ...mockTeacher, staff_notes: "Fresh notes from parent" }}
      />,
    );

    fireEvent.click(screen.getByText("Fresh notes from parent"));
    expect(
      screen.getByDisplayValue("Fresh notes from parent"),
    ).toBeInTheDocument();
  });

  it("starts an email draft when clicking Schreiben", () => {
    const originalHref = window.location.href;

    render(<TeacherDetailModal {...defaultProps} />);

    fireEvent.click(screen.getByTitle("E-Mail schreiben"));

    expect(window.location.href).toContain(
      "mailto:john@example.com?subject=Betreff%3A%20John%20Doe",
    );
    window.location.href = originalHref;
  });

  it("copies the email address through the clipboard API", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(<TeacherDetailModal {...defaultProps} />);

    fireEvent.click(screen.getByTitle("E-Mail kopieren"));

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith("john@example.com");
    });
  });

  it("falls back to document.execCommand when clipboard access fails", async () => {
    const writeText = vi.fn().mockRejectedValue(new Error("no clipboard"));
    const execCommand = vi.fn(() => true);
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: execCommand,
    });
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });

    render(<TeacherDetailModal {...defaultProps} />);

    fireEvent.click(screen.getByTitle("E-Mail kopieren"));

    await waitFor(() => {
      expect(execCommand).toHaveBeenCalledWith("copy");
    });
  });

  it("passes correct entity info to DetailModalActions", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("entity-name")).toHaveTextContent("John Doe");
    expect(screen.getByTestId("entity-type")).toHaveTextContent("Personal");
  });

  it("displays teacher ID when provided", () => {
    render(<TeacherDetailModal {...defaultProps} />);

    expect(screen.getByTestId("field-personal-id")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
  });
});
