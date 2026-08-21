import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PersonalInfoFormModal } from "./personal-info-form-modal";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import type { StudentCompanion } from "~/lib/student-companion-api";
import { CompanionPlanConflictError } from "~/lib/api";

// The birthday field moved from a native input to the kit picker; this stub
// keeps it readable/settable as an input. Imported inside the factory because
// vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

const { fetchStudentCompanionsMock } = vi.hoisted(() => ({
  fetchStudentCompanionsMock: vi.fn<
    (studentId: string) => Promise<StudentCompanion[]>
  >(() => new Promise(() => undefined)),
}));

vi.mock("~/lib/student-companion-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-companion-api")>()),
  fetchStudentCompanions: fetchStudentCompanionsMock,
}));

// Mock FormModal
vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    onClose,
    title,
    footer,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    footer: React.ReactNode;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="form-modal">
        <h1>{title}</h1>
        <button type="button" onClick={onClose} data-testid="close-modal">
          Close
        </button>
        {children}
        <div data-testid="modal-footer">{footer}</div>
      </div>
    ) : null,
}));

// Mock ToastContext
const mockToast = {
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
  warning: vi.fn(),
};

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToast,
}));

// Mock icons
vi.mock("./student-detail-components", () => ({
  ChevronDownIcon: ({ className }: { className?: string }) => (
    <span data-testid="chevron-icon" className={className} />
  ),
  WarningIcon: () => <span data-testid="warning-icon" />,
}));

describe("PersonalInfoFormModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSave = vi.fn();

  const createMockStudent = (
    overrides: Partial<ExtendedStudent> = {},
  ): ExtendedStudent => ({
    id: "123",
    name: "Max Mustermann",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "3a",
    current_location: "Raum 1",
    bus: false,
    birthday: "2015-05-15",
    buskind: false,
    sick: false,
    pickup_status: "Wird abgeholt",
    address_street: "Musterstraße 12",
    address_city: "Köln",
    address_postal_code: "50667",
    health_info: "Keine Allergien",
    supervisor_notes: "Betreuernotiz",
    extra_info: "Elternnotiz",
    ...overrides,
  });

  beforeEach(() => {
    vi.clearAllMocks();
    fetchStudentCompanionsMock.mockImplementation(
      () => new Promise(() => undefined),
    );
  });

  describe("Modal open/close behavior", () => {
    it("renders nothing when isOpen is false", () => {
      render(
        <PersonalInfoFormModal
          isOpen={false}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      expect(screen.queryByTestId("form-modal")).not.toBeInTheDocument();
    });

    it("renders modal when isOpen is true", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      expect(screen.getByTestId("form-modal")).toBeInTheDocument();
    });

    it("displays correct title", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      expect(screen.getByText("Persönliche Infos")).toBeInTheDocument();
    });
  });

  describe("Form fields", () => {
    it("displays student first name in input", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({ first_name: "Anna" })}
          onSave={mockOnSave}
        />,
      );

      const input = screen.getByLabelText<HTMLInputElement>("Vorname");
      expect(input.value).toBe("Anna");
    });

    it("displays student last name in input", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({ second_name: "Schmidt" })}
          onSave={mockOnSave}
        />,
      );

      const input = screen.getByLabelText<HTMLInputElement>("Nachname");
      expect(input.value).toBe("Schmidt");
    });

    it("displays school class in input", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({ school_class: "4b" })}
          onSave={mockOnSave}
        />,
      );

      const input = screen.getByLabelText<HTMLInputElement>("Klasse");
      expect(input.value).toBe("4b");
    });

    it("displays birthday in date input", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({ birthday: "2016-03-20" })}
          onSave={mockOnSave}
        />,
      );

      const input = screen.getByLabelText<HTMLInputElement>("Geburtsdatum");
      expect(input.value).toBe("2016-03-20");
    });

    it("displays address inputs", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      expect(
        screen.getByLabelText<HTMLInputElement>("Straße und Hausnummer").value,
      ).toBe("Musterstraße 12");
      expect(screen.getByLabelText<HTMLInputElement>("PLZ").value).toBe(
        "50667",
      );
      expect(screen.getByLabelText<HTMLInputElement>("Ort").value).toBe("Köln");
    });

    it("updates first name when changed", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      const input = screen.getByLabelText<HTMLInputElement>("Vorname");
      fireEvent.change(input, { target: { value: "Neuer Name" } });

      expect(input.value).toBe("Neuer Name");
    });

    it("does not render sick toggle (moved to StudentSickReportSection)", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      expect(screen.queryByRole("switch")).not.toBeInTheDocument();
      expect(screen.queryByText("Krankmeldung")).not.toBeInTheDocument();
    });
  });

  describe("Save functionality", () => {
    it("calls onSave with updated student data", async () => {
      mockOnSave.mockResolvedValue(undefined);

      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      const saveButton = screen.getByText("Speichern");
      fireEvent.click(saveButton);

      await waitFor(() => {
        expect(mockOnSave).toHaveBeenCalled();
      });
    });

    it("closes modal after successful save", async () => {
      mockOnSave.mockResolvedValue(undefined);

      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      const saveButton = screen.getByText("Speichern");
      fireEvent.click(saveButton);

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalled();
      });
    });

    it("shows error toast when save fails", async () => {
      mockOnSave.mockRejectedValue(new Error("Save failed"));

      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      const saveButton = screen.getByText("Speichern");
      fireEvent.click(saveButton);

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Fehler beim Speichern der persönlichen Informationen",
        );
      });
    });

    // The stranded-companion refusal is user-actionable: it says which child's
    // Heimweg has to be filled in before the link can be removed. The generic
    // save-failed toast would leave that instruction unread and the user with
    // no way to resolve the refusal.
    it("keeps the backend message when a link would strand the other child", async () => {
      const { CompanionDepartureRefusedError } = await import("~/lib/api");
      mockOnSave.mockRejectedValue(
        new CompanionDepartureRefusedError(
          JSON.stringify({
            status: "error",
            error:
              "Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.",
            code: "companion_would_lose_departure",
          }),
        ),
      );

      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.",
        );
      });
    });

    it("shows loading state while saving", async () => {
      mockOnSave.mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 100)),
      );

      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      const saveButton = screen.getByText("Speichern");
      fireEvent.click(saveButton);

      expect(screen.getByText("Wird gespeichert...")).toBeInTheDocument();

      await waitFor(() => {
        expect(mockOnClose).toHaveBeenCalled();
      });
    });
  });

  describe("Cancel functionality", () => {
    it("resets form and closes modal on cancel", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({ first_name: "Original" })}
          onSave={mockOnSave}
        />,
      );

      // Change the value
      const input = screen.getByLabelText<HTMLInputElement>("Vorname");
      fireEvent.change(input, { target: { value: "Changed" } });
      expect(input.value).toBe("Changed");

      // Click cancel
      const cancelButton = screen.getByText("Abbrechen");
      fireEvent.click(cancelButton);

      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  describe("Select inputs", () => {
    it("shows bus days as 'Bus' in the unified departure picker (#1610)", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({
            buskind: true,
            bus_days: { mon: true },
          })}
          onSave={mockOnSave}
        />,
      );

      // bus_days fold into "Bus" on Monday; Friday remains unselected.
      expect(
        screen.getByRole("checkbox", { name: "Montag: Bus" }),
      ).toBeChecked();
      expect(
        screen.getByRole("checkbox", { name: "Freitag: Zu Fuß" }),
      ).not.toBeChecked();
    });

    it("shows pickup days as 'Abholung' in the departure picker", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({
            pickup_status: "Wird abgeholt",
            pickup_days: { mon: true, wed: true },
          })}
          onSave={mockOnSave}
        />,
      );

      expect(
        screen.getByRole("checkbox", { name: "Montag: Abgeholt" }),
      ).toBeChecked();
      expect(
        screen.getByRole("checkbox", {
          name: "Mittwoch: Abgeholt",
        }),
      ).toBeChecked();
    });

    it("sets a bus day and saves the derived bus_days", async () => {
      mockOnSave.mockResolvedValue(undefined);
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({ buskind: false })}
          onSave={mockOnSave}
        />,
      );

      fireEvent.click(screen.getByRole("checkbox", { name: "Montag: Bus" }));
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(mockOnSave).toHaveBeenCalledWith(
          expect.objectContaining({
            allowed_departure_modes: { mon: ["bus"] },
            departure_days: { mon: "bus" },
            bus_days: { mon: true },
            buskind: true,
          }),
        );
      });
    });

    it("sends derived departure_days when only legacy day maps exist", async () => {
      mockOnSave.mockResolvedValue(undefined);
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({
            bus_days: { mon: true },
            pickup_days: { wed: true },
            departure_days: undefined,
          })}
          onSave={mockOnSave}
        />,
      );

      fireEvent.change(screen.getByLabelText<HTMLInputElement>("Vorname"), {
        target: { value: "Maja" },
      });
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(mockOnSave).toHaveBeenCalledWith(
          expect.objectContaining({
            first_name: "Maja",
            allowed_departure_modes: { mon: ["bus"], wed: ["pickup"] },
            departure_days: { mon: "bus", wed: "pickup" },
          }),
        );
      });
    });
  });

  describe("Laufgemeinschaft editability", () => {
    // The submitted companion list REPLACES the stored one. Editing it before
    // the stored links are on screen would either be overwritten the moment
    // the fetch resolves, or — after a failed load — be dropped without a word,
    // because the save deliberately sends no companion list it never read.
    // So the picker only appears once the load succeeded.
    const accompaniedStudentProps = {
      allowed_departure_modes: { mon: ["accompanied"] },
    } as unknown as Partial<ExtendedStudent>;

    it("offers the picker once the stored links are loaded", async () => {
      fetchStudentCompanionsMock.mockResolvedValue([
        { companion_student_id: "42", first_name: "Lina", weekdays: ["mon"] },
      ]);

      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent(accompaniedStudentProps)}
          onSave={mockOnSave}
        />,
      );

      expect(
        await screen.findByRole("button", { name: /Kind hinzufügen/ }),
      ).toBeInTheDocument();
      expect(screen.getByText("Lina")).toBeInTheDocument();
    });

    it("keeps the picker read-only while the companions are still loading", async () => {
      let resolveCompanions: (companions: StudentCompanion[]) => void = () =>
        undefined;
      fetchStudentCompanionsMock.mockImplementation(
        () =>
          new Promise<StudentCompanion[]>((resolve) => {
            resolveCompanions = resolve;
          }),
      );

      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent(accompaniedStudentProps)}
          onSave={mockOnSave}
        />,
      );

      // The accompanied day IS selected, so the only reason the picker is
      // missing is the pending load.
      expect(
        screen.getByRole("checkbox", { name: "Montag: Anderes Kind" }),
      ).toBeChecked();
      expect(
        screen.queryByRole("button", { name: /Kind hinzufügen/ }),
      ).not.toBeInTheDocument();

      resolveCompanions([]);

      expect(
        await screen.findByRole("button", { name: /Kind hinzufügen/ }),
      ).toBeInTheDocument();
    });

    it("keeps the picker read-only after the companions failed to load", async () => {
      fetchStudentCompanionsMock.mockRejectedValue(new Error("500"));

      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent(accompaniedStudentProps)}
          onSave={mockOnSave}
        />,
      );

      // The user is told why the section is missing instead of being handed an
      // empty list that would look like "no links exist".
      expect(
        await screen.findByText(/Laufgemeinschaft konnte nicht geladen werden/),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /Kind hinzufügen/ }),
      ).not.toBeInTheDocument();
    });

    // Taking the last accompanied day away hides the picker, so a list left in
    // the draft would ride along on the save with nobody able to see or edit
    // it — and the backend refuses a list on a plan that allows the mode on no
    // day at all, an error about a control that is no longer on screen.
    it("clears the linked children when the last accompanied day is removed", async () => {
      mockOnSave.mockResolvedValue(undefined);
      fetchStudentCompanionsMock.mockResolvedValue([
        { companion_student_id: "42", first_name: "Lina", weekdays: ["mon"] },
      ]);

      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent(accompaniedStudentProps)}
          onSave={mockOnSave}
        />,
      );

      await screen.findByRole("button", { name: /Kind hinzufügen/ });
      fireEvent.click(
        screen.getByRole("checkbox", { name: "Montag: Anderes Kind" }),
      );
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(mockOnSave).toHaveBeenCalledWith(
          expect.objectContaining({
            companions: [],
            allowed_departure_modes: {},
          }),
        );
      });
    });
  });

  describe("Textarea inputs", () => {
    it("displays health info in textarea", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent({ health_info: "Hat Allergie" })}
          onSave={mockOnSave}
        />,
      );

      const textarea = screen.getByLabelText<HTMLTextAreaElement>(
        "Gesundheitsinformationen",
      );
      expect(textarea.value).toBe("Hat Allergie");
    });

    it("updates health info when changed", () => {
      render(
        <PersonalInfoFormModal
          isOpen={true}
          onClose={mockOnClose}
          student={createMockStudent()}
          onSave={mockOnSave}
        />,
      );

      const textarea = screen.getByLabelText<HTMLTextAreaElement>(
        "Gesundheitsinformationen",
      );
      fireEvent.change(textarea, { target: { value: "Neue Info" } });

      expect(textarea.value).toBe("Neue Info");
    });
  });
});

// A companion-plan 409 is a question ("may we widen the linked child's own
// Heimweg?"), and the answer decides whether a write lands on a DIFFERENT
// child. Dismissing it must leave nothing behind that a later save carries.
describe("PersonalInfoFormModal — companion plan conflicts", () => {
  const student: ExtendedStudent = {
    id: "123",
    name: "Max Mustermann",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "3a",
    current_location: "Raum 1",
    bus: false,
    birthday: "2015-05-15",
    buskind: false,
    sick: false,
    pickup_status: "Wird abgeholt",
  };

  beforeEach(() => {
    vi.clearAllMocks();
    fetchStudentCompanionsMock.mockImplementation(
      () => new Promise(() => undefined),
    );
  });

  it("drops the conflicts when the question is dismissed", async () => {
    const onSave = vi
      .fn()
      .mockRejectedValueOnce(
        new CompanionPlanConflictError(
          JSON.stringify({
            error:
              "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
            conflicts: [{ student_id: 42, weekdays: ["mon", "tue"] }],
          }),
        ),
      )
      .mockResolvedValue(undefined);

    render(
      <PersonalInfoFormModal
        isOpen={true}
        onClose={vi.fn()}
        student={student}
        onSave={onSave}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByRole("button", { name: "Ergänzen und speichern" });

    // The banner's "Abbrechen" sits above the footer's — dismissing the
    // question, not the modal.
    fireEvent.click(screen.getAllByRole("button", { name: "Abbrechen" })[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
    expect(onSave.mock.calls[1]![0]).toMatchObject({
      confirmed_companion_extensions: [],
    });
  });
});
