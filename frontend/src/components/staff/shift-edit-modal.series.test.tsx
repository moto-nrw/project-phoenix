import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { setTestClock } from "~/test/clock";

import { ShiftEditModal } from "./shift-edit-modal";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import type { StaffShift } from "~/lib/shift-helpers";

const createSeries = vi.fn();
const splitSeries = vi.fn();
const endSeries = vi.fn();
const getSeries = vi.fn();
const updateShift = vi.fn();
const deleteShift = vi.fn();
const listPeriods = vi.fn();
const mockDatePickerValue = vi.hoisted(() => ({
  value: new Date(2026, 11, 31),
}));

vi.mock("~/lib/shift-api", () => ({
  ShiftApiError: class ShiftApiError extends Error {
    readonly status: number;
    readonly detail: string;

    constructor(status: number, detail: string) {
      super(detail);
      this.status = status;
      this.detail = detail;
    }
  },
  staffShiftService: {
    createShift: vi.fn(),
    updateShift: (...args: unknown[]) => updateShift(...args) as unknown,
    deleteShift: (...args: unknown[]) => deleteShift(...args) as unknown,
    applyCancellation: vi.fn(),
  },
  staffShiftSeriesService: {
    createSeries: (...args: unknown[]) => createSeries(...args) as unknown,
    splitSeries: (...args: unknown[]) => splitSeries(...args) as unknown,
    endSeries: (...args: unknown[]) => endSeries(...args) as unknown,
    // Opening a series shift now also loads the rule behind it (#2028); the
    // mock must cover the whole module surface the modal touches.
    getSeries: (...args: unknown[]) => getSeries(...args) as unknown,
  },
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: {
    list: (...args: unknown[]) => listPeriods(...args) as unknown,
  },
}));

// The kit DatePicker opens a react-day-picker calendar overlay; drive its
// onChange directly instead (same pattern as planned-status-days-modal.test).
vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: ({ onChange }: { onChange: (date: Date | null) => void }) => (
    <button type="button" onClick={() => onChange(mockDatePickerValue.value)}>
      DatePicker Auswahl
    </button>
  ),
}));

const halbjahr: CalendarPeriod = {
  id: "5",
  name: "1. Halbjahr 2026/27",
  periodType: "semester",
  startDate: "2026-08-01",
  endDate: "2027-01-31",
  weekCycleLength: 2,
  weekCycleAnchor: "2026-08-03",
  isActive: true,
} as CalendarPeriod;

const ganzjahrOhneZyklus: CalendarPeriod = {
  id: "6",
  name: "Ganzjahr ohne Zyklus",
  periodType: "school_year",
  startDate: "2026-08-01",
  endDate: "2027-07-31",
  weekCycleLength: 1,
  weekCycleAnchor: null,
  isActive: true,
} as CalendarPeriod;

const halbjahrVersetzt: CalendarPeriod = {
  ...halbjahr,
  id: "7",
  name: "Versetzter A/B-Zyklus",
  weekCycleAnchor: "2026-08-10",
} as CalendarPeriod;

const seriesShift: StaffShift = {
  id: "9",
  staffId: "7",
  date: "2026-09-07",
  startTime: "08:00",
  endTime: "12:00",
  breakMinutes: 0,
  shiftTypeId: null,
  shiftTypeName: null,
  shiftTypeColor: null,
  notes: "",
  seriesId: "5",
  detached: false,
  cancelled: false,
  changeReason: null,
  originShiftId: null,
};

function renderModal(props: Partial<Parameters<typeof ShiftEditModal>[0]>) {
  return render(
    <ShiftEditModal
      isOpen={true}
      mode="create"
      staffId="7"
      staffName="Ada Lovelace"
      date="2026-09-07"
      shift={null}
      shiftTypes={[]}
      onClose={vi.fn()}
      onSaved={vi.fn()}
      {...props}
    />,
  );
}

const seriesRule = {
  id: "5",
  staffId: "7",
  weekdays: [1, 3],
  startTime: "08:00",
  endTime: "12:00",
  breakMinutes: 0,
  shiftTypeId: null,
  calendarPeriodId: "5",
  weekPattern: 0,
  validFrom: "2026-09-01",
  validUntil: null,
  includeSchoolBreaks: false,
};

beforeEach(() => {
  vi.clearAllMocks();
  mockDatePickerValue.value = new Date(2026, 11, 31);
  listPeriods.mockResolvedValue([halbjahr]);
  getSeries.mockResolvedValue(seriesRule);
});

describe("ShiftEditModal series creation", () => {
  it("shows the repeat section with the clicked weekday preselected", async () => {
    renderModal({});

    fireEvent.click(screen.getByLabelText("Als Serie wiederholen"));

    // 2026-09-07 is a Monday.
    await waitFor(() => {
      expect(screen.getByLabelText("Mo")).toBeChecked();
    });
    expect(screen.getByLabelText("Di")).not.toBeChecked();
    expect(listPeriods).toHaveBeenCalledTimes(1);
  });

  it("creates a weekly series over the selected period", async () => {
    createSeries.mockResolvedValue({
      seriesId: "5",
      created: 20,
      skippedDates: [],
    });
    const onSaved = vi.fn();
    const onClose = vi.fn();
    renderModal({ onSaved, onClose });

    fireEvent.click(screen.getByLabelText("Als Serie wiederholen"));
    // The period select defaults asynchronously once the periods loaded.
    await screen.findByText("1. Halbjahr 2026/27");
    fireEvent.click(screen.getByLabelText("Mi"));
    expect(screen.getByLabelText("Mi")).toBeChecked();
    expect(screen.getByLabelText("Mo")).toBeChecked();

    fireEvent.click(screen.getByRole("button", { name: "Serie anlegen" }));

    await waitFor(() => {
      expect(createSeries).toHaveBeenCalledWith({
        staffId: "7",
        weekdays: [1, 3],
        startTime: "08:00",
        endTime: "16:00",
        breakMinutes: 30,
        shiftTypeId: null,
        calendarPeriodId: "5",
        weekPattern: 0,
        validFrom: "2026-09-07",
        validUntil: null,
        includeSchoolBreaks: false,
      });
    });
    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  // #3820: Ferien und Schließtage lässt die Serie standardmäßig aus; der
  // Haken ersetzt die frühere Schließtag-Rückfrage (#2032).
  it("plans Ferien and closing days only when the box is ticked", async () => {
    createSeries.mockResolvedValue({
      seriesId: "5",
      created: 20,
      skippedDates: [],
      skippedNonWorkingDays: 0,
    });
    renderModal({});

    fireEvent.click(screen.getByLabelText("Als Serie wiederholen"));
    await screen.findByText("1. Halbjahr 2026/27");
    const box = screen.getByLabelText(
      /Auch in den Ferien und an Schließtagen planen/,
    );
    expect(box).not.toBeChecked();
    fireEvent.click(box);
    fireEvent.click(screen.getByRole("button", { name: "Serie anlegen" }));

    await waitFor(() => {
      expect(createSeries).toHaveBeenCalledWith(
        expect.objectContaining({ includeSchoolBreaks: true }),
      );
    });
    expect(
      screen.queryByRole("dialog", { name: "An einem Schließtag planen?" }),
    ).not.toBeInTheDocument();
  });

  it("says how many days stayed free in the Ferien, on closing days or holidays", async () => {
    createSeries.mockResolvedValue({
      seriesId: "5",
      created: 15,
      skippedDates: [],
      skippedNonWorkingDays: 5,
    });
    const onClose = vi.fn();
    renderModal({ onClose });

    fireEvent.click(screen.getByLabelText("Als Serie wiederholen"));
    await screen.findByText("1. Halbjahr 2026/27");
    fireEvent.click(screen.getByRole("button", { name: "Serie anlegen" }));

    expect(
      await screen.findByText(
        "5 Tage liegen in den Ferien, an Schließtagen oder Feiertagen und bleiben frei.",
      ),
    ).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("drops a stale A/B pattern when switching to a period without a week cycle", async () => {
    createSeries.mockResolvedValue({
      seriesId: "5",
      created: 20,
      skippedDates: [],
    });
    listPeriods.mockResolvedValue([halbjahr, ganzjahrOhneZyklus]);
    renderModal({});

    fireEvent.click(screen.getByLabelText("Als Serie wiederholen"));
    await screen.findByText("1. Halbjahr 2026/27");
    // Enable A/B on the cycle period, then switch to a period without a
    // cycle: the hidden biweekly flag must not leak into the payload.
    fireEvent.click(screen.getByRole("radio", { name: "Alle 2 Wochen" }));
    fireEvent.click(screen.getByLabelText("Kalenderzeitraum"));
    fireEvent.click(
      screen.getByRole("option", { name: "Ganzjahr ohne Zyklus" }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Serie anlegen" }));

    await waitFor(() => {
      expect(createSeries).toHaveBeenCalledWith(
        expect.objectContaining({ calendarPeriodId: "6", weekPattern: 0 }),
      );
    });
  });

  it("keeps an explicitly reselected A week when the period default changes", async () => {
    listPeriods.mockResolvedValue([halbjahr, halbjahrVersetzt]);
    renderModal({ date: "2026-09-14" });

    fireEvent.click(screen.getByLabelText("Als Serie wiederholen"));
    await screen.findByText("1. Halbjahr 2026/27");
    fireEvent.click(screen.getByRole("radio", { name: "Alle 2 Wochen" }));

    const weekA = screen.getByRole("radio", { name: "Woche A" });
    await waitFor(() => expect(weekA).toBeChecked());
    // Native radios do not emit change when the checked option is clicked. The
    // click still represents an explicit choice and must stop future defaults.
    fireEvent.click(weekA);
    fireEvent.click(screen.getByLabelText("Kalenderzeitraum"));
    fireEvent.click(
      screen.getByRole("option", { name: "Versetzter A/B-Zyklus" }),
    );

    expect(weekA).toBeChecked();
    expect(screen.getByRole("radio", { name: "Woche B" })).not.toBeChecked();
  });

  it("sends the inclusive 'Gültig bis' date as the exclusive valid_until", async () => {
    createSeries.mockResolvedValue({
      seriesId: "5",
      created: 20,
      skippedDates: [],
    });
    renderModal({});

    fireEvent.click(screen.getByLabelText("Als Serie wiederholen"));
    await screen.findByText("1. Halbjahr 2026/27");
    // The user picks the LAST day that should still have a shift (year
    // boundary on purpose; the DatePicker mock selects 2026-12-31); the
    // API's valid_until is exclusive → +1 day.
    fireEvent.click(screen.getByRole("button", { name: "DatePicker Auswahl" }));
    fireEvent.click(screen.getByRole("button", { name: "Serie anlegen" }));

    await waitFor(() => {
      expect(createSeries).toHaveBeenCalledWith(
        expect.objectContaining({ validUntil: "2027-01-01" }),
      );
    });
  });

  it("keeps the modal open and lists skipped days after a collision", async () => {
    createSeries.mockResolvedValue({
      seriesId: "5",
      created: 19,
      skippedDates: ["2026-09-14"],
    });
    const onClose = vi.fn();
    renderModal({ onClose });

    fireEvent.click(screen.getByLabelText("Als Serie wiederholen"));
    await screen.findByText("1. Halbjahr 2026/27");
    fireEvent.click(screen.getByRole("button", { name: "Serie anlegen" }));

    await waitFor(() => {
      expect(
        screen.getByText(/wurden übersprungen: 14\.09\.2026/),
      ).toBeInTheDocument();
    });
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("ShiftEditModal series scopes", () => {
  it("asks for the edit scope and splits the series for 'Ab jetzt dauerhaft'", async () => {
    splitSeries.mockResolvedValue({
      seriesId: "6",
      created: 10,
      skippedDates: [],
    });
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );
    expect(
      screen.getByText(
        "Diese Schicht ist Teil einer Serie. Wofür soll die Änderung gelten?",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => {
      expect(splitSeries).toHaveBeenCalledWith("5", {
        effectiveDate: "2026-09-07",
        occurrenceShiftId: "9",
        startTime: "08:00",
        endTime: "12:00",
        breakMinutes: 0,
        shiftTypeId: null,
      });
    });
    expect(updateShift).not.toHaveBeenCalled();
  });

  it("asks for the delete scope and deletes only this occurrence for 'Nur diese Woche'", async () => {
    deleteShift.mockResolvedValue(undefined);
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(screen.getByRole("button", { name: "Schicht löschen" }));
    // Scope-Slot der ConfirmDeleteModal (#3110): die Wahl ist der erste
    // Schritt, ohne sie bleibt Löschen gesperrt.
    expect(screen.getByText("Was soll gelöscht werden?")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Löschen" })).toBeDisabled();

    fireEvent.click(screen.getByRole("radio", { name: /Nur diese Woche/ }));
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));

    await waitFor(() => {
      expect(deleteShift).toHaveBeenCalledWith("9");
    });
    expect(endSeries).not.toHaveBeenCalled();
  });

  it("ends the series from the shift date for 'Ab jetzt dauerhaft' delete", async () => {
    endSeries.mockResolvedValue(undefined);
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(screen.getByRole("button", { name: "Schicht löschen" }));
    fireEvent.click(screen.getByRole("radio", { name: /Ab jetzt dauerhaft/ }));
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));

    await waitFor(() => {
      expect(endSeries).toHaveBeenCalledWith("5", "2026-09-07");
    });
    expect(deleteShift).not.toHaveBeenCalled();
  });

  it("ends a moved occurrence's series from its original date", async () => {
    endSeries.mockResolvedValue(undefined);
    renderModal({
      mode: "edit",
      shift: {
        ...seriesShift,
        date: "2026-09-08",
        detached: true,
        seriesOccurrenceDate: "2026-09-07",
      },
    });

    fireEvent.click(screen.getByRole("button", { name: "Schicht löschen" }));
    fireEvent.click(screen.getByRole("radio", { name: /Ab jetzt dauerhaft/ }));
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));

    await waitFor(() => {
      expect(endSeries).toHaveBeenCalledWith("5", "2026-09-07");
    });
  });
});

// #2028: editing the series itself — its weekdays, rhythm, window and
// validity — from the opened occurrence onwards.
describe("ShiftEditModal series rule editing", () => {
  it("states the series rule and opens the editor from the shift panel", async () => {
    renderModal({ mode: "edit", shift: seriesShift });

    // The rule is visible without saving first: which days, which window,
    // how long it runs.
    expect(
      await screen.findByText(
        "Mo, Mi \u00b7 08:00\u201312:00 \u00b7 bis Ende des Kalenderzeitraums",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Serie bearbeiten" }));

    expect(await screen.findByText("Serie bearbeiten")).toBeInTheDocument();
    // Weekdays come from the rule, not from the clicked occurrence.
    expect(screen.getByLabelText("Mo")).toBeChecked();
    expect(screen.getByLabelText("Mi")).toBeChecked();
    expect(screen.getByLabelText("Di")).not.toBeChecked();
  });

  it("applies changed weekdays and window from the opened date onwards", async () => {
    splitSeries.mockResolvedValue({
      seriesId: "6",
      created: 12,
      skippedDates: [],
    });
    const onSaved = vi.fn();
    const onClose = vi.fn();
    renderModal({ mode: "edit", shift: seriesShift, onSaved, onClose });

    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );
    fireEvent.click(await screen.findByLabelText("Fr"));
    fireEvent.change(screen.getByLabelText("Ende"), {
      target: { value: "14:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Serie speichern" }));

    await waitFor(() => {
      expect(splitSeries).toHaveBeenCalledWith("5", {
        effectiveDate: "2026-09-07",
        occurrenceShiftId: "9",
        startTime: "08:00",
        endTime: "14:00",
        breakMinutes: 0,
        shiftTypeId: null,
        weekdays: [1, 3, 5],
        weekPattern: 0,
        validUntil: null,
        includeSchoolBreaks: false,
      });
    });
    expect(updateShift).not.toHaveBeenCalled();
    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it("splits a moved occurrence from its original series date", async () => {
    getSeries.mockResolvedValue({ ...seriesRule, weekPattern: 1 });
    splitSeries.mockResolvedValue({
      seriesId: "6",
      created: 12,
      skippedDates: [],
    });
    renderModal({
      mode: "edit",
      shift: {
        ...seriesShift,
        date: "2026-09-08",
        detached: true,
        seriesOccurrenceDate: "2026-09-14",
      },
    });

    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );
    expect(
      screen.getByText(/Die Änderungen gelten ab 14\.09\.2026/),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue("14.09.2026")).toBeDisabled();
    await waitFor(() => {
      expect(
        screen.getByText("Die Woche vom 14.09.2026 ist Woche A."),
      ).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: "Serie speichern" }));

    await waitFor(() => {
      expect(splitSeries).toHaveBeenCalledWith(
        "5",
        expect.objectContaining({ effectiveDate: "2026-09-14" }),
      );
    });
  });

  it("keeps the stored A/B pattern while its calendar period is unavailable", async () => {
    const alternatingRule = { ...seriesRule, weekPattern: 2 as const };
    listPeriods.mockRejectedValue(
      new Error("Kalenderzeiträume nicht verfügbar"),
    );
    getSeries.mockResolvedValue(alternatingRule);
    splitSeries.mockResolvedValue({
      seriesId: "6",
      created: 4,
      skippedDates: [],
    });
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );
    fireEvent.click(await screen.findByLabelText("Fr"));
    fireEvent.click(screen.getByRole("button", { name: "Serie speichern" }));

    await waitFor(() => {
      expect(splitSeries).toHaveBeenCalledWith(
        "5",
        expect.objectContaining({ weekPattern: 2 }),
      );
    });
  });

  it("restores the occurrence draft after returning from series editing", async () => {
    updateShift.mockResolvedValue(undefined);
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.change(screen.getByLabelText("Ende"), {
      target: { value: "13:00" },
    });
    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );
    expect(screen.getByLabelText("Ende")).toHaveValue("12:00");

    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(screen.getByLabelText("Ende")).toHaveValue("13:00");

    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );
    fireEvent.click(screen.getByRole("button", { name: /Nur diese Woche/ }));

    await waitFor(() => {
      expect(updateShift).toHaveBeenCalledWith(
        "9",
        expect.objectContaining({ endTime: "13:00" }),
      );
    });
  });

  it("sends the inclusive 'G\u00fcltig bis' as the exclusive valid_until", async () => {
    splitSeries.mockResolvedValue({
      seriesId: "6",
      created: 4,
      skippedDates: [],
    });
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );
    // The stubbed DatePicker reports 2026-12-31.
    fireEvent.click(await screen.findByText("DatePicker Auswahl"));
    fireEvent.click(screen.getByRole("button", { name: "Serie speichern" }));

    await waitFor(() => {
      expect(splitSeries).toHaveBeenCalledWith(
        "5",
        expect.objectContaining({ validUntil: "2027-01-01" }),
      );
    });
  });

  it("keeps the occurrence scopes untouched", async () => {
    renderModal({ mode: "edit", shift: seriesShift });
    await screen.findByRole("button", { name: "Serie bearbeiten" });

    fireEvent.click(
      screen.getByRole("button", { name: "\u00c4nderungen speichern" }),
    );

    // Only the two pre-existing occurrence scopes (#1889) are offered here;
    // the series itself is edited through its own editor.
    expect(
      screen.getByRole("button", { name: /Nur diese Woche/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Alle Termine der Serie/ }),
    ).not.toBeInTheDocument();
  });

  // #3820: Der Haken zeigt die gespeicherte Wahl und geht beim Speichern mit.
  it("shows the stored school-break choice and saves a changed one", async () => {
    getSeries.mockResolvedValue({ ...seriesRule, includeSchoolBreaks: true });
    splitSeries.mockResolvedValue({
      seriesId: "6",
      created: 12,
      skippedDates: [],
      skippedNonWorkingDays: 0,
    });
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );
    const box = screen.getByLabelText(
      /Auch in den Ferien und an Schließtagen planen/,
    );
    expect(box).toBeChecked();
    fireEvent.click(box);
    fireEvent.click(screen.getByRole("button", { name: "Serie speichern" }));

    await waitFor(() => {
      expect(splitSeries).toHaveBeenCalledWith(
        "5",
        expect.objectContaining({
          effectiveDate: "2026-09-07",
          includeSchoolBreaks: false,
        }),
      );
    });
  });
});

// A segment whose last day has arrived used to be uneditable: the re-plan
// starts tomorrow at the earliest, so the save came back as a generic 400
// pointing at Beginn/Ende/Pause. The editor now says what to do instead (#2028).
describe("ShiftEditModal series rule editing at the end of a segment", () => {
  beforeEach(() => {
    // Fake ONLY Date: testing-library's findBy* polls on real timers.
    // 12:00 Berlin on the opened occurrence's own day.
    setTestClock(new Date("2026-09-07T10:00:00Z"));
  });

  it("warns instead of saving when nothing is left to change", async () => {
    // Exclusive valid_until 2026-09-08 = last shift day is today.
    getSeries.mockResolvedValue({ ...seriesRule, validUntil: "2026-09-08" });
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );

    // The effective date is the clamped one, not the opened occurrence.
    expect(
      await screen.findByText(/Die Änderungen gelten ab 08\.09\.2026/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Setzen Sie „Gültig bis" auf ein späteres Datum/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Serie speichern" }));
    expect(splitSeries).not.toHaveBeenCalled();
  });

  it("saves once the end date is extended", async () => {
    getSeries.mockResolvedValue({ ...seriesRule, validUntil: "2026-09-08" });
    splitSeries.mockResolvedValue({
      seriesId: "6",
      created: 12,
      skippedDates: [],
    });
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );
    // The stubbed DatePicker picks 2026-12-31.
    fireEvent.click(await screen.findByText("DatePicker Auswahl"));
    fireEvent.click(screen.getByRole("button", { name: "Serie speichern" }));

    await waitFor(() => {
      expect(splitSeries).toHaveBeenCalledWith("5", {
        effectiveDate: "2026-09-07",
        occurrenceShiftId: "9",
        startTime: "08:00",
        endTime: "12:00",
        breakMinutes: 0,
        shiftTypeId: null,
        weekdays: [1, 3],
        weekPattern: 0,
        // Picker is inclusive, the API's valid_until exclusive.
        validUntil: "2027-01-01",
        includeSchoolBreaks: false,
      });
    });
  });

  it("warns instead of extending without a matching recurrence date", async () => {
    // The change starts on Tuesday. Extending a Mo/Mi series only through
    // Tuesday creates no future occurrence.
    mockDatePickerValue.value = new Date(2026, 8, 8);
    getSeries.mockResolvedValue({ ...seriesRule, validUntil: "2026-09-08" });
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );
    fireEvent.click(await screen.findByText("DatePicker Auswahl"));

    expect(
      await screen.findByText(/Für die gewählten Wochentage/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Serie speichern" }));
    expect(splitSeries).not.toHaveBeenCalled();
  });
});
