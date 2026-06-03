/**
 * Tests for StudentCreateModal
 * Tests the rendering and functionality of the student creation modal
 */
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { StudentCreateModal } from "./student-create-modal";
import { handleStudentFormSubmit } from "~/lib/student-form-validation";

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

// Mock the reused GuardianFormModal: when open, expose a button that calls
// onSubmit with one canned guardian entry (the same shape the real modal emits).
// This lets us exercise StudentCreateModal's collection/summary logic without
// driving the full guardian form.
vi.mock("~/components/guardians/guardian-form-modal", () => ({
  default: ({
    isOpen,
    onSubmit,
  }: {
    isOpen: boolean;
    onSubmit: (entries: unknown[]) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="guardian-form-modal">
        <button
          type="button"
          onClick={() =>
            void onSubmit([
              {
                id: "g1",
                guardianData: {
                  firstName: "Erika",
                  lastName: "Muster",
                  email: "erika@example.com",
                  languagePreference: "de",
                },
                relationshipData: {
                  relationshipType: "parent",
                  isPrimary: true,
                  isEmergencyContact: false,
                  canPickup: true,
                  emergencyPriority: 1,
                },
                phoneNumbers: [
                  {
                    phoneNumber: "0151 2345678",
                    phoneType: "mobile",
                    isPrimary: true,
                  },
                ],
              },
            ])
          }
        >
          MockSubmitGuardian
        </button>
      </div>
    ) : null,
}));

// Mock the inline existing-guardian picker panel: it's mounted only while
// active, so it always renders here and exposes a button that calls onSelect
// with one canned EXISTING guardian + relationship (the shape the real panel
// emits). Lets us exercise StudentCreateModal's link-existing collection without
// driving the real search UI.
vi.mock("~/components/guardians/guardian-picker-panel", () => ({
  default: ({
    onSelect,
  }: {
    onSelect: (guardian: unknown, relationship: unknown) => void;
    onCancel: () => void;
  }) => (
    <div data-testid="guardian-picker-panel">
      <button
        type="button"
        onClick={() =>
          onSelect(
            {
              id: "777",
              firstName: "Hans",
              lastName: "Schmidt",
              email: "hans@example.com",
              phoneNumbers: [],
              preferredContactMethod: "email",
              languagePreference: "de",
              hasAccount: false,
            },
            {
              relationshipType: "parent",
              isPrimary: false,
              isEmergencyContact: false,
              canPickup: true,
              emergencyPriority: 1,
            },
          )
        }
      >
        MockPickExisting
      </button>
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
      onCreate: (data: Record<string, unknown>) => Promise<void>,
      setSaveLoading: (loading: boolean) => void,
      _setErrors: unknown,
    ) => {
      e.preventDefault();
      setSaveLoading(true);
      void onCreate({})
        .then(() => setSaveLoading(false))
        .catch(() => setSaveLoading(false));
    },
  ),
}));

describe("StudentCreateModal", () => {
  const mockOnClose = vi.fn();
  const mockOnCreate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the modal when open", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
  });

  it("does not render when closed", () => {
    render(
      <StudentCreateModal
        isOpen={false}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("displays the correct title", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Neuer Schüler")).toBeInTheDocument();
    });
  });

  it("renders personal info section", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("personal-info-section")).toBeInTheDocument();
    });
  });

  it("renders bus status section", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("bus-status-section")).toBeInTheDocument();
    });
  });

  it("renders pickup status section", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("pickup-status-section")).toBeInTheDocument();
    });
  });

  it("renders common form sections", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("common-form-sections")).toBeInTheDocument();
    });
  });

  it("displays guardian information note", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getAllByText(/Erziehungsberechtigte/i).length,
      ).toBeGreaterThan(0);
    });
  });

  it("renders action buttons", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Abbrechen")).toBeInTheDocument();
      expect(screen.getByText("Erstellen")).toBeInTheDocument();
    });
  });

  it("calls onClose when cancel button is clicked", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
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

  it("calls onCreate when form is submitted", async () => {
    mockOnCreate.mockResolvedValue(undefined);

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    const form = screen.getByTestId("modal").querySelector("form");
    expect(form).toBeTruthy();

    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(mockOnCreate).toHaveBeenCalled();
    });
  });

  it("updates form data when input changes", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("first-name-input")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.change(screen.getByTestId("first-name-input"), {
        target: { value: "John" },
      });
    });

    expect(screen.getByTestId("first-name-input")).toHaveValue("John");
  });

  it("resets form data when modal opens", async () => {
    const { rerender } = render(
      <StudentCreateModal
        isOpen={false}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    rerender(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      const firstNameInput = screen.getByTestId("first-name-input");
      expect(firstNameInput).toHaveValue("");
    });
  });

  it("disables buttons when saving", async () => {
    mockOnCreate.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    const form = screen.getByTestId("modal").querySelector("form");
    expect(form).toBeTruthy();

    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(screen.getByText("Wird erstellt...")).toBeInTheDocument();
    });
  });

  it("passes groups prop to PersonalInfoSection", async () => {
    const groups = [
      { value: "1", label: "Group 1" },
      { value: "2", label: "Group 2" },
    ];

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
        groups={groups}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("personal-info-section")).toBeInTheDocument();
    });
  });

  it("opens the reused guardian form when the add button is clicked", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await act(async () => {
      fireEvent.click(screen.getByText("Neu anlegen"));
    });

    expect(screen.getByTestId("guardian-form-modal")).toBeInTheDocument();
  });

  it("adds a submitted guardian to the summary list", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await act(async () => {
      fireEvent.click(screen.getByText("Neu anlegen"));
    });
    await act(async () => {
      fireEvent.click(screen.getByText("MockSubmitGuardian"));
    });

    await waitFor(() => {
      expect(screen.getByText(/Erika Muster/)).toBeInTheDocument();
    });
    // Relationship label and pickup flag are rendered from the payload.
    expect(screen.getByText("Elternteil")).toBeInTheDocument();
    expect(screen.getByText(/Abholberechtigt/)).toBeInTheDocument();
    expect(screen.getByText("1 hinzugefügt")).toBeInTheDocument();
    // Sub-modal closes after collecting.
    expect(screen.queryByTestId("guardian-form-modal")).not.toBeInTheDocument();
  });

  it("removes a guardian from the summary list", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await act(async () => {
      fireEvent.click(screen.getByText("Neu anlegen"));
    });
    await act(async () => {
      fireEvent.click(screen.getByText("MockSubmitGuardian"));
    });

    await waitFor(() => {
      expect(screen.getByText(/Erika Muster/)).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(
        screen.getByLabelText("Erziehungsberechtigte/n entfernen"),
      );
    });

    expect(screen.queryByText(/Erika Muster/)).not.toBeInTheDocument();
    expect(screen.getByText("Optional")).toBeInTheDocument();
  });

  it("forwards collected guardians (snake_case mapped) to onCreate on submit", async () => {
    mockOnCreate.mockResolvedValue(undefined);

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await act(async () => {
      fireEvent.click(screen.getByText("Neu anlegen"));
    });
    await act(async () => {
      fireEvent.click(screen.getByText("MockSubmitGuardian"));
    });
    await waitFor(() => {
      expect(screen.getByText(/Erika Muster/)).toBeInTheDocument();
    });

    const form = screen.getByTestId("modal").querySelector("form");
    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(handleStudentFormSubmit).toHaveBeenCalled();
    });

    // handleSubmit must hand the form payload (arg index 1) a snake_case-mapped
    // guardians array, nested exactly as the create endpoint expects.
    const submitted = vi.mocked(handleStudentFormSubmit).mock
      .calls[0]?.[1] as Record<string, unknown>;
    expect(submitted).toMatchObject({
      guardians: [
        {
          first_name: "Erika",
          last_name: "Muster",
          email: "erika@example.com",
          language_preference: "de",
          relationship_type: "parent",
          is_primary: true,
          is_emergency_contact: false,
          can_pickup: true,
          emergency_priority: 1,
          phone_numbers: [
            {
              phone_number: "0151 2345678",
              phone_type: "mobile",
              is_primary: true,
            },
          ],
        },
      ],
    });
  });

  it("does not attach a guardians field when none were added", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    const form = screen.getByTestId("modal").querySelector("form");
    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(handleStudentFormSubmit).toHaveBeenCalled();
    });
    const submitted = vi.mocked(handleStudentFormSubmit).mock
      .calls[0]?.[1] as Record<string, unknown>;
    expect(submitted).not.toHaveProperty("guardians");
  });

  it("opens the existing-guardian picker when the search button is clicked", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await act(async () => {
      fireEvent.click(screen.getByText("Vorhandene/n suchen"));
    });

    expect(screen.getByTestId("guardian-picker-panel")).toBeInTheDocument();
  });

  it("adds a picked existing guardian to the summary list", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await act(async () => {
      fireEvent.click(screen.getByText("Vorhandene/n suchen"));
    });
    await act(async () => {
      fireEvent.click(screen.getByText("MockPickExisting"));
    });

    await waitFor(() => {
      expect(screen.getByText(/Hans Schmidt/)).toBeInTheDocument();
    });
    expect(screen.getByText("1 hinzugefügt")).toBeInTheDocument();
    // Picker closes after a selection.
    expect(
      screen.queryByTestId("guardian-picker-panel"),
    ).not.toBeInTheDocument();
  });

  it("forwards a picked existing guardian with guardian_profile_id to onCreate", async () => {
    mockOnCreate.mockResolvedValue(undefined);

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await act(async () => {
      fireEvent.click(screen.getByText("Vorhandene/n suchen"));
    });
    await act(async () => {
      fireEvent.click(screen.getByText("MockPickExisting"));
    });
    await waitFor(() => {
      expect(screen.getByText(/Hans Schmidt/)).toBeInTheDocument();
    });

    const form = screen.getByTestId("modal").querySelector("form");
    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(handleStudentFormSubmit).toHaveBeenCalled();
    });

    // The existing selection must carry guardian_profile_id (link, not create)
    // and the relationship flags, with no phone numbers.
    const submitted = vi.mocked(handleStudentFormSubmit).mock
      .calls[0]?.[1] as Record<string, unknown>;
    expect(submitted).toMatchObject({
      guardians: [
        {
          guardian_profile_id: 777,
          relationship_type: "parent",
          can_pickup: true,
          phone_numbers: [],
        },
      ],
    });
  });
});
