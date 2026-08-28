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
  within,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { StudentCreateModal } from "./student-create-modal";
import { handleStudentFormSubmit } from "~/lib/student-form-validation";

const { mockFetchArrivalSettings } = vi.hoisted(() => ({
  mockFetchArrivalSettings: vi.fn(),
}));

vi.mock("~/lib/student-arrival-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/student-arrival-api")
  >("~/lib/student-arrival-api");
  return {
    ...actual,
    fetchArrivalSettings: mockFetchArrivalSettings,
  };
});

// Der Erstell-Dialog laeuft als SlideOver (Vaul). Vaul rendert in jsdom nicht,
// deshalb wird das Panel hier durch dieselbe Struktur ohne Animationsschicht
// ersetzt; data-testid="modal" bleibt der Einstieg der Tests.
vi.mock("~/components/ui/slide-over", () => ({
  SlideOver: ({
    open,
    onOpenChange,
    children,
  }: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    children: React.ReactNode;
  }) =>
    open ? (
      <div data-testid="modal">
        <button type="button" onClick={() => onOpenChange(false)}>
          Close
        </button>
        {children}
      </div>
    ) : null,
  SlideOverContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SlideOverHeader: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SlideOverFooter: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SlideOverTitle: ({ children }: { children: React.ReactNode }) => (
    <h2>{children}</h2>
  ),
  SlideOverDescription: ({ children }: { children: React.ReactNode }) => (
    <p>{children}</p>
  ),
  SlideOverCloseButton: (
    props: React.ButtonHTMLAttributes<HTMLButtonElement>,
  ) => <button type="button" {...props} />,
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
  // Unified per-day departure selector (#1610).
  DepartureSection: ({
    days,
    onChange,
  }: {
    days?: Record<string, string>;
    onChange: (v: Record<string, string>) => void;
  }) => (
    <div data-testid="departure-section">
      <span data-testid="departure-mon">{days?.mon ?? "alone"}</span>
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
                  guardianRole: "legal_guardian",
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

// Mock the reused CareWeeklyPlanModal the same way as the guardian modal: when
// open, expose a button that calls onSubmit with one canned arrival + pickup
// entry (the shapes the real modal emits). This exercises StudentCreateModal's
// staging/summary/payload logic without driving the full weekly-plan editor.
vi.mock("./care-weekly-plan-modal", () => ({
  CareWeeklyPlanModal: ({
    isOpen,
    onClose,
    onSubmit,
    careDaysSource,
  }: {
    isOpen: boolean;
    onClose: () => void;
    careDaysSource: "weekly_plan" | "bookings";
    onSubmit: (data: {
      arrivalSchedules: Array<{
        weekday: number;
        inCare: boolean;
        expected_arrival: string;
        notes: string | null;
      }>;
      pickupData: {
        schedules: Array<{
          weekday: number;
          pickupTime: string;
          notes?: string;
        }>;
      };
    }) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid="care-weekly-plan-modal">
        <span data-testid="care-days-source">{careDaysSource}</span>
        <button
          type="button"
          onClick={() =>
            // Mirror the real modal: after onSubmit resolves it closes itself.
            void onSubmit({
              arrivalSchedules: [
                {
                  weekday: 1,
                  inCare: true,
                  expected_arrival: "07:30",
                  notes: "Bus",
                },
                {
                  weekday: 3,
                  inCare: true,
                  expected_arrival: "08:00",
                  notes: null,
                },
              ],
              pickupData: {
                schedules: [{ weekday: 1, pickupTime: "15:00", notes: "Oma" }],
              },
            }).then(() => onClose())
          }
        >
          MockSubmitCarePlan
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
              guardianRole: "legal_guardian",
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
    mockFetchArrivalSettings.mockResolvedValue({
      care_days_source: "weekly_plan",
    });
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
      expect(screen.getByText("Neues Kind")).toBeInTheDocument();
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

  it("renders the unified departure section", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await waitFor(() => {
      expect(screen.getByTestId("departure-section")).toBeInTheDocument();
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

    await screen.findByTestId("personal-info-section");
    const form = screen.getByTestId("modal").querySelector("form");
    expect(form).toBeTruthy();

    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(mockOnCreate).toHaveBeenCalled();
    });
  });

  it("passes a scroll-to-error callback to the submit helper", async () => {
    mockOnCreate.mockResolvedValue(undefined);

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await screen.findByTestId("personal-info-section");
    const form = screen.getByTestId("modal").querySelector("form");
    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(handleStudentFormSubmit).toHaveBeenCalled();
    });
    // 7th arg drives scroll-to-first-error on a failed submit (shared with the
    // parents' enrollment form via useScrollToFirstError).
    const onError = vi.mocked(handleStudentFormSubmit).mock.calls[0]?.[6];
    expect(onError).toBeTypeOf("function");
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

    await screen.findByTestId("personal-info-section");
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

    await screen.findByTestId("personal-info-section");
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

    await screen.findByTestId("personal-info-section");
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

    await screen.findByTestId("personal-info-section");
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
    expect(
      within(
        screen.getByRole("region", { name: "Erziehungsberechtigte" }),
      ).getByText("Optional"),
    ).toBeInTheDocument();
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

    await screen.findByTestId("personal-info-section");
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
          guardian_role: "legal_guardian",
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

    await screen.findByTestId("personal-info-section");
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

  it("submits an explicit empty pickup_days map (goes-home-alone) for an untouched create", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await screen.findByTestId("personal-info-section");
    const form = screen.getByTestId("modal").querySelector("form");
    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(handleStudentFormSubmit).toHaveBeenCalled();
    });
    const submitted = vi.mocked(handleStudentFormSubmit).mock
      .calls[0]?.[1] as Record<string, unknown>;
    // The new UI semantics say "no selected days = goes home alone", and that
    // must be sent as an explicit empty map (not omitted) so the stored
    // pickup_days/pickup_status pair is correct without anyone editing later.
    expect(submitted.pickup_days).toEqual({});
    expect(submitted.departure_days).toEqual({});
    expect(submitted.pickup_status).toBe("Geht alleine nach Hause");
  });

  it("opens the reused weekly care-plan editor when the add button is clicked", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await screen.findByTestId("personal-info-section");
    await act(async () => {
      fireEvent.click(screen.getByText("Wochenplan hinzufügen"));
    });

    expect(screen.getByTestId("care-weekly-plan-modal")).toBeInTheDocument();
    expect(screen.getByTestId("care-days-source")).toHaveTextContent(
      "weekly_plan",
    );
  });

  it("routes OGS creation through manual enrollment when bookings are authoritative", async () => {
    mockFetchArrivalSettings.mockResolvedValueOnce({
      care_days_source: "bookings",
    });

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(
      await screen.findByText(
        /Die Betreuungstage dieser Schule kommen aus Buchungen\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Anmeldephasen öffnen" }),
    ).toHaveAttribute("href", "/enrollment-phases");
    expect(
      screen.queryByTestId("personal-info-section"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Erstellen")).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("care-weekly-plan-modal"),
    ).not.toBeInTheDocument();
  });

  it("keeps class-list-only creation available in booking mode", async () => {
    mockFetchArrivalSettings.mockResolvedValueOnce({
      care_days_source: "bookings",
    });

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
        onCreateListEntry={vi.fn()}
      />,
    );

    await screen.findByText(
      /Die Betreuungstage dieser Schule kommen aus Buchungen\./,
    );
    fireEvent.click(screen.getByRole("button", { name: "Nur Klassenliste" }));

    expect(
      screen.getByRole("textbox", { name: /Vorname/ }),
    ).toBeInTheDocument();
    expect(screen.getByText("Erstellen")).toBeInTheDocument();
  });

  it("blocks direct creation when care days cannot be loaded", async () => {
    mockFetchArrivalSettings.mockRejectedValueOnce(new Error("offline"));

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    expect(
      await screen.findByText(
        "Die Betreuungstage konnten nicht geladen werden. Schließen Sie das Fenster und öffnen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("care-weekly-plan-modal"),
    ).not.toBeInTheDocument();
  });

  it("stages a submitted care plan and shows the weekday summary", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await screen.findByTestId("personal-info-section");
    await act(async () => {
      fireEvent.click(screen.getByText("Wochenplan hinzufügen"));
    });
    await act(async () => {
      fireEvent.click(screen.getByText("MockSubmitCarePlan"));
    });

    // Summary lists the covered weekdays (Mo from arrival+pickup, Mi from
    // arrival) and the per-type counts, and the add button flips to "edit".
    await waitFor(() => {
      expect(screen.getByText("Mo · Mi")).toBeInTheDocument();
    });
    expect(screen.getByText("2× Ankunft · 1× Abholung")).toBeInTheDocument();
    expect(screen.getByText("2 Tage")).toBeInTheDocument();
    expect(screen.getByText("Wochenplan bearbeiten")).toBeInTheDocument();
    // Sub-modal closes after collecting.
    expect(
      screen.queryByTestId("care-weekly-plan-modal"),
    ).not.toBeInTheDocument();
  });

  it("removes the staged care plan via the remove button", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await screen.findByTestId("personal-info-section");
    await act(async () => {
      fireEvent.click(screen.getByText("Wochenplan hinzufügen"));
    });
    await act(async () => {
      fireEvent.click(screen.getByText("MockSubmitCarePlan"));
    });
    await waitFor(() => {
      expect(screen.getByText("Mo · Mi")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByLabelText("Betreuungszeiten entfernen"));
    });

    expect(screen.queryByText("Mo · Mi")).not.toBeInTheDocument();
    // Section reverts to the "Optional" empty state + "add" wording.
    expect(
      within(
        screen.getByRole("region", { name: "Betreuungszeiten" }),
      ).getByText("Optional"),
    ).toBeInTheDocument();
    expect(screen.getByText("Wochenplan hinzufügen")).toBeInTheDocument();
  });

  it("forwards staged schedules (snake_case mapped) to the submit payload", async () => {
    mockOnCreate.mockResolvedValue(undefined);

    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await screen.findByTestId("personal-info-section");
    await act(async () => {
      fireEvent.click(screen.getByText("Wochenplan hinzufügen"));
    });
    await act(async () => {
      fireEvent.click(screen.getByText("MockSubmitCarePlan"));
    });
    await waitFor(() => {
      expect(screen.getByText("Mo · Mi")).toBeInTheDocument();
    });

    const form = screen.getByTestId("modal").querySelector("form");
    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(handleStudentFormSubmit).toHaveBeenCalled();
    });

    // Arrival entries travel backend-shaped already; pickup entries are mapped
    // from the care-plan form (pickupTime → pickup_time) before submit.
    const submitted = vi.mocked(handleStudentFormSubmit).mock
      .calls[0]?.[1] as Record<string, unknown>;
    expect(submitted).toMatchObject({
      arrival_schedules: [
        {
          weekday: 1,
          inCare: true,
          expected_arrival: "07:30",
          notes: "Bus",
        },
        {
          weekday: 3,
          inCare: true,
          expected_arrival: "08:00",
          notes: null,
        },
      ],
      pickup_schedules: [{ weekday: 1, pickup_time: "15:00", notes: "Oma" }],
    });
  });

  it("does not attach schedule fields when no care plan was staged", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await screen.findByTestId("personal-info-section");
    const form = screen.getByTestId("modal").querySelector("form");
    await act(async () => {
      fireEvent.submit(form!);
    });

    await waitFor(() => {
      expect(handleStudentFormSubmit).toHaveBeenCalled();
    });
    const submitted = vi.mocked(handleStudentFormSubmit).mock
      .calls[0]?.[1] as Record<string, unknown>;
    expect(submitted).not.toHaveProperty("arrival_schedules");
    expect(submitted).not.toHaveProperty("pickup_schedules");
  });

  it("opens the existing-guardian picker when the search button is clicked", async () => {
    render(
      <StudentCreateModal
        isOpen={true}
        onClose={mockOnClose}
        onCreate={mockOnCreate}
      />,
    );

    await screen.findByTestId("personal-info-section");
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

    await screen.findByTestId("personal-info-section");
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

    await screen.findByTestId("personal-info-section");
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
          guardian_role: "legal_guardian",
          can_pickup: true,
          phone_numbers: [],
        },
      ],
    });
  });
});
