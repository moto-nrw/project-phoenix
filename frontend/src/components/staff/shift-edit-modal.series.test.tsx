import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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
    <button type="button" onClick={() => onChange(new Date(2026, 11, 31))}>
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
};

beforeEach(() => {
  vi.clearAllMocks();
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
      });
    });
    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
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
    expect(
      screen.getByText(
        "Diese Schicht ist Teil einer Serie. Was soll gelöscht werden?",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Nur diese Woche/ }));

    await waitFor(() => {
      expect(deleteShift).toHaveBeenCalledWith("9");
    });
    expect(endSeries).not.toHaveBeenCalled();
  });

  it("ends the series from the shift date for 'Ab jetzt dauerhaft' delete", async () => {
    endSeries.mockResolvedValue(undefined);
    renderModal({ mode: "edit", shift: seriesShift });

    fireEvent.click(screen.getByRole("button", { name: "Schicht löschen" }));
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => {
      expect(endSeries).toHaveBeenCalledWith("5", "2026-09-07");
    });
    expect(deleteShift).not.toHaveBeenCalled();
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
        startTime: "08:00",
        endTime: "14:00",
        breakMinutes: 0,
        shiftTypeId: null,
        weekdays: [1, 3, 5],
        weekPattern: 0,
        validUntil: null,
      });
    });
    expect(updateShift).not.toHaveBeenCalled();
    expect(onSaved).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it("splits a moved occurrence from its original series date", async () => {
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
        seriesOccurrenceDate: "2026-09-07",
      },
    });

    fireEvent.click(
      await screen.findByRole("button", { name: "Serie bearbeiten" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Serie speichern" }));

    await waitFor(() => {
      expect(splitSeries).toHaveBeenCalledWith(
        "5",
        expect.objectContaining({ effectiveDate: "2026-09-07" }),
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
});
