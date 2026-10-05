import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { StaffScheduleStaff, StaffShift } from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

import {
  DienstplanPersonWeekGrid,
  clickSpan,
  gridWindow,
  minutesToClock,
  sumsByType,
} from "./dienstplan-person-week-grid";

const member: StaffScheduleStaff = {
  id: "7",
  firstName: "Ada",
  lastName: "Lovelace",
};

const WEEK_DAYS = [
  "2026-07-06",
  "2026-07-07",
  "2026-07-08",
  "2026-07-09",
  "2026-07-10",
];

const GT: ShiftType = {
  id: "1",
  name: "GT",
  color: "#83CD2D",
  description: "",
  isActive: true,
};
const RAND: ShiftType = {
  id: "2",
  name: "Randstunde",
  color: "#5080D8",
  description: "",
  isActive: true,
};
const TYPES = [GT, RAND];
const TYPES_BY_ID = new Map(TYPES.map((type) => [type.id, type]));

function shift(overrides: Partial<StaffShift> = {}): StaffShift {
  return {
    id: "1",
    staffId: member.id,
    date: "2026-07-06",
    startTime: "08:00",
    endTime: "12:00",
    breakMinutes: 0,
    shiftTypeId: null,
    shiftTypeName: null,
    shiftTypeColor: null,
    notes: "",
    seriesId: null,
    detached: false,
    cancelled: false,
    changeReason: null,
    originShiftId: null,
    ...overrides,
  };
}

function byDate(shifts: StaffShift[]): Map<string, StaffShift[]> {
  const map = new Map<string, StaffShift[]>();
  for (const s of shifts) map.set(s.date, [...(map.get(s.date) ?? []), s]);
  return map;
}

function renderGrid(
  shifts: StaffShift[],
  props: Partial<Parameters<typeof DienstplanPersonWeekGrid>[0]> = {},
) {
  const onCreate = vi.fn();
  const onEdit = vi.fn();
  render(
    <DienstplanPersonWeekGrid
      member={member}
      shiftsByDate={byDate(shifts)}
      weekDays={WEEK_DAYS}
      todayIso="2026-07-06"
      typesById={TYPES_BY_ID}
      shiftTypes={TYPES}
      onCreate={onCreate}
      onEdit={onEdit}
      {...props}
    />,
  );
  return { onCreate, onEdit };
}

describe("gridWindow", () => {
  it("defaults to 08:00–16:00", () => {
    expect(gridWindow([])).toEqual({ start: 480, end: 960 });
  });

  it("widens to full hours around early and late shifts", () => {
    expect(
      gridWindow([shift({ startTime: "07:15", endTime: "17:10" })]),
    ).toEqual({ start: 420, end: 1080 });
  });
});

describe("clickSpan", () => {
  it("proposes one hour from the slot", () => {
    expect(clickSpan(600, 960, [])).toEqual({ start: 600, end: 660 });
  });

  it("stops before the next shift that takes place", () => {
    expect(
      clickSpan(600, 960, [shift({ startTime: "10:30", endTime: "12:00" })]),
    ).toEqual({ start: 600, end: 630 });
  });

  it("keeps a sub-quarter-hour gap before the next shift", () => {
    expect(
      clickSpan(600, 960, [shift({ startTime: "10:05", endTime: "12:00" })]),
    ).toEqual({ start: 600, end: 605 });
  });

  it("ignores cancelled shifts and never passes the window end", () => {
    expect(
      clickSpan(930, 960, [
        shift({ startTime: "15:45", endTime: "16:00", cancelled: true }),
      ]),
    ).toEqual({ start: 930, end: 960 });
  });
});

describe("minutesToClock", () => {
  it("formats minutes and caps the day end", () => {
    expect(minutesToClock(495)).toBe("08:15");
    expect(minutesToClock(1440)).toBe("23:59");
  });
});

describe("sumsByType", () => {
  it("sums net minutes per type and day, skipping cancelled shifts", () => {
    const shifts = [
      shift({
        id: "a",
        startTime: "08:00",
        endTime: "11:30",
        shiftTypeId: GT.id,
      }),
      shift({
        id: "b",
        startTime: "11:30",
        endTime: "12:15",
        shiftTypeId: RAND.id,
      }),
      shift({
        id: "c",
        date: "2026-07-07",
        startTime: "08:00",
        endTime: "15:00",
        breakMinutes: 30,
        shiftTypeId: GT.id,
      }),
      shift({
        id: "d",
        date: "2026-07-07",
        startTime: "15:00",
        endTime: "16:00",
        shiftTypeId: RAND.id,
        cancelled: true,
      }),
    ];
    const result = sumsByType(WEEK_DAYS, byDate(shifts), TYPES_BY_ID, TYPES);
    expect(result.rows.map((row) => row.label)).toEqual(["GT", "Randstunde"]);
    expect(result.rows[0]?.byDay.get("2026-07-06")).toBe(210);
    expect(result.rows[0]?.byDay.get("2026-07-07")).toBe(390);
    expect(result.rows[1]?.total).toBe(45);
    expect(result.dayTotals.get("2026-07-06")).toBe(255);
    expect(result.weekTotal).toBe(645);
  });
});

describe("DienstplanPersonWeekGrid", () => {
  it("renders shifts as blocks with type label and daily totals", () => {
    renderGrid([
      shift({
        id: "a",
        startTime: "08:00",
        endTime: "11:30",
        shiftTypeId: GT.id,
      }),
      shift({
        id: "b",
        startTime: "11:30",
        endTime: "12:15",
        shiftTypeId: RAND.id,
      }),
    ]);
    expect(
      screen.getByRole("button", { name: "08:00–11:30 GT" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "11:30–12:15 Randstunde" }),
    ).toBeInTheDocument();
    expect(
      screen.getByTestId("person-week-total-2026-07-06"),
    ).toHaveTextContent("4,25 h");
    expect(
      screen.getByTestId("person-week-total-2026-07-07"),
    ).toHaveTextContent("0 h");
    expect(screen.getByText("Diese Woche geplant:")).toHaveTextContent(
      "4,25 h",
    );
  });

  it("shows the Soll from the weekly summary", () => {
    renderGrid([], {
      summary: {
        staffId: member.id,
        weekStart: WEEK_DAYS[0] ?? "",
        plannedMinutes: 0,
        targetMinutes: 1200,
        deltaMinutes: -1200,
        plannedByShiftType: [],
      },
    });
    expect(screen.getByText(/Soll 20 h/)).toBeInTheDocument();
  });

  it("uses the full weekly summary for the planned total", () => {
    renderGrid([shift({ startTime: "08:00", endTime: "10:00" })], {
      summary: {
        staffId: member.id,
        weekStart: WEEK_DAYS[0] ?? "",
        plannedMinutes: 300,
        targetMinutes: 300,
        deltaMinutes: 0,
        plannedByShiftType: [{ shiftTypeId: null, plannedMinutes: 300 }],
      },
    });

    expect(screen.getByText("5 h").parentElement).toHaveTextContent(
      "Diese Woche geplant: 5 h",
    );
  });

  it("keeps horizontal touch panning available on each day column", () => {
    renderGrid([]);

    expect(screen.getByTestId("person-week-day-2026-07-06")).toHaveClass(
      "touch-pan-x",
    );
  });

  it("opens the edit flow when a block is clicked", () => {
    const target = shift({ id: "a", shiftTypeId: GT.id });
    const { onEdit, onCreate } = renderGrid([target]);
    fireEvent.click(screen.getByRole("button", { name: "08:00–12:00 GT" }));
    expect(onEdit).toHaveBeenCalledWith("2026-07-06", target);
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("creates a quarter-hour span by dragging over the grid", () => {
    const { onCreate } = renderGrid([]);
    const column = screen.getByTestId("person-week-day-2026-07-08");
    // jsdom: getBoundingClientRect().top is 0, so clientY is the offset.
    // Slot 4 = 09:00, slot 6 = 09:30 → 09:00–09:45.
    fireEvent.pointerDown(column, { button: 0, pointerId: 1, clientY: 4 * 16 });
    fireEvent.pointerMove(column, { pointerId: 1, clientY: 6 * 16 + 3 });
    fireEvent.pointerUp(column, { pointerId: 1, clientY: 6 * 16 + 3 });
    expect(onCreate).toHaveBeenCalledWith("2026-07-08", "09:00", "09:45");
  });

  it("proposes one hour for a single click on a free quarter hour", () => {
    const { onCreate } = renderGrid([]);
    const column = screen.getByTestId("person-week-day-2026-07-09");
    fireEvent.pointerDown(column, { button: 0, pointerId: 1, clientY: 9 * 16 });
    fireEvent.pointerUp(column, { pointerId: 1, clientY: 9 * 16 });
    expect(onCreate).toHaveBeenCalledWith("2026-07-09", "10:15", "11:15");
  });

  it("asks before creating a shift on a closing day", () => {
    const { onCreate } = renderGrid([], {
      closingDays: new Map([["2026-07-10", "Brückentag"]]),
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Schicht anlegen, Fr 10.07." }),
    );
    expect(onCreate).not.toHaveBeenCalled();
    const dialog = screen.getByRole("dialog");
    fireEvent.click(
      within(dialog)
        .getAllByRole("button")
        .find((button) =>
          /anlegen|trotzdem/i.test(button.textContent ?? ""),
        ) as HTMLElement,
    );
    expect(onCreate).toHaveBeenCalledWith("2026-07-10", "08:00", "16:00");
  });

  it("keeps the header add button usable without a pointer", () => {
    const { onCreate } = renderGrid([]);
    fireEvent.click(
      screen.getByRole("button", { name: "Schicht anlegen, Mo 06.07." }),
    );
    expect(onCreate).toHaveBeenCalledWith("2026-07-06", "08:00", "16:00");
  });
});

// Leseansicht (#3821): der eigene Dienstplan der Mitarbeitenden nutzt dasselbe
// Raster ohne onCreate/onEdit.
describe("DienstplanPersonWeekGrid read-only", () => {
  function renderReadOnly(
    shifts: StaffShift[],
    weekDays: readonly string[] = WEEK_DAYS,
  ) {
    render(
      <DienstplanPersonWeekGrid
        shiftsByDate={byDate(shifts)}
        weekDays={weekDays}
        todayIso="2026-07-06"
        typesById={TYPES_BY_ID}
        shiftTypes={TYPES}
      />,
    );
  }

  it("shows blocks without any create or edit affordance", () => {
    renderReadOnly([shift({ id: "a", shiftTypeId: GT.id })]);

    expect(screen.queryAllByRole("button")).toHaveLength(0);
    expect(
      screen.getByRole("group", { name: "08:00–12:00 GT" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Ziehen Sie über die Viertelstunden/),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        /Nur zur Information\. Ihre Schichten plant die Leitung/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByTestId("person-week-day-2026-07-06")).not.toHaveClass(
      "cursor-cell",
    );
    expect(
      screen.getByTestId("person-week-total-2026-07-06"),
    ).toHaveTextContent("4 h");
  });

  it("ignores pointer drags on the grid", () => {
    renderReadOnly([]);
    const column = screen.getByTestId("person-week-day-2026-07-08");
    fireEvent.pointerDown(column, { button: 0, pointerId: 1, clientY: 4 * 16 });
    fireEvent.pointerMove(column, { pointerId: 1, clientY: 6 * 16 + 3 });

    expect(screen.queryByText("09:00–09:45")).not.toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("labels weekend columns when the week includes them", () => {
    renderReadOnly(
      [shift({ id: "a", date: "2026-07-11", shiftTypeId: GT.id })],
      [...WEEK_DAYS, "2026-07-11"],
    );

    expect(screen.getByText("Sa 11.07.")).toBeInTheDocument();
    expect(
      screen.getByTestId("person-week-total-2026-07-11"),
    ).toHaveTextContent("4 h");
  });
});
