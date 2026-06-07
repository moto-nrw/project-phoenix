import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CareDayOverrideModal } from "./care-day-override-modal";
import type { ArrivalDayData } from "~/lib/arrival-schedule-helpers";
import type { DayData as PickupDayData } from "~/lib/pickup-schedule-helpers";

const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
  }),
}));

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

const baseArrivalDay: ArrivalDayData = {
  date: new Date("2026-05-25T00:00:00"),
  weekday: 1,
  isToday: true,
  showSick: false,
  showExcused: false,
  exception: undefined,
  baseSchedule: {
    id: 1,
    student_id: 42,
    weekday: 1,
    weekday_name: "Montag",
    expected_arrival: "08:00",
    notes: "Haupteingang",
    created_by: 1,
    created_at: "2026-05-01T00:00:00Z",
    updated_at: "2026-05-01T00:00:00Z",
  },
  effectiveTime: "08:00",
  effectiveReason: "Haupteingang",
  isException: false,
  isAbsent: false,
  notes: [
    {
      id: 11,
      student_id: 42,
      note_date: "2026-05-25",
      content: "Kommt mit Oma",
      created_by: 1,
      created_at: "2026-05-01T00:00:00Z",
      updated_at: "2026-05-01T00:00:00Z",
    },
  ],
};

const basePickupDay: PickupDayData = {
  date: new Date("2026-05-25T00:00:00"),
  weekday: 1,
  isToday: true,
  showSick: false,
  showExcused: false,
  exception: undefined,
  baseSchedule: {
    id: "1",
    studentId: "42",
    weekday: 1,
    weekdayName: "Montag",
    pickupTime: "15:00",
    notes: "Bus",
    createdBy: "1",
    createdAt: "2026-05-01T00:00:00Z",
    updatedAt: "2026-05-01T00:00:00Z",
  },
  effectiveTime: "15:00",
  effectiveNotes: "Bus",
  isException: false,
  notes: [
    {
      id: "21",
      studentId: "42",
      noteDate: "2026-05-25",
      content: "Traffic",
      createdBy: "1",
      createdAt: "2026-05-01T00:00:00Z",
      updatedAt: "2026-05-01T00:00:00Z",
    },
  ],
};

function renderModal(
  props: Partial<React.ComponentProps<typeof CareDayOverrideModal>> = {},
) {
  return render(
    <CareDayOverrideModal
      isOpen
      onClose={vi.fn()}
      arrivalDay={baseArrivalDay}
      pickupDay={basePickupDay}
      onSaveArrivalException={vi.fn().mockResolvedValue(undefined)}
      onDeleteArrivalException={vi.fn().mockResolvedValue(undefined)}
      onSavePickupException={vi.fn().mockResolvedValue(undefined)}
      onDeletePickupException={vi.fn().mockResolvedValue(undefined)}
      onCreateArrivalNote={vi.fn().mockResolvedValue(undefined)}
      onDeleteArrivalNote={vi.fn().mockResolvedValue(undefined)}
      onCreatePickupNote={vi.fn().mockResolvedValue(undefined)}
      onDeletePickupNote={vi.fn().mockResolvedValue(undefined)}
      {...props}
    />,
  );
}

describe("CareDayOverrideModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("saves arrival and pickup exceptions with a pickup note", async () => {
    const onSaveArrivalException = vi.fn().mockResolvedValue(undefined);
    const onSavePickupException = vi.fn().mockResolvedValue(undefined);
    const onCreatePickupNote = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();

    renderModal({
      onSaveArrivalException,
      onSavePickupException,
      onCreatePickupNote,
      onClose,
    });

    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[0]!);
    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[1]!);
    const timeFields = screen.getAllByLabelText("Uhrzeit");
    fireEvent.change(timeFields[0]!, { target: { value: "10:10" } });
    fireEvent.change(timeFields[1]!, { target: { value: "15:15" } });
    const reasonFields = screen.getAllByLabelText("Grund");
    fireEvent.change(reasonFields[0]!, { target: { value: "Arzt" } });
    fireEvent.change(reasonFields[1]!, { target: { value: "Training" } });
    fireEvent.change(screen.getByLabelText("Neue Notiz"), {
      target: { value: "Bitte anrufen" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Tagesänderung speichern" }),
    );

    await waitFor(() => {
      expect(onSaveArrivalException).toHaveBeenCalledWith({
        expectedArrival: "10:10",
        reason: "Arzt",
      });
      expect(onSavePickupException).toHaveBeenCalledWith({
        pickupTime: "15:15",
        reason: "Training",
      });
      expect(onCreatePickupNote).toHaveBeenCalledWith("Bitte anrufen");
    });
    expect(toastSuccess).toHaveBeenCalledWith(
      "Tagesänderung wurde gespeichert",
    );
    expect(onClose).toHaveBeenCalled();
  });

  it("deletes existing arrival and pickup notes", async () => {
    const onDeleteArrivalNote = vi.fn().mockResolvedValue(undefined);
    const onDeletePickupNote = vi.fn().mockResolvedValue(undefined);

    renderModal({ onDeleteArrivalNote, onDeletePickupNote });

    const deleteButtons = screen.getAllByRole("button", {
      name: "Notiz löschen",
    });
    fireEvent.click(deleteButtons[0]!);
    fireEvent.click(deleteButtons[1]!);

    await waitFor(() => {
      expect(onDeleteArrivalNote).toHaveBeenCalledWith(11);
      expect(onDeletePickupNote).toHaveBeenCalledWith("21");
    });
  });

  it("validates invalid times and shows submit errors", async () => {
    const onSaveArrivalException = vi
      .fn()
      .mockRejectedValue(new Error("Speichern fehlgeschlagen"));

    renderModal({ onSaveArrivalException });

    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[0]!);
    fireEvent.change(screen.getByLabelText("Uhrzeit"), {
      target: { value: "99:99" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Tagesänderung speichern" }),
    );

    expect(
      screen.getByText("Bitte eine gültige Ankunftszeit eingeben."),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Uhrzeit"), {
      target: { value: "09:30" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Tagesänderung speichern" }),
    );

    await waitFor(() => {
      expect(screen.getByText("Speichern fehlgeschlagen")).toBeInTheDocument();
    });
    expect(toastError).toHaveBeenCalledWith("Speichern fehlgeschlagen");
  });
});
