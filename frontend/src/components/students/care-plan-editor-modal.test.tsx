import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  CarePlanEditorModal,
  schoolDaysBetween,
} from "./care-plan-editor-modal";
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

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    title,
    children,
    onConfirm,
    onClose,
    confirmText,
    cancelText,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    onConfirm: () => void;
    onClose: () => void;
    confirmText: string;
    cancelText: string;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        {children}
        <button type="button" onClick={onClose}>
          {cancelText}
        </button>
        <button type="button" onClick={onConfirm}>
          {confirmText}
        </button>
      </div>
    ) : null,
}));

// The kit date picker renders a calendar popover; a plain date input keeps
// these tests about the editor's scope logic rather than calendar interaction.
vi.mock("~/components/ui/date-picker", () => ({
  ISODatePicker: ({
    label,
    value,
    onChange,
  }: {
    label?: string;
    value: string;
    onChange: (value: string) => void;
  }) => (
    <label>
      {label}
      <input
        type="date"
        aria-label={label}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  ),
}));

const MONDAY = new Date("2026-05-25T00:00:00");

const baseArrivalDay: ArrivalDayData = {
  date: MONDAY,
  weekday: 1,
  isToday: true,
  showSick: false,
  showClassTrip: false,
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
  date: MONDAY,
  weekday: 1,
  isToday: true,
  showSick: false,
  showClassTrip: false,
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

const weeklyArrival = [
  { weekday: 1, expected_arrival: "08:00", notes: "Haupteingang" },
  { weekday: 2, expected_arrival: "08:15", notes: null },
  { weekday: 3, expected_arrival: "", notes: null },
  { weekday: 4, expected_arrival: "", notes: null },
  { weekday: 5, expected_arrival: "", notes: null },
];

const weeklyPickup = [
  { weekday: 1, pickupTime: "15:00", notes: "Bus" },
  { weekday: 2, pickupTime: "16:00" },
  { weekday: 3, pickupTime: "" },
  { weekday: 4, pickupTime: "" },
  { weekday: 5, pickupTime: "" },
];

function renderEditor(
  props: Partial<React.ComponentProps<typeof CarePlanEditorModal>> = {},
) {
  const onSubmitExceptions = vi.fn().mockResolvedValue(undefined);
  const onSubmitWeekly = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  render(
    <CarePlanEditorModal
      isOpen
      onClose={onClose}
      date={MONDAY}
      arrivalDay={baseArrivalDay}
      pickupDay={basePickupDay}
      weeklyArrival={weeklyArrival}
      weeklyPickup={weeklyPickup}
      guardianArrivalDates={[]}
      guardianPickupDates={[]}
      onSubmitExceptions={onSubmitExceptions}
      onSubmitWeekly={onSubmitWeekly}
      onDeleteArrivalNote={vi.fn().mockResolvedValue(undefined)}
      onDeletePickupNote={vi.fn().mockResolvedValue(undefined)}
      {...props}
    />,
  );
  return { onSubmitExceptions, onSubmitWeekly, onClose };
}

function save(): void {
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
}

describe("CarePlanEditorModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("offers all three scopes when opened from a day", () => {
    renderEditor();

    expect(
      screen.getByRole("radio", { name: /Nur an diesem Tag/ }),
    ).toHaveAttribute("aria-checked", "true");
    expect(
      screen.getByRole("radio", { name: /An mehreren Tagen/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /Jeden Montag/ }),
    ).toBeInTheDocument();
  });

  it("shows only the weekly plan when opened without a day", () => {
    renderEditor({ date: null, arrivalDay: null, pickupDay: null });

    expect(screen.queryByRole("radio")).not.toBeInTheDocument();
    expect(screen.getByText("Wochenplan bearbeiten")).toBeInTheDocument();
    expect(
      screen.getByText(/Gilt ab sofort für alle kommenden Wochen/),
    ).toBeInTheDocument();
  });

  it("saves a single-day override with a note", async () => {
    const { onSubmitExceptions } = renderEditor();

    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[1]!);
    fireEvent.change(screen.getByLabelText("Uhrzeit"), {
      target: { value: "16:30" },
    });
    fireEvent.change(screen.getByLabelText("Neue Notiz"), {
      target: { value: "Oma holt ab" },
    });
    save();

    await waitFor(() => {
      expect(onSubmitExceptions).toHaveBeenCalledWith({
        dates: ["2026-05-25"],
        arrival: null,
        pickup: { kind: "time", time: "16:30", reason: null },
        note: { target: "pickup", content: "Oma holt ab" },
      });
    });
  });

  it("can set 'keine Abholung', which the old day editor could not express", async () => {
    const { onSubmitExceptions } = renderEditor();

    fireEvent.click(screen.getByRole("button", { name: "Keine Abholung" }));
    save();

    await waitFor(() => {
      expect(onSubmitExceptions).toHaveBeenCalledWith(
        expect.objectContaining({
          pickup: { kind: "none", reason: null },
        }),
      );
    });
  });

  it("expands a range to its school days and skips the weekend", async () => {
    const { onSubmitExceptions } = renderEditor();

    fireEvent.click(screen.getByRole("radio", { name: /An mehreren Tagen/ }));
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-06-01" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Keine Abholung" }));
    save();

    await waitFor(() => {
      expect(onSubmitExceptions).toHaveBeenCalledWith(
        expect.objectContaining({
          dates: [
            "2026-05-25",
            "2026-05-26",
            "2026-05-27",
            "2026-05-28",
            "2026-05-29",
            "2026-06-01",
          ],
        }),
      );
    });
  });

  it("leaves an untouched leg alone across a range", async () => {
    // The form is seeded from ONE day, so "Regulär" on the arrival side says
    // nothing about the other days in the range. Writing it anyway would delete
    // arrival overrides (parent-set ones included) across the whole range.
    const { onSubmitExceptions } = renderEditor();

    fireEvent.click(screen.getByRole("radio", { name: /An mehreren Tagen/ }));
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-05-27" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Keine Abholung" }));
    save();

    await waitFor(() => {
      expect(onSubmitExceptions).toHaveBeenCalledWith(
        expect.objectContaining({ arrival: null }),
      );
    });
  });

  it("refuses to save when nothing was changed", async () => {
    const { onSubmitExceptions } = renderEditor();

    save();

    expect(
      await screen.findByText("Bitte zuerst eine Zeit oder eine Notiz ändern."),
    ).toBeInTheDocument();
    expect(onSubmitExceptions).not.toHaveBeenCalled();
  });

  it("refuses a range without an end date", async () => {
    const { onSubmitExceptions } = renderEditor();

    fireEvent.click(screen.getByRole("radio", { name: /An mehreren Tagen/ }));
    save();

    expect(
      await screen.findByText(/Bitte ein Enddatum wählen/),
    ).toBeInTheDocument();
    expect(onSubmitExceptions).not.toHaveBeenCalled();
  });

  it("writes the weekly plan when the recurring scope is chosen", async () => {
    const { onSubmitWeekly } = renderEditor();

    fireEvent.click(screen.getByRole("radio", { name: /Jeden Montag/ }));
    fireEvent.change(
      screen.getByLabelText("Ankunft", { selector: "#weekly-arrival-1" }),
      {
        target: { value: "08:45" },
      },
    );
    save();

    await waitFor(() => {
      expect(onSubmitWeekly).toHaveBeenCalledWith({
        arrivalSchedules: [
          { weekday: 1, expected_arrival: "08:45", notes: "Haupteingang" },
          { weekday: 2, expected_arrival: "08:15", notes: null },
        ],
        pickupSchedules: [
          { weekday: 1, pickupTime: "15:00", notes: "Bus" },
          { weekday: 2, pickupTime: "16:00", notes: undefined },
        ],
      });
    });
  });

  it("confirms before an emptied weekly field deletes a scheduled time", async () => {
    const { onSubmitWeekly } = renderEditor();

    fireEvent.click(screen.getByRole("radio", { name: /Jeden Montag/ }));
    fireEvent.change(
      screen.getByLabelText("Abholung", { selector: "#weekly-pickup-2" }),
      { target: { value: "" } },
    );
    save();

    expect(
      await screen.findByRole("dialog", { name: "Zeiten entfernen?" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Dienstag Abholung")).toBeInTheDocument();
    expect(onSubmitWeekly).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Trotzdem speichern" }));

    await waitFor(() => {
      expect(onSubmitWeekly).toHaveBeenCalledWith(
        expect.objectContaining({
          pickupSchedules: [{ weekday: 1, pickupTime: "15:00", notes: "Bus" }],
        }),
      );
    });
  });

  it("surfaces a recurring note in the day view instead of hiding it", () => {
    renderEditor();

    // The weekly note used to appear on the day card but nowhere in the day
    // editor, leaving no way to reach it (#893).
    expect(screen.getByText("Haupteingang")).toBeInTheDocument();
    expect(screen.getAllByText(/Jede Woche/).length).toBeGreaterThan(0);
  });

  it("switches to the weekly scope from a recurring note", () => {
    renderEditor();

    fireEvent.click(
      screen.getAllByRole("button", { name: "Im Wochenplan ändern" })[0]!,
    );

    expect(screen.getByRole("radio", { name: /Jeden Montag/ })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });

  it("asks before overwriting a parent-set time", async () => {
    const { onSubmitExceptions } = renderEditor({
      guardianPickupDates: ["2026-05-25"],
    });

    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[1]!);
    fireEvent.change(screen.getByLabelText("Uhrzeit"), {
      target: { value: "16:30" },
    });
    save();

    expect(
      await screen.findByRole("dialog", {
        name: "Eltern-Angabe überschreiben?",
      }),
    ).toBeInTheDocument();
    expect(onSubmitExceptions).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Trotzdem überschreiben" }),
    );

    await waitFor(() => {
      expect(onSubmitExceptions).toHaveBeenCalled();
    });
  });

  it("does not ask, and does not rewrite the leg, when only a note is added", async () => {
    const { onSubmitExceptions } = renderEditor({
      guardianPickupDates: ["2026-05-25"],
    });

    fireEvent.change(screen.getByLabelText("Neue Notiz"), {
      target: { value: "Nur eine Notiz" },
    });
    save();

    await waitFor(() => {
      expect(onSubmitExceptions).toHaveBeenCalledWith({
        dates: ["2026-05-25"],
        arrival: null,
        pickup: null,
        note: { target: "pickup", content: "Nur eine Notiz" },
      });
    });
    expect(
      screen.queryByRole("dialog", { name: "Eltern-Angabe überschreiben?" }),
    ).not.toBeInTheDocument();
  });

  it("rejects an invalid time", async () => {
    const { onSubmitExceptions } = renderEditor();

    fireEvent.click(screen.getAllByRole("button", { name: "Andere Zeit" })[0]!);
    fireEvent.change(screen.getByLabelText("Uhrzeit"), {
      target: { value: "99:99" },
    });
    save();

    expect(
      await screen.findByText("Bitte eine gültige Ankunftszeit eingeben."),
    ).toBeInTheDocument();
    expect(onSubmitExceptions).not.toHaveBeenCalled();
  });
});

describe("schoolDaysBetween", () => {
  it("returns the weekdays of an inclusive range", () => {
    expect(schoolDaysBetween("2026-05-25", "2026-05-27")).toEqual([
      "2026-05-25",
      "2026-05-26",
      "2026-05-27",
    ]);
  });

  it("drops Saturday and Sunday", () => {
    expect(schoolDaysBetween("2026-05-29", "2026-06-01")).toEqual([
      "2026-05-29",
      "2026-06-01",
    ]);
  });

  it("returns nothing for an inverted or incomplete range", () => {
    expect(schoolDaysBetween("2026-05-27", "2026-05-25")).toEqual([]);
    expect(schoolDaysBetween("2026-05-27", "")).toEqual([]);
  });
});
