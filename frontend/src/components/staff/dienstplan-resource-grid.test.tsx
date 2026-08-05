import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type {
  StaffScheduleAssignment,
  StaffScheduleStaff,
  StaffShift,
  StaffWeeklySummary,
} from "~/lib/shift-helpers";
import type { ShiftType } from "~/lib/shift-type-helpers";

import { DienstplanResourceGrid } from "./dienstplan-resource-grid";

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  useMediaQuery: vi.fn(() => false),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.push }),
}));

vi.mock("~/lib/hooks/use-media-query", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/hooks/use-media-query")>()),
  useMediaQuery: mocks.useMediaQuery,
}));

// Der Verschieben-Dialog fragt die Schließtage des Zieltags ab (#2032); der
// echte Hook braucht eine NextAuth-Session, die dieses Raster nicht stellt.
vi.mock("~/lib/hooks/use-closing-days", () => ({
  useClosingDaysState: () => ({
    closingDays: new Map<string, string>(),
    closingDayRanges: [],
    isLoading: false,
  }),
}));

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

function baseShift(overrides: Partial<StaffShift> = {}): StaffShift {
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

function absentAssignment(
  overrides: Partial<StaffScheduleAssignment> = {},
): StaffScheduleAssignment {
  return {
    instanceId: "42",
    staffId: member.id,
    date: "2026-07-06",
    startTime: "12:00",
    endTime: "14:00",
    activityTitle: "Mensa",
    roomId: "3",
    roomName: "Speisesaal",
    status: "planned",
    isAbsent: true,
    isSubstitute: false,
    absenceReason: "krank",
    coverageStatus: "not_applicable",
    coverageReason: "absent",
    uncoveredIntervals: [],
    ...overrides,
  };
}

// Read-only Betreuungsplan assignment — mirrors the default fixture from the
// retired per-cell week grid's test suite (coverage carried forward 1:1).
function assignment(
  overrides: Partial<StaffScheduleAssignment> = {},
): StaffScheduleAssignment {
  return {
    instanceId: "42",
    staffId: member.id,
    date: "2026-07-06",
    startTime: "12:00",
    endTime: "14:00",
    activityTitle: "Mensa",
    roomId: "3",
    roomName: "Speisesaal",
    status: "planned",
    isAbsent: false,
    isSubstitute: false,
    absenceReason: null,
    coverageStatus: "uncovered",
    coverageReason: null,
    uncoveredIntervals: [{ startTime: "12:30", endTime: "14:00" }],
    ...overrides,
  };
}

interface RenderOverrides {
  staff?: readonly StaffScheduleStaff[];
  shiftsByStaff?: Map<string, Map<string, StaffShift[]>>;
  assignmentsByStaff?: Map<string, Map<string, StaffScheduleAssignment[]>>;
  summaryByStaff?: Map<string, StaffWeeklySummary>;
  typesById?: Map<string, ShiftType>;
  shiftTypes?: readonly ShiftType[];
  reducedPath?: boolean;
  currentStaffId?: string | null;
  onSickReport?: (staff: StaffScheduleStaff) => void;
  weekDays?: readonly string[];
  todayIso?: string;
}

function renderGrid(overrides: RenderOverrides = {}) {
  const onCellClick = vi.fn();
  const renderResult = render(
    <DienstplanResourceGrid
      staff={overrides.staff ?? [member]}
      shiftsByStaff={overrides.shiftsByStaff ?? new Map()}
      assignmentsByStaff={overrides.assignmentsByStaff ?? new Map()}
      summaryByStaff={overrides.summaryByStaff ?? new Map()}
      weekDays={overrides.weekDays ?? WEEK_DAYS}
      todayIso={overrides.todayIso ?? TODAY}
      typesById={overrides.typesById ?? new Map()}
      shiftTypes={overrides.shiftTypes ?? []}
      reducedPath={overrides.reducedPath}
      currentStaffId={overrides.currentStaffId}
      onCellClick={onCellClick}
      onSickReport={overrides.onSickReport}
    />,
  );
  return { onCellClick, ...renderResult };
}

function shiftMap(
  date: string,
  shifts: StaffShift[],
  staffId = member.id,
): Map<string, Map<string, StaffShift[]>> {
  return new Map([[staffId, new Map([[date, shifts]])]]);
}

describe("DienstplanResourceGrid stacking", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useMediaQuery.mockReturnValue(false);
  });

  it("stacks a day's shifts chronologically as separate blocks", () => {
    const late = baseShift({ id: "1", startTime: "10:30", endTime: "15:00" });
    const early = baseShift({ id: "2", startTime: "08:00", endTime: "10:00" });
    // Passed late-first to prove the grid sorts by start time.
    renderGrid({ shiftsByStaff: shiftMap("2026-07-06", [late, early]) });

    const times = screen
      .getAllByText(/^\d\d:\d\d–\d\d:\d\d$/)
      .map((el) => el.textContent);
    expect(times).toEqual(["08:00–10:00", "10:30–15:00"]);
  });

  it("extends the filled-cell hover group across the cell height", () => {
    renderGrid({ shiftsByStaff: shiftMap("2026-07-06", [baseShift()]) });

    const addShiftButton = screen.getByRole("button", {
      name: "Schicht anlegen, Ada Lovelace, Mo 06.07.",
    });
    expect(addShiftButton.parentElement).toHaveClass("group", "flex-1");
  });
});

describe("DienstplanResourceGrid mobile day selector", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useMediaQuery.mockReturnValue(true);
  });

  afterEach(() => {
    mocks.useMediaQuery.mockReturnValue(false);
  });

  it("resets to Monday when the displayed week changes", async () => {
    const { rerender } = renderGrid();
    const friday = screen.getByRole("button", {
      name: /Fr.*10\.07\./,
      pressed: false,
    });

    fireEvent.click(friday);
    expect(friday).toHaveAttribute("aria-pressed", "true");

    rerender(
      <DienstplanResourceGrid
        staff={[member]}
        shiftsByStaff={new Map()}
        assignmentsByStaff={new Map()}
        summaryByStaff={new Map()}
        weekDays={[
          "2026-07-13",
          "2026-07-14",
          "2026-07-15",
          "2026-07-16",
          "2026-07-17",
        ]}
        todayIso={TODAY}
        typesById={new Map()}
        shiftTypes={[]}
        onCellClick={vi.fn()}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("button", {
          name: /Mo.*13\.07\./,
          pressed: true,
        }),
      ).toHaveAttribute("aria-pressed", "true");
    });
  });
});

describe("DienstplanResourceGrid status mapping", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders a cancelled series shift with its reason and no Repeat icon", () => {
    const shift = baseShift({
      cancelled: true,
      changeReason: "Krankheit",
      seriesId: "5",
    });
    renderGrid({ shiftsByStaff: shiftMap("2026-07-06", [shift]) });

    expect(screen.getByText("Fällt aus · Krankheit")).toBeInTheDocument();
    expect(screen.queryByLabelText("Teil einer Serie")).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText("Serie, für diese Woche angepasst"),
    ).not.toBeInTheDocument();
  });

  it("labels a replacement shift as Vertretung with its reason", () => {
    const shift = baseShift({ originShiftId: "9", changeReason: "Tausch" });
    renderGrid({ shiftsByStaff: shiftMap("2026-07-06", [shift]) });

    expect(screen.getByText("Vertretung · Tausch")).toBeInTheDocument();
  });

  it("hatches the regular blocks on a day the person is absent", () => {
    const shift = baseShift({ startTime: "08:00", endTime: "12:00" });
    renderGrid({
      shiftsByStaff: shiftMap("2026-07-06", [shift]),
      assignmentsByStaff: new Map([
        [member.id, new Map([["2026-07-06", [absentAssignment()]]])],
      ]),
    });

    const block = screen.getByRole("button", {
      name: /08:00–12:00 Schicht, abwesend/,
    });
    expect(block.style.backgroundImage).toContain("repeating-linear-gradient");
  });

  it("shows no series icon on a standalone shift", () => {
    renderGrid({ shiftsByStaff: shiftMap("2026-07-06", [baseShift()]) });

    expect(screen.queryByLabelText("Teil einer Serie")).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText("Serie, für diese Woche angepasst"),
    ).not.toBeInTheDocument();
  });

  it("shows the neutral series icon and the amber detached icon", () => {
    const { unmount } = renderGrid({
      shiftsByStaff: shiftMap("2026-07-06", [baseShift({ seriesId: "5" })]),
    });
    expect(screen.getByLabelText("Teil einer Serie")).toHaveClass(
      "text-gray-400",
    );
    unmount();

    renderGrid({
      shiftsByStaff: shiftMap("2026-07-06", [
        baseShift({ seriesId: "5", detached: true }),
      ]),
    });
    expect(
      screen.getByLabelText("Serie, für diese Woche angepasst"),
    ).toHaveClass("text-moto-amber");
  });
});

describe("DienstplanResourceGrid assignment cards", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders a read-only assignment with activity, room and exact gap; clicking it does not open the shift editor", () => {
    const { onCellClick } = renderGrid({
      assignmentsByStaff: new Map([
        [member.id, new Map([["2026-07-06", [assignment()]]])],
      ]),
    });

    expect(screen.getByText("Mensa")).toBeInTheDocument();
    expect(screen.getByText("Speisesaal")).toBeInTheDocument();
    expect(screen.getByText("12:00–14:00")).toBeInTheDocument();
    expect(
      screen.getByText("Nicht abgedeckt: 12:30–14:00"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByText("Mensa"));
    expect(onCellClick).not.toHaveBeenCalled();
  });

  it("shows the assignment-level absence and substitute badges, hiding the uncovered note once covered", () => {
    renderGrid({
      assignmentsByStaff: new Map([
        [
          member.id,
          new Map([
            [
              "2026-07-06",
              [
                assignment({
                  instanceId: "43",
                  isAbsent: true,
                  absenceReason: "krank",
                  coverageStatus: "not_applicable",
                  coverageReason: "absent",
                  uncoveredIntervals: [],
                }),
                assignment({
                  instanceId: "44",
                  activityTitle: "Lernzeit",
                  isSubstitute: true,
                  coverageStatus: "covered",
                  uncoveredIntervals: [],
                }),
              ],
            ],
          ]),
        ],
      ]),
    });

    expect(screen.getByText("Abwesend · krank")).toBeInTheDocument();
    // "Vertretung" also appears as a legend label, so scope to the card.
    expect(
      within(screen.getByTestId("dienstplan-assignment-44-7")).getByText(
        "Vertretung",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Nicht abgedeckt:/)).not.toBeInTheDocument();
  });
});

describe("DienstplanResourceGrid shift click", () => {
  beforeEach(() => vi.clearAllMocks());

  it("opens the shift editor when an existing shift block is clicked", () => {
    const shift = baseShift();
    const { onCellClick } = renderGrid({
      shiftsByStaff: shiftMap("2026-07-06", [shift]),
    });

    fireEvent.click(
      screen.getByRole("button", { name: /08:00–12:00 Schicht/ }),
    );
    expect(onCellClick).toHaveBeenCalledWith(member, "2026-07-06", shift);
  });
});

describe("DienstplanResourceGrid weekly summary", () => {
  beforeEach(() => vi.clearAllMocks());

  function summary(
    overrides: Partial<StaffWeeklySummary> = {},
  ): StaffWeeklySummary {
    return {
      staffId: member.id,
      weekStart: "2026-07-06",
      plannedMinutes: 1080,
      targetMinutes: 1215,
      deltaMinutes: -135,
      ...overrides,
    };
  }

  it("colors an under-contract summary red and names the Soll source", () => {
    renderGrid({ summaryByStaff: new Map([[member.id, summary()]]) });

    const label = screen.getByText("18/20,25 h");
    expect(label).toHaveStyle({ color: "#DC2626" });
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "Geplant 18 h · Soll 20,25 h (aus Arbeitszeitmodell)",
    );
  });

  it("colors an over-contract summary amber", () => {
    renderGrid({
      summaryByStaff: new Map([
        [member.id, summary({ plannedMinutes: 1350, deltaMinutes: 135 })],
      ]),
    });

    expect(screen.getByText("22,5/20,25 h")).toHaveStyle({ color: "#EAB308" });
  });

  it("leaves an exact-match summary neutral (no inline color)", () => {
    renderGrid({
      summaryByStaff: new Map([
        [member.id, summary({ plannedMinutes: 1215, deltaMinutes: 0 })],
      ]),
    });

    const label = screen.getByText("20,25/20,25 h");
    expect(label.style.color).toBe("");
  });

  it("shows only the planned sum without a target", () => {
    renderGrid({
      summaryByStaff: new Map([
        [
          member.id,
          summary({
            plannedMinutes: 270,
            targetMinutes: null,
            deltaMinutes: null,
          }),
        ],
      ]),
    });

    expect(screen.getByText("4,5 h")).toBeInTheDocument();
    expect(screen.queryByText(/\//)).not.toBeInTheDocument();
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "kein Arbeitszeitmodell hinterlegt",
    );
  });
});

describe("DienstplanResourceGrid capacity footer", () => {
  beforeEach(() => vi.clearAllMocks());

  it("counts each staffed person once and excludes cancelled / out-of-window shifts", () => {
    const memberB: StaffScheduleStaff = {
      id: "8",
      firstName: "Bo",
      lastName: "Yilmaz",
    };
    const memberC: StaffScheduleStaff = {
      id: "9",
      firstName: "Cem",
      lastName: "Zorn",
    };
    // A: two overlapping shifts on the same day → counts once.
    const a1 = baseShift({ id: "1", startTime: "12:00", endTime: "14:00" });
    const a2 = baseShift({ id: "2", startTime: "14:00", endTime: "16:00" });
    // B: origin cancelled + replacement in-window → counts 1, not 2, not 0.
    const bOrigin = baseShift({
      id: "3",
      staffId: "8",
      startTime: "12:00",
      endTime: "16:00",
      cancelled: true,
    });
    const bRepl = baseShift({
      id: "4",
      staffId: "8",
      startTime: "12:00",
      endTime: "16:00",
      originShiftId: "3",
    });
    // C: shift entirely before the window → not counted.
    const cOut = baseShift({
      id: "5",
      staffId: "9",
      startTime: "08:00",
      endTime: "11:00",
    });

    const shiftsByStaff = new Map([
      ["7", new Map([["2026-07-06", [a1, a2]]])],
      ["8", new Map([["2026-07-06", [bOrigin, bRepl]]])],
      ["9", new Map([["2026-07-06", [cOut]]])],
    ]);

    renderGrid({ staff: [member, memberB, memberC], shiftsByStaff });

    const capRow = screen.getByText("Kapazität 12–16").closest("tr");
    expect(capRow).not.toBeNull();
    const cells = within(capRow as HTMLElement).getAllByRole("cell");
    // Monday: A (1) + B (replacement 1) = 2; C excluded.
    expect(cells[0]).toHaveTextContent("2");
    // The remaining weekdays have no shifts.
    expect(cells[1]).toHaveTextContent("0");
  });

  it("keeps an absent assignment in planned capacity while its shift remains active", () => {
    renderGrid({
      shiftsByStaff: shiftMap("2026-07-06", [
        baseShift({ startTime: "12:00", endTime: "16:00" }),
      ]),
      assignmentsByStaff: new Map([
        [member.id, new Map([["2026-07-06", [absentAssignment()]]])],
      ]),
    });

    const capRow = screen.getByText("Kapazität 12–16").closest("tr");
    expect(capRow).not.toBeNull();
    const [monday] = within(capRow as HTMLElement).getAllByRole("cell");

    // The strip deliberately describes planned Dienstplan capacity. An
    // assignment-level absence only changes coverage; the shift must be
    // cancelled before the person leaves planned capacity.
    expect(monday).toHaveTextContent("1");
  });
});

describe("DienstplanResourceGrid empty cells", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders a labelled empty-cell button that opens the create modal", () => {
    const { onCellClick } = renderGrid();

    const button = screen.getByRole("button", {
      name: "Schicht anlegen, Ada Lovelace, Mo 06.07.",
    });
    fireEvent.click(button);
    expect(onCellClick).toHaveBeenCalledWith(member, "2026-07-06", null);
  });
});

describe("DienstplanResourceGrid reduced path", () => {
  beforeEach(() => vi.clearAllMocks());

  it("shows only the name (no summary, note, menu) without a sick callback", () => {
    renderGrid({
      reducedPath: true,
      shiftsByStaff: shiftMap("2026-07-06", [baseShift()]),
      // A summary is passed to prove the reduced path ignores it.
      summaryByStaff: new Map([
        [
          member.id,
          {
            staffId: member.id,
            weekStart: "2026-07-06",
            plannedMinutes: 1080,
            targetMinutes: 1215,
            deltaMinutes: -135,
          },
        ],
      ]),
    });

    expect(screen.getByText("Lovelace, Ada")).toBeInTheDocument();
    expect(screen.queryByText("18/20,25 h")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Aktionen für/ }),
    ).not.toBeInTheDocument();
    // The grid still renders shifts and stays usable.
    expect(
      screen.getByRole("button", { name: /08:00–12:00 Schicht/ }),
    ).toBeInTheDocument();
  });

  it("keeps only 'Krank melden' in the reduced-path menu (#1843)", () => {
    const onSickReport = vi.fn();
    renderGrid({
      reducedPath: true,
      shiftsByStaff: shiftMap("2026-07-06", [baseShift()]),
      onSickReport,
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Ada Lovelace" }),
    );
    expect(screen.getAllByRole("menuitem")).toHaveLength(1);
    expect(
      screen.queryByRole("menuitem", { name: "Für heute abwesend melden" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Zeiterfassung öffnen" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("menuitem", { name: "Krank melden" }));
    expect(onSickReport).toHaveBeenCalledWith(member);
  });
});

describe("DienstplanResourceGrid move menu", () => {
  beforeEach(() => vi.clearAllMocks());

  it("opens the move dialog from a non-cancelled shift's context menu", () => {
    renderGrid({ shiftsByStaff: shiftMap("2026-07-06", [baseShift()]) });

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen zur Schicht 08:00–12:00" }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Verschieben nach" }));

    expect(screen.getByText("Schicht verschieben")).toBeInTheDocument();
  });

  it("offers no move menu on a cancelled shift", () => {
    renderGrid({
      shiftsByStaff: shiftMap("2026-07-06", [
        baseShift({ cancelled: true, changeReason: "Krankheit" }),
      ]),
    });

    expect(screen.getByText("Fällt aus · Krankheit")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Aktionen zur Schicht/ }),
    ).not.toBeInTheDocument();
  });
});

describe("DienstplanResourceGrid scroll hint", () => {
  beforeEach(() => vi.clearAllMocks());

  it("renders the horizontal-scroll hint linked to the grid region", () => {
    renderGrid();

    const hint = screen.getByText(
      "Wische horizontal, um weitere Wochentage zu sehen.",
    );
    const region = screen.getByRole("region", {
      name: "Dienstplan-Wochenansicht",
    });
    expect(region).toHaveAttribute("aria-describedby", hint.id);
    expect(region).toHaveAttribute("tabindex", "0");
  });
});

describe("DienstplanResourceGrid row-header menu", () => {
  beforeEach(() => vi.clearAllMocks());

  it("jumps to Vertretung and to /time-tracking for the own person", () => {
    renderGrid({ currentStaffId: "7", onSickReport: vi.fn() });

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Ada Lovelace" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Für heute abwesend melden" }),
    );
    expect(mocks.push).toHaveBeenCalledWith("/vertretung?d=2026-07-06");

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Ada Lovelace" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Zeiterfassung öffnen" }),
    );
    expect(mocks.push).toHaveBeenCalledWith("/time-tracking");
  });

  it("opens the staff detail time-tracking tab for another person", () => {
    renderGrid({ currentStaffId: "99" });

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Ada Lovelace" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Zeiterfassung öffnen" }),
    );
    expect(mocks.push).toHaveBeenCalledWith("/staff/7?tab=zeiterfassung");
  });

  it("hides Krank melden but keeps the other actions when no sick-report callback is given", () => {
    renderGrid({ currentStaffId: "7" });

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Ada Lovelace" }),
    );
    expect(
      screen.queryByRole("menuitem", { name: "Krank melden" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Für heute abwesend melden" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Zeiterfassung öffnen" }),
    ).toBeInTheDocument();
  });
});
