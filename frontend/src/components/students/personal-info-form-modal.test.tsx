import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PersonalInfoFormModal } from "./personal-info-form-modal";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";

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
        <button onClick={onClose} data-testid="close-modal">
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
    health_info: "Keine Allergien",
    supervisor_notes: "Betreuernotiz",
    extra_info: "Elternnotiz",
    ...overrides,
  });

  beforeEach(() => {
    vi.clearAllMocks();
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

      // bus_days fold into "Bus" on Monday; Friday defaults to "Alleine".
      expect(
        screen.getByRole("button", { name: "Montag: Bus", pressed: true }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Freitag: Alleine", pressed: true }),
      ).toBeInTheDocument();
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
        screen.getByRole("button", { name: "Montag: Abholung", pressed: true }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", {
          name: "Mittwoch: Abholung",
          pressed: true,
        }),
      ).toBeInTheDocument();
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

      fireEvent.click(screen.getByRole("button", { name: "Montag: Bus" }));
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(mockOnSave).toHaveBeenCalledWith(
          expect.objectContaining({
            departure_days: { mon: "bus" },
            bus_days: { mon: true },
            buskind: true,
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
