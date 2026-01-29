import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PickupDayEditModal } from "./pickup-day-edit-modal";
import type { DayData, PickupNote } from "@/lib/pickup-schedule-helpers";

// Mock lucide-react icons
vi.mock("lucide-react", () => ({
  Loader2: () => <span data-testid="loader-icon">Loading</span>,
  Plus: () => <span data-testid="plus-icon">+</span>,
  Pencil: () => <span data-testid="pencil-icon">Edit</span>,
  Trash2: () => <span data-testid="trash-icon">Delete</span>,
  X: () => <span data-testid="x-icon">X</span>,
  StickyNote: () => <span data-testid="sticky-note-icon">Note</span>,
}));

// Mock FormModal to be testable
vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    onClose: _onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="form-modal">
        <div data-testid="modal-title">{title}</div>
        {children}
      </div>
    ) : null,
}));

const createMockDayData = (overrides?: Partial<DayData>): DayData => ({
  date: new Date("2025-01-28"),
  weekday: 2,
  isToday: false,
  showSick: false,
  exception: undefined,
  baseSchedule: {
    id: "1",
    studentId: "123",
    weekday: 2,
    weekdayName: "Dienstag",
    pickupTime: "15:30",
    notes: "Regular schedule",
    createdBy: "1",
    createdAt: "2025-01-01T00:00:00Z",
    updatedAt: "2025-01-01T00:00:00Z",
  },
  effectiveTime: "15:30",
  effectiveNotes: "Regular schedule",
  isException: false,
  notes: [],
  ...overrides,
});

const createMockNote = (id: string, content: string): PickupNote => ({
  id,
  studentId: "123",
  noteDate: "2025-01-28",
  content,
  createdBy: "1",
  createdAt: "2025-01-28T10:00:00Z",
  updatedAt: "2025-01-28T10:00:00Z",
});

describe("PickupDayEditModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSaveException = vi.fn();
  const mockOnDeleteException = vi.fn();
  const mockOnCreateNote = vi.fn();
  const mockOnUpdateNote = vi.fn();
  const mockOnDeleteNote = vi.fn();

  const defaultProps = {
    onClose: mockOnClose,
    studentId: "123",
    onSaveException: mockOnSaveException,
    onDeleteException: mockOnDeleteException,
    onCreateNote: mockOnCreateNote,
    onUpdateNote: mockOnUpdateNote,
    onDeleteNote: mockOnDeleteNote,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockOnSaveException.mockResolvedValue(undefined);
    mockOnDeleteException.mockResolvedValue(undefined);
    mockOnCreateNote.mockResolvedValue(undefined);
    mockOnUpdateNote.mockResolvedValue(undefined);
    mockOnDeleteNote.mockResolvedValue(undefined);
  });

  describe("Rendering", () => {
    it("renders nothing when not open", () => {
      const { container } = render(
        <PickupDayEditModal
          isOpen={false}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );
      expect(container).toBeEmptyDOMElement();
    });

    it("renders nothing when day is null", () => {
      render(<PickupDayEditModal isOpen={true} day={null} {...defaultProps} />);
      expect(screen.queryByTestId("form-modal")).not.toBeInTheDocument();
    });

    it("renders modal with correct title", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );
      expect(screen.getByTestId("modal-title")).toHaveTextContent(
        "Dienstag, 28.01.",
      );
    });

    it("renders with existing exception showing time input", () => {
      const day = createMockDayData({
        exception: {
          id: "exc-1",
          studentId: "123",
          exceptionDate: "2025-01-28",
          pickupTime: "14:00",
          reason: "Arzttermin",
          createdBy: "1",
          createdAt: "2025-01-27T00:00:00Z",
          updatedAt: "2025-01-27T00:00:00Z",
        },
        isException: true,
        effectiveTime: "14:00",
      });

      render(<PickupDayEditModal isOpen={true} day={day} {...defaultProps} />);

      // Should show time input pre-filled with exception time
      const timeInput = screen.getByDisplayValue("14:00");
      expect(timeInput).toBeInTheDocument();
    });

    it("renders empty notes state", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );
      expect(
        screen.getByText("Keine Notizen für diesen Tag."),
      ).toBeInTheDocument();
    });

    it("renders existing notes", () => {
      const day = createMockDayData({
        notes: [
          createMockNote("note-1", "First note"),
          createMockNote("note-2", "Second note"),
        ],
      });

      render(<PickupDayEditModal isOpen={true} day={day} {...defaultProps} />);

      expect(screen.getByText("First note")).toBeInTheDocument();
      expect(screen.getByText("Second note")).toBeInTheDocument();
    });

    it("shows base schedule time label", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );
      expect(screen.getByText("Reguläre Zeit: 15:30 Uhr")).toBeInTheDocument();
    });
  });

  describe("Exception Time Override", () => {
    it("shows add override button when no exception exists", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );
      expect(
        screen.getByText("Abweichende Zeit eintragen"),
      ).toBeInTheDocument();
    });

    it("shows time input when override button is clicked", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Abweichende Zeit eintragen"));
      expect(screen.getByText("Speichern")).toBeInTheDocument();
    });

    it("saves exception with valid time", async () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Abweichende Zeit eintragen"));

      const timeInput = screen.getByDisplayValue("");
      fireEvent.change(timeInput, { target: { value: "14:00" } });
      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockOnSaveException).toHaveBeenCalledWith({
          pickupTime: "14:00",
        });
      });
    });

    it("saves exception with empty time (undefined pickupTime)", async () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Abweichende Zeit eintragen"));
      // Don't set a time — leave empty
      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(mockOnSaveException).toHaveBeenCalledWith({
          pickupTime: undefined,
        });
      });
    });

    it("shows loading state while saving", async () => {
      let resolvePromise: () => void;
      const promise = new Promise<void>((resolve) => {
        resolvePromise = resolve;
      });
      mockOnSaveException.mockReturnValue(promise);

      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Abweichende Zeit eintragen"));

      const timeInput = screen.getByDisplayValue("");
      fireEvent.change(timeInput, { target: { value: "14:00" } });
      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(screen.getByTestId("loader-icon")).toBeInTheDocument();
      });

      resolvePromise!();

      await waitFor(() => {
        expect(screen.queryByTestId("loader-icon")).not.toBeInTheDocument();
      });
    });

    it("removes existing exception", async () => {
      const day = createMockDayData({
        exception: {
          id: "exc-1",
          studentId: "123",
          exceptionDate: "2025-01-28",
          pickupTime: "14:00",
          reason: "Arzttermin",
          createdBy: "1",
          createdAt: "2025-01-27T00:00:00Z",
          updatedAt: "2025-01-27T00:00:00Z",
        },
        isException: true,
      });

      render(<PickupDayEditModal isOpen={true} day={day} {...defaultProps} />);

      fireEvent.click(screen.getByTitle("Zeitausnahme entfernen"));

      await waitFor(() => {
        expect(mockOnDeleteException).toHaveBeenCalled();
      });
    });

    it("cancels time override entry", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Abweichende Zeit eintragen"));
      expect(screen.getByText("Speichern")).toBeInTheDocument();

      fireEvent.click(screen.getByTitle("Abbrechen"));
      expect(
        screen.getByText("Abweichende Zeit eintragen"),
      ).toBeInTheDocument();
    });

    it("shows error message when save fails", async () => {
      mockOnSaveException.mockRejectedValueOnce(new Error("Save failed"));

      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Abweichende Zeit eintragen"));

      const timeInput = screen.getByDisplayValue("");
      fireEvent.change(timeInput, { target: { value: "14:00" } });
      fireEvent.click(screen.getByText("Speichern"));

      await waitFor(() => {
        expect(screen.getByText("Save failed")).toBeInTheDocument();
      });
    });

    it("shows error when remove exception fails", async () => {
      mockOnDeleteException.mockRejectedValueOnce(new Error("Delete failed"));

      const day = createMockDayData({
        exception: {
          id: "exc-1",
          studentId: "123",
          exceptionDate: "2025-01-28",
          pickupTime: "14:00",
          reason: "Arzttermin",
          createdBy: "1",
          createdAt: "2025-01-27T00:00:00Z",
          updatedAt: "2025-01-27T00:00:00Z",
        },
        isException: true,
      });

      render(<PickupDayEditModal isOpen={true} day={day} {...defaultProps} />);

      fireEvent.click(screen.getByTitle("Zeitausnahme entfernen"));

      await waitFor(() => {
        expect(screen.getByText("Delete failed")).toBeInTheDocument();
      });
    });
  });

  describe("Notes Management", () => {
    it("shows add note button", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );
      expect(screen.getByText("Notiz hinzufügen")).toBeInTheDocument();
    });

    it("shows note input when add button is clicked", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Notiz hinzufügen"));
      expect(
        screen.getByPlaceholderText("Notiz eingeben..."),
      ).toBeInTheDocument();
    });

    it("creates a new note", async () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Notiz hinzufügen"));

      const noteInput = screen.getByPlaceholderText("Notiz eingeben...");
      fireEvent.change(noteInput, { target: { value: "New test note" } });
      fireEvent.click(screen.getByText("Hinzufügen"));

      await waitFor(() => {
        expect(mockOnCreateNote).toHaveBeenCalledWith("New test note");
      });
    });

    it("does not create empty note", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Notiz hinzufügen"));

      const noteInput = screen.getByPlaceholderText("Notiz eingeben...");
      fireEvent.change(noteInput, { target: { value: "   " } });

      const addButton = screen.getByText("Hinzufügen");
      expect(addButton).toBeDisabled();
    });

    it("cancels adding note", () => {
      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Notiz hinzufügen"));
      expect(
        screen.getByPlaceholderText("Notiz eingeben..."),
      ).toBeInTheDocument();

      // Find the Abbrechen button in the add-note section
      const cancelButtons = screen.getAllByText("Abbrechen");
      fireEvent.click(cancelButtons[cancelButtons.length - 1]!);

      expect(
        screen.queryByPlaceholderText("Notiz eingeben..."),
      ).not.toBeInTheDocument();
    });

    it("starts editing a note", () => {
      const day = createMockDayData({
        notes: [createMockNote("note-1", "Original content")],
      });

      render(<PickupDayEditModal isOpen={true} day={day} {...defaultProps} />);

      fireEvent.click(screen.getByTitle("Bearbeiten"));
      expect(screen.getByDisplayValue("Original content")).toBeInTheDocument();
    });

    it("updates an existing note", async () => {
      const day = createMockDayData({
        notes: [createMockNote("note-1", "Original content")],
      });

      render(<PickupDayEditModal isOpen={true} day={day} {...defaultProps} />);

      fireEvent.click(screen.getByTitle("Bearbeiten"));

      const editInput = screen.getByDisplayValue("Original content");
      fireEvent.change(editInput, { target: { value: "Updated content" } });

      // Click the Speichern button in the edit section
      const saveButtons = screen.getAllByText("Speichern");
      fireEvent.click(saveButtons[saveButtons.length - 1]!);

      await waitFor(() => {
        expect(mockOnUpdateNote).toHaveBeenCalledWith(
          "note-1",
          "Updated content",
        );
      });
    });

    it("cancels editing a note", () => {
      const day = createMockDayData({
        notes: [createMockNote("note-1", "Original content")],
      });

      render(<PickupDayEditModal isOpen={true} day={day} {...defaultProps} />);

      fireEvent.click(screen.getByTitle("Bearbeiten"));

      const editInput = screen.getByDisplayValue("Original content");
      fireEvent.change(editInput, { target: { value: "Changed" } });

      const cancelButtons = screen.getAllByText("Abbrechen");
      fireEvent.click(cancelButtons[0]!);

      expect(screen.getByText("Original content")).toBeInTheDocument();
      expect(screen.queryByDisplayValue("Changed")).not.toBeInTheDocument();
    });

    it("deletes a note", async () => {
      const day = createMockDayData({
        notes: [createMockNote("note-1", "Note to delete")],
      });

      render(<PickupDayEditModal isOpen={true} day={day} {...defaultProps} />);

      fireEvent.click(screen.getByTitle("Löschen"));

      await waitFor(() => {
        expect(mockOnDeleteNote).toHaveBeenCalledWith("note-1");
      });
    });

    it("shows error when create note fails", async () => {
      mockOnCreateNote.mockRejectedValueOnce(new Error("Create failed"));

      render(
        <PickupDayEditModal
          isOpen={true}
          day={createMockDayData()}
          {...defaultProps}
        />,
      );

      fireEvent.click(screen.getByText("Notiz hinzufügen"));

      const noteInput = screen.getByPlaceholderText("Notiz eingeben...");
      fireEvent.change(noteInput, { target: { value: "Test note" } });
      fireEvent.click(screen.getByText("Hinzufügen"));

      await waitFor(() => {
        expect(screen.getByText("Create failed")).toBeInTheDocument();
      });
    });

    it("shows error when delete note fails", async () => {
      mockOnDeleteNote.mockRejectedValueOnce(new Error("Delete note failed"));

      const day = createMockDayData({
        notes: [createMockNote("note-1", "Note content")],
      });

      render(<PickupDayEditModal isOpen={true} day={day} {...defaultProps} />);

      fireEvent.click(screen.getByTitle("Löschen"));

      await waitFor(() => {
        expect(screen.getByText("Delete note failed")).toBeInTheDocument();
      });
    });
  });

  describe("Form Reset", () => {
    it("resets form when day changes", () => {
      const day1 = createMockDayData();
      const day2 = createMockDayData({
        date: new Date("2025-01-29"),
        weekday: 3,
      });

      const { rerender } = render(
        <PickupDayEditModal isOpen={true} day={day1} {...defaultProps} />,
      );

      // Start adding a note
      fireEvent.click(screen.getByText("Notiz hinzufügen"));
      expect(
        screen.getByPlaceholderText("Notiz eingeben..."),
      ).toBeInTheDocument();

      // Change day
      rerender(
        <PickupDayEditModal isOpen={true} day={day2} {...defaultProps} />,
      );

      // Note input should be gone
      expect(
        screen.queryByPlaceholderText("Notiz eingeben..."),
      ).not.toBeInTheDocument();
    });
  });
});
