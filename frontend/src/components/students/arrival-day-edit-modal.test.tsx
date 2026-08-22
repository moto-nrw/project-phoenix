import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { ArrivalDayData } from "~/lib/arrival-schedule-helpers";
import { useNFCEnabled } from "~/lib/tenant-context";

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    title,
    children,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        {children}
      </div>
    ) : null,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ type, message }: { type: string; message: string }) => (
    <div data-testid="alert" data-type={type}>
      {message}
    </div>
  ),
}));

import { ArrivalDayEditModal } from "./arrival-day-edit-modal";

function makeDay(overrides: Partial<ArrivalDayData> = {}): ArrivalDayData {
  return {
    date: new Date("2024-01-15"),
    weekday: 1,
    isToday: false,
    isWeekend: false,
    showClassTrip: false,
    baseSchedule: null,
    exception: null,
    isAbsent: false,
    isException: false,
    effectiveTime: null,
    notes: [],
    ...overrides,
  } as ArrivalDayData;
}

describe("ArrivalDayEditModal", () => {
  const callbacks = {
    onClose: vi.fn(),
    onSaveException: vi.fn(),
    onDeleteException: vi.fn(),
    onCreateNote: vi.fn(),
    onUpdateNote: vi.fn(),
    onDeleteNote: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useNFCEnabled).mockReturnValue(true);
    Object.values(callbacks).forEach((fn) => fn.mockReset?.());
  });

  it("shows the NFC tablet note only for NFC tenants", () => {
    const props = { isOpen: true, day: makeDay(), ...callbacks };
    const { rerender } = render(<ArrivalDayEditModal {...props} />);

    expect(
      screen.getByText("Notizen werden auch auf den NFC-Tablets angezeigt."),
    ).toBeInTheDocument();

    vi.mocked(useNFCEnabled).mockReturnValue(false);
    rerender(<ArrivalDayEditModal {...props} />);

    expect(screen.queryByText(/NFC-Tablets/i)).not.toBeInTheDocument();
  });

  it("returns null (no dialog) when day is null", () => {
    render(<ArrivalDayEditModal isOpen={true} day={null} {...callbacks} />);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("shows 'no regular day' hint when no baseSchedule", () => {
    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    expect(
      screen.getByText(/Kein regulärer Ankunftstag hinterlegt/),
    ).toBeInTheDocument();
  });

  it("shows regular time when baseSchedule present", () => {
    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          baseSchedule: {
            id: 1,
            student_id: 1,
            weekday: 1,
            weekday_name: "Monday",
            expected_arrival: "08:00:00",
            notes: null,
            created_by: 1,
            created_at: "",
            updated_at: "",
          },
        })}
        {...callbacks}
      />,
    );

    expect(screen.getByText(/Reguläre Zeit: 08:00 Uhr/)).toBeInTheDocument();
  });

  it("clicking 'Abweichende Ankunft eintragen' enters override mode", () => {
    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Abweichende Ankunft/ }),
    );

    expect(screen.getByText("Kind kommt nicht")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Speichern/ }),
    ).toBeInTheDocument();
  });

  it("shows validation error when no time and not absent", async () => {
    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Abweichende Ankunft/ }),
    );
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() =>
      expect(screen.getByText(/Bitte eine Uhrzeit/)).toBeInTheDocument(),
    );
  });

  it("saves exception when time is provided", async () => {
    callbacks.onSaveException.mockResolvedValueOnce(undefined);

    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Abweichende Ankunft/ }),
    );
    const timeInput = screen
      .getByRole("dialog")
      .querySelector("input[type='time']") as HTMLInputElement;
    fireEvent.change(timeInput, { target: { value: "09:30" } });

    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(callbacks.onSaveException).toHaveBeenCalled());
    expect(callbacks.onSaveException).toHaveBeenCalledWith({
      expectedArrival: "09:30",
      reason: null,
    });
  });

  it("saves absence when 'Kind kommt nicht' is checked", async () => {
    callbacks.onSaveException.mockResolvedValueOnce(undefined);

    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Abweichende Ankunft/ }),
    );
    const checkbox = screen.getByRole("checkbox", { name: /Kind kommt nicht/ });
    fireEvent.click(checkbox);
    // Reason input (text input for reason)
    const reasonInput = screen.getByPlaceholderText("Grund (optional)");
    fireEvent.change(reasonInput, { target: { value: "krank" } });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(callbacks.onSaveException).toHaveBeenCalled());
    expect(callbacks.onSaveException).toHaveBeenCalledWith({
      expectedArrival: null,
      reason: "krank",
    });
  });

  it("preloads fields from existing exception (time-based)", () => {
    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          exception: {
            id: 10,
            student_id: 1,
            exception_date: "2024-01-15",
            expected_arrival: "09:30:00",
            reason: "late",
            created_by: 1,
            created_at: "",
            updated_at: "",
          },
        })}
        {...callbacks}
      />,
    );

    // Override UI already shown, no "Abweichende Ankunft eintragen" button
    expect(
      screen.queryByRole("button", { name: /Abweichende Ankunft/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByDisplayValue("09:30")).toBeInTheDocument();
    expect(screen.getByDisplayValue("late")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Ausnahme entfernen/ }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Ändern" })).toBeInTheDocument();
  });

  it("preloads 'markAbsent' from exception without time", () => {
    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          exception: {
            id: 10,
            student_id: 1,
            exception_date: "2024-01-15",
            expected_arrival: null,
            reason: "krank",
            created_by: 1,
            created_at: "",
            updated_at: "",
          },
        })}
        {...callbacks}
      />,
    );

    const checkbox = screen.getByRole("checkbox", { name: /Kind kommt nicht/ });
    expect(checkbox).toBeChecked();
  });

  it("deletes an existing exception", async () => {
    callbacks.onDeleteException.mockResolvedValueOnce(undefined);

    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          exception: {
            id: 10,
            student_id: 1,
            exception_date: "2024-01-15",
            expected_arrival: "09:30:00",
            reason: null,
            created_by: 1,
            created_at: "",
            updated_at: "",
          },
        })}
        {...callbacks}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Ausnahme entfernen/ }));

    await waitFor(() => expect(callbacks.onDeleteException).toHaveBeenCalled());
  });

  it("shows error when exception save throws", async () => {
    callbacks.onSaveException.mockRejectedValueOnce(new Error("Save broke"));

    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Abweichende Ankunft/ }),
    );
    const timeInput = screen
      .getByRole("dialog")
      .querySelector("input[type='time']") as HTMLInputElement;
    fireEvent.change(timeInput, { target: { value: "09:30" } });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() =>
      expect(screen.getByText("Save broke")).toBeInTheDocument(),
    );
  });

  it("shows error when delete exception throws", async () => {
    callbacks.onDeleteException.mockRejectedValueOnce(new Error("Del fail"));

    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          exception: {
            id: 10,
            student_id: 1,
            exception_date: "2024-01-15",
            expected_arrival: "09:30:00",
            reason: null,
            created_by: 1,
            created_at: "",
            updated_at: "",
          },
        })}
        {...callbacks}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Ausnahme entfernen/ }));

    await waitFor(() =>
      expect(screen.getByText("Del fail")).toBeInTheDocument(),
    );
  });

  it("shows 'no notes' hint when notes list empty", () => {
    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    expect(
      screen.getByText("Keine Notizen für diesen Tag."),
    ).toBeInTheDocument();
  });

  it("adds a new note", async () => {
    callbacks.onCreateNote.mockResolvedValueOnce(undefined);

    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Notiz hinzufügen/ }));
    fireEvent.change(screen.getByPlaceholderText("Notiz eingeben..."), {
      target: { value: "  Neue Notiz  " },
    });
    fireEvent.click(screen.getByRole("button", { name: /Hinzufügen/ }));

    await waitFor(() => expect(callbacks.onCreateNote).toHaveBeenCalled());
    expect(callbacks.onCreateNote).toHaveBeenCalledWith("Neue Notiz");
  });

  it("cancels adding a new note", () => {
    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Notiz hinzufügen/ }));
    fireEvent.change(screen.getByPlaceholderText("Notiz eingeben..."), {
      target: { value: "temp" },
    });
    // First cancel button is in notes section ("Abbrechen" for add note)
    fireEvent.click(screen.getAllByRole("button", { name: "Abbrechen" })[0]!);

    expect(
      screen.queryByPlaceholderText("Notiz eingeben..."),
    ).not.toBeInTheDocument();
  });

  it("edits an existing note", async () => {
    callbacks.onUpdateNote.mockResolvedValueOnce(undefined);

    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          notes: [
            {
              id: 77,
              student_id: 1,
              note_date: "2024-01-15",
              content: "hello",
              created_by: 1,
              created_at: "",
              updated_at: "",
            },
          ],
        })}
        {...callbacks}
      />,
    );

    fireEvent.click(screen.getByTitle("Bearbeiten"));
    const textarea = screen.getByDisplayValue("hello");
    fireEvent.change(textarea, { target: { value: "updated" } });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(callbacks.onUpdateNote).toHaveBeenCalled());
    expect(callbacks.onUpdateNote).toHaveBeenCalledWith(77, "updated");
  });

  it("cancels editing a note", () => {
    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          notes: [
            {
              id: 77,
              student_id: 1,
              note_date: "2024-01-15",
              content: "hello",
              created_by: 1,
              created_at: "",
              updated_at: "",
            },
          ],
        })}
        {...callbacks}
      />,
    );

    fireEvent.click(screen.getByTitle("Bearbeiten"));
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(screen.queryByDisplayValue("hello")).not.toBeInTheDocument();
  });

  it("deletes a note", async () => {
    callbacks.onDeleteNote.mockResolvedValueOnce(undefined);

    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          notes: [
            {
              id: 77,
              student_id: 1,
              note_date: "2024-01-15",
              content: "hello",
              created_by: 1,
              created_at: "",
              updated_at: "",
            },
          ],
        })}
        {...callbacks}
      />,
    );

    fireEvent.click(screen.getByTitle("Löschen"));

    await waitFor(() =>
      expect(callbacks.onDeleteNote).toHaveBeenCalledWith(77),
    );
  });

  it("shows error when add note throws", async () => {
    callbacks.onCreateNote.mockRejectedValueOnce(new Error("Note fail"));

    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Notiz hinzufügen/ }));
    fireEvent.change(screen.getByPlaceholderText("Notiz eingeben..."), {
      target: { value: "test" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Hinzufügen/ }));

    await waitFor(() =>
      expect(screen.getByText("Note fail")).toBeInTheDocument(),
    );
  });

  it("shows error when note update throws", async () => {
    callbacks.onUpdateNote.mockRejectedValueOnce(new Error("Upd fail"));

    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          notes: [
            {
              id: 77,
              student_id: 1,
              note_date: "2024-01-15",
              content: "hello",
              created_by: 1,
              created_at: "",
              updated_at: "",
            },
          ],
        })}
        {...callbacks}
      />,
    );

    fireEvent.click(screen.getByTitle("Bearbeiten"));
    fireEvent.change(screen.getByDisplayValue("hello"), {
      target: { value: "updated" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() =>
      expect(screen.getByText("Upd fail")).toBeInTheDocument(),
    );
  });

  it("shows error when note delete throws", async () => {
    callbacks.onDeleteNote.mockRejectedValueOnce(new Error("Del fail"));

    render(
      <ArrivalDayEditModal
        isOpen={true}
        day={makeDay({
          notes: [
            {
              id: 77,
              student_id: 1,
              note_date: "2024-01-15",
              content: "hello",
              created_by: 1,
              created_at: "",
              updated_at: "",
            },
          ],
        })}
        {...callbacks}
      />,
    );

    fireEvent.click(screen.getByTitle("Löschen"));

    await waitFor(() =>
      expect(screen.getByText("Del fail")).toBeInTheDocument(),
    );
  });

  it("exits override mode via 'Abbrechen' when no existing exception", () => {
    render(
      <ArrivalDayEditModal isOpen={true} day={makeDay()} {...callbacks} />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Abweichende Ankunft/ }),
    );
    expect(screen.getByPlaceholderText("Grund (optional)")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(
      screen.queryByPlaceholderText("Grund (optional)"),
    ).not.toBeInTheDocument();
  });
});
