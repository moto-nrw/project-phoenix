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
  // Unified per-day departure selector (#1610): renders each weekday's current
  // mode and offers a button to set Monday to "bus" for interaction tests.
  DepartureSection: ({
    days,
    onChange,
  }: {
    days?: Record<string, string>;
    onChange: (v: Record<string, string>) => void;
  }) => (
    <div data-testid="departure-section">
      {["mon", "tue", "wed", "thu", "fri"].map((d) => (
        <span key={d} data-testid={`departure-${d}`}>
          {days?.[d] ?? "alone"}
        </span>
      ))}
      <button
        type="button"
        data-testid="departure-set-mon-bus"
        onClick={() => onChange({ ...days, mon: "bus" })}
      >
        set-mon-bus
      </button>
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
      expect(screen.getByText("Kind bearbeiten")).toBeInTheDocument();
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

  it("renders the unified departure section", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("departure-section")).toBeInTheDocument();
    });
  });

  it("updates the departure plan on interaction", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={mockStudent}
        onSave={mockOnSave}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("departure-section")).toBeInTheDocument();
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("departure-set-mon-bus"));
    });
    expect(screen.getByTestId("departure-mon")).toHaveTextContent("bus");
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
      expect(screen.getByText("Zur Kinddetailseite")).toBeInTheDocument();
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
      const link = screen.getByText("Zur Kinddetailseite").closest("a");
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

  it("initializes the departure plan from the student's bus days", async () => {
    render(
      <StudentEditModal
        isOpen={true}
        onClose={mockOnClose}
        student={{
          ...mockStudent,
          bus: true,
          bus_days: { mon: true, wed: true },
        }}
        onSave={mockOnSave}
      />,
    );

    // Legacy bus_days fold into the unified plan as "bus" on those weekdays.
    await waitFor(() => {
      expect(screen.getByTestId("departure-mon")).toHaveTextContent("bus");
      expect(screen.getByTestId("departure-wed")).toHaveTextContent("bus");
      expect(screen.getByTestId("departure-tue")).toHaveTextContent("alone");
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
