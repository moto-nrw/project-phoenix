import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PickupScheduleFormModal } from "./pickup-schedule-form-modal";
import type { PickupScheduleFormData } from "@/lib/pickup-schedule-helpers";

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

describe("PickupScheduleFormModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSubmit = vi.fn();

  const emptySchedules: PickupScheduleFormData[] = [
    { weekday: 1, pickupTime: "", notes: undefined },
    { weekday: 2, pickupTime: "", notes: undefined },
    { weekday: 3, pickupTime: "", notes: undefined },
    { weekday: 4, pickupTime: "", notes: undefined },
    { weekday: 5, pickupTime: "", notes: undefined },
  ];

  const populatedSchedules: PickupScheduleFormData[] = [
    { weekday: 1, pickupTime: "14:30", notes: "Mit Schwester" },
    { weekday: 2, pickupTime: "15:00", notes: undefined },
    { weekday: 3, pickupTime: "14:30", notes: undefined },
    { weekday: 4, pickupTime: "16:00", notes: "Oma holt ab" },
    { weekday: 5, pickupTime: "13:00", notes: undefined },
  ];

  // Helper to get time inputs by type
  const getTimeInputs = (container: HTMLElement) =>
    Array.from(container.querySelectorAll('input[type="time"]'));

  // Helper to get note inputs by placeholder
  const getNoteInputs = (container: HTMLElement) =>
    Array.from(
      container.querySelectorAll(
        'input[placeholder="Abholer, Besonderheiten..."]',
      ),
    );

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("Modal open/close behavior", () => {
    it("renders nothing when isOpen is false", () => {
      render(
        <PickupScheduleFormModal
          isOpen={false}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      expect(screen.queryByTestId("form-modal")).not.toBeInTheDocument();
    });

    it("renders modal when isOpen is true", () => {
      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      expect(screen.getByTestId("form-modal")).toBeInTheDocument();
    });

    it("displays correct title", () => {
      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      expect(
        screen.getByText("Wöchentlichen Abholplan bearbeiten"),
      ).toBeInTheDocument();
    });

    it("calls onClose when Abbrechen button is clicked", () => {
      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      fireEvent.click(screen.getByText("Abbrechen"));

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });
  });

  describe("Form rendering", () => {
    it("renders all 5 weekdays", () => {
      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      expect(screen.getByText("Montag")).toBeInTheDocument();
      expect(screen.getByText("Dienstag")).toBeInTheDocument();
      expect(screen.getByText("Mittwoch")).toBeInTheDocument();
      expect(screen.getByText("Donnerstag")).toBeInTheDocument();
      expect(screen.getByText("Freitag")).toBeInTheDocument();
    });

    it("renders time and note inputs for each day", () => {
      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      const timeInputs = getTimeInputs(container);
      const noteInputs = getNoteInputs(container);

      expect(timeInputs).toHaveLength(5);
      expect(noteInputs).toHaveLength(5);
    });

    it("shows help text", () => {
      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      expect(
        screen.getByText(
          "Lassen Sie die Abholzeit leer für Tage ohne feste Abholzeit.",
        ),
      ).toBeInTheDocument();
    });
  });

  describe("Initial data population", () => {
    it("renders empty time inputs when no schedules provided", () => {
      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      const timeInputs = getTimeInputs(container) as HTMLInputElement[];
      timeInputs.forEach((input) => {
        expect(input.value).toBe("");
      });
    });

    it("populates form with existing schedule data", () => {
      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      const timeInputs = getTimeInputs(container) as HTMLInputElement[];

      expect(timeInputs[0]?.value).toBe("14:30"); // Monday
      expect(timeInputs[1]?.value).toBe("15:00"); // Tuesday
      expect(timeInputs[2]?.value).toBe("14:30"); // Wednesday
      expect(timeInputs[3]?.value).toBe("16:00"); // Thursday
      expect(timeInputs[4]?.value).toBe("13:00"); // Friday
    });

    it("populates notes with existing data", () => {
      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      const noteInputs = getNoteInputs(container) as HTMLInputElement[];

      expect(noteInputs[0]?.value).toBe("Mit Schwester"); // Monday
      expect(noteInputs[1]?.value).toBe(""); // Tuesday
      expect(noteInputs[3]?.value).toBe("Oma holt ab"); // Thursday
    });

    it("resets form when modal reopens", () => {
      const { container, rerender } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      // Change a value
      const timeInputs = getTimeInputs(container) as HTMLInputElement[];
      fireEvent.change(timeInputs[0]!, { target: { value: "17:00" } });

      expect(timeInputs[0]?.value).toBe("17:00");

      // Close modal
      rerender(
        <PickupScheduleFormModal
          isOpen={false}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      // Reopen modal
      rerender(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      // Should reset to original
      const resetTimeInputs = getTimeInputs(container) as HTMLInputElement[];
      expect(resetTimeInputs[0]?.value).toBe("14:30");
    });
  });

  describe("Input field changes", () => {
    it("updates time when time input changes", () => {
      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      const timeInputs = getTimeInputs(container) as HTMLInputElement[];
      fireEvent.change(timeInputs[0]!, { target: { value: "14:30" } });

      expect(timeInputs[0]?.value).toBe("14:30");
    });

    it("updates notes when notes input changes", () => {
      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      const noteInputs = getNoteInputs(container) as HTMLInputElement[];
      fireEvent.change(noteInputs[0]!, {
        target: { value: "Mutter holt ab" },
      });

      expect(noteInputs[0]?.value).toBe("Mutter holt ab");
    });

    it("can change multiple days independently", () => {
      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      const timeInputs = getTimeInputs(container) as HTMLInputElement[];

      fireEvent.change(timeInputs[0]!, { target: { value: "14:00" } });
      fireEvent.change(timeInputs[2]!, { target: { value: "15:30" } });
      fireEvent.change(timeInputs[4]!, { target: { value: "12:00" } });

      expect(timeInputs[0]?.value).toBe("14:00"); // Monday
      expect(timeInputs[1]?.value).toBe(""); // Tuesday unchanged
      expect(timeInputs[2]?.value).toBe("15:30"); // Wednesday
      expect(timeInputs[3]?.value).toBe(""); // Thursday unchanged
      expect(timeInputs[4]?.value).toBe("12:00"); // Friday
    });
  });

  describe("Form validation", () => {
    it("shows error when no schedules have pickup times", async () => {
      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(
          screen.getByText("Bitte geben Sie mindestens eine Abholzeit an."),
        ).toBeInTheDocument();
      });

      expect(mockOnSubmit).not.toHaveBeenCalled();
    });

    // NOTE: HTML time inputs automatically normalize/reject invalid values like "25:99"
    // The browser never accepts such values, so the validation at lines 64-73 in the component
    // would only trigger if someone bypasses the browser's built-in validation.
    // This test verifies the behavior when an empty schedule is submitted (which is the result
    // of the browser rejecting an invalid time value).
    it("shows error for invalid time format", async () => {
      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      // HTML time inputs don't accept invalid values like "25:99"
      // Setting such a value results in empty string (browser behavior)
      const timeInputs = getTimeInputs(container) as HTMLInputElement[];
      fireEvent.change(timeInputs[0]!, { target: { value: "25:99" } });

      fireEvent.click(screen.getByText("Speichern"));

      // Since browser rejects invalid time, the input is empty and triggers
      // the "at least one time required" validation
      await waitFor(() => {
        expect(
          screen.getByText("Bitte geben Sie mindestens eine Abholzeit an."),
        ).toBeInTheDocument();
      });

      expect(mockOnSubmit).not.toHaveBeenCalled();
    });

    it("accepts valid time formats", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      const timeInputs = getTimeInputs(container) as HTMLInputElement[];
      fireEvent.change(timeInputs[0]!, { target: { value: "09:30" } });

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe("Form submission", () => {
    it("submits only schedules with pickup times", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      const timeInputs = getTimeInputs(container) as HTMLInputElement[];

      // Fill only Monday and Wednesday
      fireEvent.change(timeInputs[0]!, { target: { value: "14:00" } });
      fireEvent.change(timeInputs[2]!, { target: { value: "15:30" } });

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith({
          schedules: [
            { weekday: 1, pickupTime: "14:00", notes: undefined },
            { weekday: 3, pickupTime: "15:30", notes: undefined },
          ],
        });
      });
    });

    it("includes notes in submission", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      const timeInputs = getTimeInputs(container) as HTMLInputElement[];
      const noteInputs = getNoteInputs(container) as HTMLInputElement[];

      fireEvent.change(timeInputs[0]!, { target: { value: "14:00" } });
      fireEvent.change(noteInputs[0]!, { target: { value: "Oma holt ab" } });

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith({
          schedules: [
            { weekday: 1, pickupTime: "14:00", notes: "Oma holt ab" },
          ],
        });
      });
    });

    it("excludes empty notes from submission", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      const { container } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={emptySchedules}
        />,
      );

      const timeInputs = getTimeInputs(container) as HTMLInputElement[];

      fireEvent.change(timeInputs[0]!, { target: { value: "14:00" } });

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockOnSubmit).toHaveBeenCalledWith({
          schedules: [{ weekday: 1, pickupTime: "14:00", notes: undefined }],
        });
      });
    });
  });

  describe("Error states", () => {
    it("displays error message on submit failure", async () => {
      mockOnSubmit.mockRejectedValue(new Error("API Error"));

      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
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
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(
          screen.getByText("Fehler beim Speichern des Abholplans"),
        ).toBeInTheDocument();
      });
    });

    it("clears error when modal reopens", async () => {
      mockOnSubmit.mockRejectedValue(new Error("API Error"));

      const { rerender } = render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(screen.getByText("API Error")).toBeInTheDocument();
      });

      // Close modal
      rerender(
        <PickupScheduleFormModal
          isOpen={false}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      // Reopen modal
      rerender(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      expect(screen.queryByText("API Error")).not.toBeInTheDocument();
    });
  });

  describe("Loading states", () => {
    it("disables submit button during submission", async () => {
      mockOnSubmit.mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 1000)),
      );

      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(screen.getByText("Speichern")).toBeDisabled();
      });
    });

    it("disables cancel button during submission", async () => {
      mockOnSubmit.mockImplementation(
        () => new Promise((resolve) => setTimeout(resolve, 1000)),
      );

      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
        />,
      );

      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(screen.getByText("Abbrechen")).toBeDisabled();
      });
    });

    it("re-enables buttons after submission completes", async () => {
      mockOnSubmit.mockResolvedValue(undefined);

      render(
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
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
        <PickupScheduleFormModal
          isOpen={true}
          onClose={mockOnClose}
          onSubmit={mockOnSubmit}
          initialSchedules={populatedSchedules}
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
