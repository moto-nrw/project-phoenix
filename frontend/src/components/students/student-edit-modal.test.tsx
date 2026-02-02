/**
 * Tests for StudentEditModal
 * Tests the rendering and functionality of the student edit modal
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { StudentEditModal } from "./student-edit-modal";
import type { Student } from "@/lib/api";

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

// Mock student form field components
vi.mock("./student-form-fields", () => ({
  PersonalInfoSection: ({
    formData,
    onChange,
    errors,
  }: {
    formData: Record<string, unknown>;
    onChange: (field: string, value: unknown) => void;
    errors: Record<string, string>;
    groups?: Array<{ value: string; label: string }>;
  }) => (
    <div data-testid="personal-info-section">
      {errors.first_name && (
        <div data-testid="error-first-name">{errors.first_name}</div>
      )}
      <input
        data-testid="first-name-input"
        value={(formData.first_name as string) ?? ""}
        onChange={(e) => onChange("first_name", e.target.value)}
      />
    </div>
  ),
  BusStatusSection: ({
    value,
    onChange,
  }: {
    value: unknown;
    onChange: (v: boolean) => void;
  }) => (
    <div data-testid="bus-status-section">
      <input
        type="checkbox"
        data-testid="bus-checkbox"
        checked={(value as boolean) ?? false}
        onChange={(e) => onChange(e.target.checked)}
      />
    </div>
  ),
  PickupStatusSection: ({
    value,
    onChange,
  }: {
    value: unknown;
    onChange: (v: string) => void;
  }) => (
    <div data-testid="pickup-status-section">
      <input
        data-testid="pickup-input"
        value={(value as string) ?? ""}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  ),
}));

// Mock student common form sections
vi.mock("./student-common-form-sections", () => ({
  StudentCommonFormSections: ({
    formData,
    errors: _errors,
    onChange,
  }: {
    formData: Record<string, unknown>;
    errors: Record<string, string>;
    onChange: (field: string, value: unknown) => void;
  }) => (
    <div data-testid="common-form-sections">
      <input
        data-testid="health-info-input"
        value={(formData.health_info as string) ?? ""}
        onChange={(e) => onChange("health_info", e.target.value)}
      />
    </div>
  ),
}));

// Mock validation utilities
vi.mock("~/lib/student-form-validation", () => ({
  validateStudentForm: vi.fn(() => ({})),
  handleStudentFormSubmit: vi.fn(
    (
      e: Event,
      _formData: unknown,
      _validateForm: unknown,
      onSave: (data: Record<string, unknown>) => Promise<void>,
      setSaveLoading: (v: boolean) => void,
      _setErrors: unknown,
    ) => {
      e.preventDefault();
      setSaveLoading(true);
      void onSave({})
        .then(() => setSaveLoading(false))
        .catch(() => setSaveLoading(false));
    },
  ),
}));

describe("StudentEditModal", () => {
  const mockStudent: Student = {
    id: "1",
    name: "John Doe",
    first_name: "John",
    second_name: "Doe",
    school_class: "5a",
    current_location: "Gruppenraum",
    group_id: "1",
    birthday: "2010-01-01",
    health_info: "None",
    supervisor_notes: "Good student",
    extra_info: "Likes sports",
    privacy_consent_accepted: true,
    data_retention_days: 30,
    bus: false,
    pickup_status: "self",
  };

  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the modal when open with student data", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <StudentEditModal
        isOpen={false}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("returns null when student is null", () => {
    const { container } = render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={null}
        onSave={mockOnSave}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("displays the correct title", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Schüler bearbeiten")).toBeInTheDocument();
    });
  });

  it("shows loading state when loading prop is true", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
        loading={true}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
    });
  });

  it("renders personal info section", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("personal-info-section")).toBeInTheDocument();
    });
  });

  it("renders bus status section", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("bus-status-section")).toBeInTheDocument();
    });
  });

  it("renders pickup status section", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("pickup-status-section")).toBeInTheDocument();
    });
  });

  it("renders common form sections", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("common-form-sections")).toBeInTheDocument();
    });
  });

  it("displays guardian management note with link", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Erziehungsberechtigte verwalten/i),
      ).toBeInTheDocument();
      expect(screen.getByText("Zur Schülerdetailseite")).toBeInTheDocument();
    });
  });

  it("renders link to student detail page", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      const link = screen.getByText("Zur Schülerdetailseite").closest("a");
      expect(link).toHaveAttribute("href", "/students/1");
      expect(link).toHaveAttribute("target", "_blank");
    });
  });

  it("renders action buttons", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Abbrechen")).toBeInTheDocument();
      expect(screen.getByText("Speichern")).toBeInTheDocument();
    });
  });

  it("calls onClose when cancel button is clicked", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
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

  it("calls onSave when form is submitted", async () => {
    mockOnSave.mockResolvedValue(undefined);

    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    const form = screen.getByTestId("modal").querySelector("form");
    expect(form).toBeTruthy();

    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(mockOnSave).toHaveBeenCalled();
    });
  });

  it("initializes form with student data", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("first-name-input")).toHaveValue("John");
    });
  });

  it("updates form data when input changes", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("first-name-input")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.change(screen.getByTestId("first-name-input"), {
        target: { value: "Jane" },
      });
    });

    expect(screen.getByTestId("first-name-input")).toHaveValue("Jane");
  });

  it("disables buttons when saving", async () => {
    mockOnSave.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    const form = screen.getByTestId("modal").querySelector("form");
    expect(form).toBeTruthy();

    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(screen.getByText("Wird gespeichert...")).toBeInTheDocument();
    });
  });

  it("passes groups prop to PersonalInfoSection", async () => {
    const groups = [
      { value: "1", label: "Group 1" },
      { value: "2", label: "Group 2" },
    ];

    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
        groups={groups}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("personal-info-section")).toBeInTheDocument();
    });
  });

  it("reinitializes form data when student prop changes", async () => {
    const { rerender } = render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("first-name-input")).toHaveValue("John");
    });

    const newStudent = { ...mockStudent, first_name: "Jane" };

    rerender(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={newStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("first-name-input")).toHaveValue("Jane");
    });
  });
});
