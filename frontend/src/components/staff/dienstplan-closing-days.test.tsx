import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import type { ClosingDayRange } from "~/lib/closing-day-helpers";
import type { StaffScheduleStaff, StaffShift } from "~/lib/shift-helpers";

const mocks = vi.hoisted(() => ({ push: vi.fn(), mutateMatching: vi.fn() }));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.push }),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutateMatching: () => mocks.mutateMatching,
}));

vi.mock("~/lib/hooks/use-closing-days", () => ({
  useClosingDays: () => new Map(),
}));

import { DienstplanResourceGrid } from "./dienstplan-resource-grid";
import { ShiftMoveDialog } from "./shift-move-dialog";

/**
 * #2032: Schließtage sind im Dienstplan erkennbar, und das Anlegen bzw.
 * Verschieben einer Schicht auf einen Schließtag fragt einmal nach. Blockiert
 * wird nichts — nach der Bestätigung läuft der bisherige Ablauf weiter.
 */

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
const TODAY = "2026-07-06";
const CLOSING = new Map([["2026-07-08", "Pädagogischer Tag"]]);

function baseShift(overrides: Partial<StaffShift> = {}): StaffShift {
  return {
    id: "1",
    staffId: member.id,
    date: "2026-07-08",
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

function renderGrid(options: {
  closingDays?: ReadonlyMap<string, string>;
  closingDaysLoading?: boolean;
  shiftsByStaff?: Map<string, Map<string, StaffShift[]>>;
}) {
  const onCellClick = vi.fn();
  render(
    <DienstplanResourceGrid
      staff={[member]}
      shiftsByStaff={options.shiftsByStaff ?? new Map()}
      assignmentsByStaff={new Map()}
      summaryByStaff={new Map()}
      weekDays={WEEK_DAYS}
      todayIso={TODAY}
      closingDays={options.closingDays}
      closingDaysLoading={options.closingDaysLoading}
      typesById={new Map()}
      shiftTypes={[]}
      onCellClick={onCellClick}
    />,
  );
  return { onCellClick };
}

describe("DienstplanResourceGrid closing days (#2032)", () => {
  beforeEach(() => vi.clearAllMocks());

  it("marks the closing day column head with its reason", () => {
    renderGrid({ closingDays: CLOSING });

    const header = screen.getByRole("columnheader", { name: /Mi 08\.07\./ });
    expect(
      within(header).getByTitle("Schließtag: Pädagogischer Tag"),
    ).toBeInTheDocument();
  });

  it("asks before creating a shift on a closing day and creates after confirming", () => {
    const { onCellClick } = renderGrid({ closingDays: CLOSING });

    fireEvent.click(
      screen.getByRole("button", {
        name: /Schicht anlegen, Ada Lovelace, Mi 08\.07\./,
      }),
    );
    expect(onCellClick).not.toHaveBeenCalled();
    expect(screen.getByText("An einem Schließtag planen?")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Trotzdem planen" }));
    expect(onCellClick).toHaveBeenCalledWith(member, "2026-07-08", null);
  });

  it("keeps empty cells inactive until closing days have loaded", () => {
    const { onCellClick } = renderGrid({
      closingDays: new Map(),
      closingDaysLoading: true,
    });

    expect(
      screen.queryByRole("button", {
        name: /Schicht anlegen, Ada Lovelace, Mo 06\.07\./,
      }),
    ).not.toBeInTheDocument();
    expect(onCellClick).not.toHaveBeenCalled();
  });

  it("does not create the shift when the warning is dismissed", () => {
    const { onCellClick } = renderGrid({ closingDays: CLOSING });

    fireEvent.click(
      screen.getByRole("button", {
        name: /Schicht anlegen, Ada Lovelace, Mi 08\.07\./,
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(onCellClick).not.toHaveBeenCalled();
    expect(
      screen.queryByText("An einem Schließtag planen?"),
    ).not.toBeInTheDocument();
  });

  it("opens an existing shift on a closing day without asking", () => {
    const { onCellClick } = renderGrid({
      closingDays: CLOSING,
      shiftsByStaff: new Map([
        [member.id, new Map([["2026-07-08", [baseShift()]]])],
      ]),
    });

    fireEvent.click(
      screen.getByRole("button", { name: "08:00–12:00 Schicht" }),
    );

    expect(
      screen.queryByText("An einem Schließtag planen?"),
    ).not.toBeInTheDocument();
    expect(onCellClick).toHaveBeenCalledWith(
      member,
      "2026-07-08",
      expect.objectContaining({ id: "1" }),
    );
  });

  it("creates without asking on a day that is not a closing day", () => {
    const { onCellClick } = renderGrid({ closingDays: CLOSING });

    fireEvent.click(
      screen.getByRole("button", {
        name: /Schicht anlegen, Ada Lovelace, Mo 06\.07\./,
      }),
    );

    expect(
      screen.queryByText("An einem Schließtag planen?"),
    ).not.toBeInTheDocument();
    expect(onCellClick).toHaveBeenCalledWith(member, "2026-07-06", null);
  });
});

describe("ShiftMoveDialog closing days (#2032)", () => {
  const ranges: ClosingDayRange[] = [
    {
      startDate: "2026-07-08",
      endDate: "2026-07-08",
      reason: "Pädagogischer Tag",
    },
  ];

  beforeEach(() => vi.clearAllMocks());

  it("flags a target day that falls on a closing day, in the form and in the confirmation", () => {
    render(
      <ShiftMoveDialog
        isOpen
        shift={baseShift({ date: "2026-07-08" })}
        sourceMember={member}
        staff={[member]}
        shiftTypes={[]}
        closingDayRanges={ranges}
        onClose={vi.fn()}
        onDataChanged={vi.fn()}
      />,
    );

    expect(
      screen.getByTitle("Schließtag: Pädagogischer Tag"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Verschieben" }));

    expect(
      screen.getByText(/Am Zieltag ist ein Schließtag hinterlegt/),
    ).toBeInTheDocument();
  });

  it("stays silent when the target day is no closing day", () => {
    render(
      <ShiftMoveDialog
        isOpen
        shift={baseShift({ date: "2026-07-07" })}
        sourceMember={member}
        staff={[member]}
        shiftTypes={[]}
        closingDayRanges={ranges}
        onClose={vi.fn()}
        onDataChanged={vi.fn()}
      />,
    );

    expect(screen.queryByText(/Schließtag/)).not.toBeInTheDocument();
  });
});
