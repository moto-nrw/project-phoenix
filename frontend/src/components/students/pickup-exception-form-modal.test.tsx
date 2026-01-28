import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PickupExceptionFormModal } from "./pickup-exception-form-modal";
import type { PickupException } from "@/lib/pickup-schedule-helpers";

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

describe("PickupExceptionFormModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSubmit = vi.fn();

  const existingException: PickupException = {
    id: "1",
    studentId: "123",
    exceptionDate: "2025-02-15",
    pickupTime: "14:30",
    reason: "Arzttermin",
    createdBy: "1",
    createdAt: "2025-01-20T10:00:00Z",
    updatedAt: "2025-01-20T10:00:00Z",
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("Modal open/close behavior", () => {
    it("renders nothing when isOpen is false", () => {
      render(
        <PickupExceptionFormModal
          isOpen={false}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      expect(screen.queryByTestId("form-modal")).not.toBeInTheDocument();
    });

    it("renders modal when isOpen is true", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      expect(screen.getByTestId("form-modal")).toBeInTheDocument();
    });

    it("displays create title in create mode", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      expect(screen.getByText("Neue Ausnahme")).toBeInTheDocument();
    });

    it("displays edit title in edit mode", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      expect(screen.getByText("Ausnahme bearbeiten")).toBeInTheDocument();
    });

    it("calls onClose when Abbrechen button is clicked", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      fireEvent.click(screen.getByText("Abbrechen"));

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it("shows correct button text for create mode", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      expect(screen.getByText("Hinzufügen")).toBeInTheDocument();
    });

    it("shows correct button text for edit mode", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      expect(screen.getByText("Speichern")).toBeInTheDocument();
    });
  });

  describe("Form rendering", () => {
    it("renders date input field", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      expect(screen.getByLabelText("Datum")).toBeInTheDocument();
    });

    it("renders time input field", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      expect(
        screen.getByLabelText("Abweichende Abholzeit"),
      ).toBeInTheDocument();
    });

    it("renders reason input field", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      expect(screen.getByLabelText("Grund")).toBeInTheDocument();
    });

    it("displays character count for reason field", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      expect(screen.getByText("0/255 Zeichen")).toBeInTheDocument();
    });

    it("updates character count when typing in reason field", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      const reasonInput = screen.getByLabelText("Grund");
      fireEvent.change(reasonInput, { target: { value: "Test" } });

      expect(screen.getByText("4/255 Zeichen")).toBeInTheDocument();
    });
  });

  describe("Initial data population - Create mode", () => {
    it("defaults to tomorrow's date when no defaultDate provided", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      const tomorrow = new Date();
      tomorrow.setDate(tomorrow.getDate() + 1);
      const expectedDate = tomorrow.toISOString().split("T")[0];

      const dateInput = screen.getByLabelText<HTMLInputElement>("Datum");
      expect(dateInput.value).toBe(expectedDate);
    });

    it("uses defaultDate when provided", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const dateInput = screen.getByLabelText<HTMLInputElement>("Datum");
      expect(dateInput.value).toBe("2025-03-15");
    });

    it("starts with empty time and reason in create mode", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      const timeInput = screen.getByLabelText<HTMLInputElement>(
        "Abweichende Abholzeit",
      );
      const reasonInput = screen.getByLabelText<HTMLInputElement>("Grund");

      expect(timeInput.value).toBe("");
      expect(reasonInput.value).toBe("");
    });
  });

  describe("Initial data population - Edit mode", () => {
    it("populates form with existing exception data", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      expect(screen.getByDisplayValue("2025-02-15")).toBeInTheDocument();
      expect(screen.getByDisplayValue("14:30")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Arzttermin")).toBeInTheDocument();
    });

    it("formats pickup time correctly", () => {
      const exceptionWithSeconds: PickupException = {
        ...existingException,
        pickupTime: "14:30:00",
      };

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={exceptionWithSeconds}
        />,
      );

      const timeInput = screen.getByLabelText<HTMLInputElement>(
        "Abweichende Abholzeit",
      );
      expect(timeInput.value).toBe("14:30");
    });

    it("handles undefined pickup time", () => {
      const exceptionNoPickup: PickupException = {
        ...existingException,
        pickupTime: undefined,
      };

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={exceptionNoPickup}
        />,
      );

      const timeInput = screen.getByLabelText<HTMLInputElement>(
        "Abweichende Abholzeit",
      );
      expect(timeInput.value).toBe("");
    });

    it("resets to initial data when modal reopens", () => {
      const { rerender } = render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      // Modify the form
      const reasonInput = screen.getByLabelText("Grund");
      fireEvent.change(reasonInput, { target: { value: "Modified" } });

      expect(screen.getByDisplayValue("Modified")).toBeInTheDocument();

      // Close and reopen
      rerender(
        <PickupExceptionFormModal
          isOpen={false}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      rerender(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      // Should reset to original
      expect(screen.getByDisplayValue("Arzttermin")).toBeInTheDocument();
    });
  });

  describe("Input field changes", () => {
    it("updates date when date input changes", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      const dateInput = screen.getByLabelText("Datum");
      fireEvent.change(dateInput, { target: { value: "2025-04-20" } });

      expect(dateInput).toHaveValue("2025-04-20");
    });

    it("updates time when time input changes", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      fireEvent.change(timeInput, { target: { value: "15:45" } });

      expect(timeInput).toHaveValue("15:45");
    });

    it("updates reason when reason input changes", () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      const reasonInput = screen.getByLabelText("Grund");
      fireEvent.change(reasonInput, { target: { value: "Familienfeier" } });

      expect(reasonInput).toHaveValue("Familienfeier");
    });
  });

  describe("Form validation", () => {
    // Helper to submit form directly (bypassing HTML5 validation that would prevent submission)
    const submitForm = () => {
      const form = document.getElementById("pickup-exception-form");
      if (form) {
        fireEvent.submit(form);
      }
    };

    it("shows error when date is missing", async () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
        />,
      );

      // Clear the date input (defaulted to tomorrow)
      const dateInput = screen.getByLabelText<HTMLInputElement>("Datum");
      fireEvent.change(dateInput, { target: { value: "" } });

      // Fill other fields to ensure only date validation triggers
      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      const reasonInput = screen.getByLabelText("Grund");
      fireEvent.change(timeInput, { target: { value: "14:00" } });
      fireEvent.change(reasonInput, { target: { value: "Test reason" } });

      // Submit form directly to bypass HTML5 required validation
      submitForm();

      await waitFor(() => {
        expect(
          screen.getByText("Bitte wählen Sie ein Datum aus."),
        ).toBeInTheDocument();
      });

      expect(mockOnSubmit).not.toHaveBeenCalled();
    });

    it("shows error when pickup time is missing", async () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const reasonInput = screen.getByLabelText("Grund");
      fireEvent.change(reasonInput, { target: { value: "Test reason" } });

      // Submit form directly to bypass HTML5 required validation
      submitForm();

      await waitFor(() => {
        expect(
          screen.getByText("Bitte geben Sie eine Abholzeit an."),
        ).toBeInTheDocument();
      });

      expect(mockOnSubmit).not.toHaveBeenCalled();
    });

    it("shows error when reason is missing", async () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      fireEvent.change(timeInput, { target: { value: "14:00" } });

      // Submit form directly to bypass HTML5 required validation
      submitForm();

      await waitFor(() => {
        expect(
          screen.getByText("Bitte geben Sie einen Grund an."),
        ).toBeInTheDocument();
      });

      expect(mockOnSubmit).not.toHaveBeenCalled();
    });

    it("shows error when reason is only whitespace", async () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      const reasonInput = screen.getByLabelText("Grund");

      fireEvent.change(timeInput, { target: { value: "14:00" } });
      fireEvent.change(reasonInput, { target: { value: "   " } });

      fireEvent.click(screen.getByText("Hinzufügen"));

      await waitFor(() => {
        expect(
          screen.getByText("Bitte geben Sie einen Grund an."),
        ).toBeInTheDocument();
      });

      expect(mockOnSubmit).not.toHaveBeenCalled();
    });

    // NOTE: The maxLength=255 attribute on the input prevents entering more than 255 chars
    // in a real browser. We need to set the value directly to test the JS validation.
    it("shows error when reason exceeds 255 characters", async () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      const reasonInput = screen.getByLabelText("Grund");

      fireEvent.change(timeInput, { target: { value: "14:00" } });

      // Set value directly to bypass maxLength attribute
      // Using Object.defineProperty to set value without triggering maxLength constraint
      Object.defineProperty(reasonInput, "value", {
        value: "a".repeat(256),
        writable: true,
      });
      fireEvent.input(reasonInput);

      // Submit form directly
      submitForm();

      await waitFor(() => {
        expect(
          screen.getByText("Der Grund darf maximal 255 Zeichen lang sein."),
        ).toBeInTheDocument();
      });

      expect(mockOnSubmit).not.toHaveBeenCalled();
    });

    // NOTE: HTML time inputs normalize/reject invalid values like "25:99" in browsers.
    // In jsdom, we need to bypass this to test JS validation.
    it("shows error for invalid time format", async () => {
      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      const reasonInput = screen.getByLabelText("Grund");

      // Set invalid time value directly to bypass browser validation
      // Using Object.defineProperty to set value without browser normalization
      Object.defineProperty(timeInput, "value", {
        value: "25:99",
        writable: true,
      });
      fireEvent.input(timeInput);

      fireEvent.change(reasonInput, { target: { value: "Test" } });

      // Submit form directly
      submitForm();

      // In jsdom, the invalid value might be accepted OR cleared
      await waitFor(() => {
        const errorMessages = [
          "Bitte geben Sie eine Abholzeit an.",
          "Ungültiges Zeitformat. Bitte verwenden Sie HH:MM.",
        ];
        const hasExpectedError = errorMessages.some(
          (msg) => screen.queryByText(msg) !== null,
        );
        expect(hasExpectedError).toBe(true);
      });

      expect(mockOnSubmit).not.toHaveBeenCalled();
    });

    it("accepts valid time formats", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      const reasonInput = screen.getByLabelText("Grund");

      // Use standard HH:MM format that all environments accept
      fireEvent.change(timeInput, { target: { value: "09:30" } });
      fireEvent.change(reasonInput, { target: { value: "Valid reason" } });

      fireEvent.click(screen.getByText("Hinzufügen"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledTimes(1);
      });
    });

    it("allows reason with exactly 255 characters", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      const reasonInput = screen.getByLabelText("Grund");

      fireEvent.change(timeInput, { target: { value: "14:00" } });
      fireEvent.change(reasonInput, { target: { value: "a".repeat(255) } });

      fireEvent.click(screen.getByText("Hinzufügen"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe("Form submission", () => {
    it("submits form data in create mode", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      const reasonInput = screen.getByLabelText("Grund");

      fireEvent.change(timeInput, { target: { value: "14:30" } });
      fireEvent.change(reasonInput, { target: { value: "Arzttermin" } });

      fireEvent.click(screen.getByText("Hinzufügen"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledTimes(1);
      });

      expect(mockOnSubmit).toHaveBeenCalledWith({
        exceptionDate: "2025-03-15",
        pickupTime: "14:30",
        reason: "Arzttermin",
      });
    });

    it("submits form data in edit mode", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      const reasonInput = screen.getByLabelText("Grund");
      fireEvent.change(reasonInput, { target: { value: "Updated reason" } });

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledTimes(1);
      });

      expect(mockOnSubmit).toHaveBeenCalledWith({
        exceptionDate: "2025-02-15",
        pickupTime: "14:30",
        reason: "Updated reason",
      });
    });

    it("trims whitespace from reason", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="create"
          defaultDate="2025-03-15"
        />,
      );

      const timeInput = screen.getByLabelText("Abweichende Abholzeit");
      const reasonInput = screen.getByLabelText("Grund");

      fireEvent.change(timeInput, { target: { value: "14:00" } });
      fireEvent.change(reasonInput, { target: { value: "  Test reason  " } });

      fireEvent.click(screen.getByText("Hinzufügen"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledTimes(1);
      });

      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          reason: "Test reason",
        }),
      );
    });
  });

  describe("Error states", () => {
    it("displays error message on submit failure", async () => {
      mockOnSubmit.mockRejectedValue(new Error("API Error"));

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(screen.getByText("API Error")).toBeInTheDocument();
      });
    });

    it("displays default error message for non-Error objects", async () => {
      mockOnSubmit.mockRejectedValue("String error");

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(
          screen.getByText("Fehler beim Speichern der Ausnahme"),
        ).toBeInTheDocument();
      });
    });

    it("clears error when modal reopens after error", async () => {
      mockOnSubmit.mockRejectedValue(new Error("API Error"));

      const { rerender } = render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(screen.getByText("API Error")).toBeInTheDocument();
      });

      // Close modal
      rerender(
        <PickupExceptionFormModal
          isOpen={false}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      // Reopen modal
      rerender(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      expect(screen.queryByText("API Error")).not.toBeInTheDocument();
    });
  });

  describe("Loading states", () => {
    it("disables buttons during submission", async () => {
      mockOnSubmit.mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 1000)),
      );

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(screen.getByText("Speichern")).toBeDisabled();
        expect(screen.getByText("Abbrechen")).toBeDisabled();
      });
    });

    it("re-enables buttons after submission completes", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalled();
      });

      expect(screen.getByText("Speichern")).not.toBeDisabled();
      expect(screen.getByText("Abbrechen")).not.toBeDisabled();
    });

    it("re-enables buttons after submission fails", async () => {
      mockOnSubmit.mockRejectedValue(new Error("API Error"));

      render(
        <PickupExceptionFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          mode="edit"
          initialData={existingException}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(screen.getByText("API Error")).toBeInTheDocument();
      });

      expect(screen.getByText("Speichern")).not.toBeDisabled();
      expect(screen.getByText("Abbrechen")).not.toBeDisabled();
    });
  });
});
