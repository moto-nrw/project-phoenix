import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import type {
  StaffScheduleOverview,
  StaffScheduleStaff,
  StaffShift,
  StaffWeeklySummary,
} from "~/lib/shift-helpers";

const mocks = vi.hoisted(() => ({
  useSWRAuth: vi.fn(),
  push: vi.fn(),
  replace: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mocks.useSWRAuth,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.push, replace: mocks.replace }),
}));

import { DienstplanHalbjahrGrid } from "./dienstplan-halbjahr-grid";

// Six-week planning period, KW 37–42 (2026-09-07 Mon … 2026-10-16 Fri). One
// week cycle (no A/B), active, covers the anchor day 2026-09-16.
const PERIOD_6W: CalendarPeriod = {
  id: "5",
  tenantId: "1",
  name: "Halbjahr 1",
  periodType: "semester",
  startDate: "2026-09-07",
  endDate: "2026-10-16",
  weekCycleLength: 1,
  weekCycleAnchor: null,
  isActive: true,
  createdAt: "2026-08-01T00:00:00Z",
  updatedAt: "2026-08-01T00:00:00Z",
};

const ANCHOR = "2026-09-16"; // Wednesday of KW 38, inside PERIOD_6W.

// [from, to, kw] per week column of PERIOD_6W.
const WEEKS: ReadonlyArray<readonly [string, string, number]> = [
  ["2026-09-07", "2026-09-11", 37],
  ["2026-09-14", "2026-09-18", 38],
  ["2026-09-21", "2026-09-25", 39],
  ["2026-09-28", "2026-10-02", 40],
  ["2026-10-05", "2026-10-09", 41],
  ["2026-10-12", "2026-10-16", 42],
];

const STAFF: StaffScheduleStaff[] = [
  { id: "1", firstName: "Ada", lastName: "Krause" },
  { id: "2", firstName: "Bruno", lastName: "Yilmaz" },
  { id: "3", firstName: "Carla", lastName: "Meyer" },
  { id: "4", firstName: "Dora", lastName: "Schmidt" },
];
const STAFF_ADA: StaffScheduleStaff[] = [STAFF[0]!];

const ovKey = (from: string, to: string): string =>
  `dienstplan-overview-${from}-${to}`;
const shKey = (from: string, to: string): string =>
  `dienstplan-shifts-${from}-${to}`;

function summary(
  staffId: string,
  weekStart: string,
  planned: number,
  target: number | null,
): StaffWeeklySummary {
  return {
    staffId,
    weekStart,
    plannedMinutes: planned,
    targetMinutes: target,
    deltaMinutes: target === null ? null : planned - target,
  };
}

function overview(
  from: string,
  to: string,
  summaries: StaffWeeklySummary[],
  shifts: StaffShift[] = [],
): StaffScheduleOverview {
  return {
    from,
    to,
    dienstplanInUse: true,
    dienstplanUsedWeeks: [],
    staff: [],
    shifts,
    assignments: [],
    weeklySummaries: summaries,
  };
}

function shift(
  overrides: Partial<StaffShift> & { id: string; staffId: string },
): StaffShift {
  return {
    date: "2026-09-07",
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

type OverviewEntry = StaffScheduleOverview | "error";
type ShiftsEntry = StaffShift[] | "error";

// All six weeks resolve to a (possibly empty) ready overview, overridden per
// key. Providing every week keeps cells out of the skeleton state.
function allWeeksReady(
  overrides: Record<string, OverviewEntry> = {},
): Record<string, OverviewEntry> {
  const map: Record<string, OverviewEntry> = {};
  for (const [from, to] of WEEKS) {
    map[ovKey(from, to)] = overview(from, to, []);
  }
  return { ...map, ...overrides };
}

interface SWROptions {
  periods?: CalendarPeriod[];
  periodsError?: Error;
  periodsLoading?: boolean;
  mutatePeriods?: ReturnType<typeof vi.fn>;
  overviewByKey?: Record<string, OverviewEntry>;
  shiftsByKey?: Record<string, ShiftsEntry>;
  weekMutates?: Record<string, ReturnType<typeof vi.fn>>;
}

// Real SWR hands back referentially stable `data`/`mutate` per key across
// renders; the loader's report-up effect relies on that stability to converge.
// So the mock MUST return the same result object for a given key every call —
// a fresh object per render would keep `retry`/`summaryByStaff` changing and
// spin the effect forever. Cache per key.
function configureSWR(opts: SWROptions) {
  const cache = new Map<string, unknown>();
  const stableEmptyShifts: StaffShift[] = [];
  const mutateFor = (key: string): ReturnType<typeof vi.fn> => {
    const cacheKey = `mutate:${key}`;
    let m = cache.get(cacheKey) as ReturnType<typeof vi.fn> | undefined;
    if (!m) {
      m = opts.weekMutates?.[key] ?? vi.fn();
      cache.set(cacheKey, m);
    }
    return m;
  };
  const build = (key: string | null): unknown => {
    if (key === null) {
      return {
        data: undefined,
        error: undefined,
        isLoading: false,
        mutate: vi.fn(),
      };
    }
    if (key === "database-calendar-periods-list") {
      return {
        data: opts.periods,
        error: opts.periodsError,
        isLoading: opts.periodsLoading ?? false,
        mutate: opts.mutatePeriods ?? vi.fn(),
      };
    }
    if (key.startsWith("dienstplan-overview-")) {
      const entry = opts.overviewByKey?.[key];
      const mutate = mutateFor(key);
      if (entry === "error") {
        return {
          data: undefined,
          error: new Error("week failed"),
          isLoading: false,
          mutate,
        };
      }
      if (entry === undefined) {
        return { data: undefined, error: undefined, isLoading: true, mutate };
      }
      return { data: entry, error: undefined, isLoading: false, mutate };
    }
    if (key.startsWith("dienstplan-shifts-")) {
      const entry = opts.shiftsByKey?.[key];
      const mutate = mutateFor(key);
      if (entry === "error") {
        return {
          data: undefined,
          error: new Error("week failed"),
          isLoading: false,
          mutate,
        };
      }
      return {
        data: entry ?? stableEmptyShifts,
        error: undefined,
        isLoading: false,
        mutate,
      };
    }
    return {
      data: undefined,
      error: undefined,
      isLoading: false,
      mutate: vi.fn(),
    };
  };
  mocks.useSWRAuth.mockImplementation((key: string | null) => {
    // null keys can share one stable result; everything else caches per key.
    const cacheKey = key ?? "__null__";
    let result = cache.get(cacheKey);
    if (!result) {
      result = build(key);
      cache.set(cacheKey, result);
    }
    return result;
  });
}

function renderGrid(
  props: Partial<React.ComponentProps<typeof DienstplanHalbjahrGrid>> = {},
) {
  const onWeekClick = props.onWeekClick ?? vi.fn();
  render(
    <DienstplanHalbjahrGrid
      dayISO={props.dayISO ?? ANCHOR}
      staff={props.staff ?? STAFF}
      reducedPath={props.reducedPath ?? false}
      todayIso={props.todayIso ?? ANCHOR}
      closingDayRanges={props.closingDayRanges}
      onWeekClick={onWeekClick}
    />,
  );
  return { onWeekClick };
}

describe("DienstplanHalbjahrGrid", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders one column per calendar week of the found period", () => {
    configureSWR({ periods: [PERIOD_6W], overviewByKey: allWeeksReady() });

    renderGrid();

    for (const [, , kw] of WEEKS) {
      expect(screen.getByText(`KW ${kw}`)).toBeInTheDocument();
    }
    // No week beyond the period end.
    expect(screen.queryByText("KW 43")).not.toBeInTheDocument();
    expect(screen.queryByText("KW 36")).not.toBeInTheDocument();
  });

  it("clamps boundary-week requests and totals to the planning period", async () => {
    const partialPeriod: CalendarPeriod = {
      ...PERIOD_6W,
      startDate: "2026-09-09", // Wednesday of KW 37
      endDate: "2026-09-16", // Wednesday of KW 38
    };
    const firstBoundary = ovKey("2026-09-09", "2026-09-13");
    const secondBoundary = ovKey("2026-09-14", "2026-09-16");
    configureSWR({
      periods: [partialPeriod],
      overviewByKey: {
        [firstBoundary]: overview(
          "2026-09-09",
          "2026-09-13",
          // The backend summary covers the complete week and includes an
          // out-of-period Monday shift (8 h total). The viewport payload only
          // contains Wednesday's in-period 4 h shift.
          [summary("1", "2026-09-07", 480, null)],
          [shift({ id: "boundary", staffId: "1", date: "2026-09-09" })],
        ),
        [secondBoundary]: overview("2026-09-14", "2026-09-16", []),
      },
    });

    renderGrid({ staff: STAFF_ADA });

    const keys = mocks.useSWRAuth.mock.calls.map(([key]) => key);
    expect(keys).toContain(ovKey("2026-09-09", "2026-09-13"));
    expect(keys).toContain(ovKey("2026-09-14", "2026-09-16"));
    expect(keys).not.toContain(ovKey("2026-09-07", "2026-09-11"));
    expect(keys).not.toContain(ovKey("2026-09-14", "2026-09-18"));
    expect(await screen.findByText("4 h")).toBeInTheDocument();
    expect(screen.queryByText("8 h")).not.toBeInTheDocument();
  });

  it("uses a valid weekend range when the period begins on Saturday", () => {
    const weekendStartPeriod: CalendarPeriod = {
      ...PERIOD_6W,
      startDate: "2026-09-12", // Saturday of KW 37
      endDate: "2026-09-14", // Monday of KW 38
    };
    configureSWR({
      periods: [weekendStartPeriod],
      overviewByKey: {
        [ovKey("2026-09-12", "2026-09-13")]: overview(
          "2026-09-12",
          "2026-09-13",
          [],
        ),
        [ovKey("2026-09-14", "2026-09-14")]: overview(
          "2026-09-14",
          "2026-09-14",
          [],
        ),
      },
    });

    renderGrid({ dayISO: "2026-09-12", staff: STAFF_ADA });

    const keys = mocks.useSWRAuth.mock.calls.map(([key]) => key);
    expect(keys).toContain(ovKey("2026-09-12", "2026-09-13"));
    expect(keys).toContain(ovKey("2026-09-14", "2026-09-14"));
    expect(keys).not.toContain(ovKey("2026-09-12", "2026-09-11"));
  });

  it("colors the cell sum by delta and shows only the sum without a target", async () => {
    // All summaries in KW 38 only; the other weeks stay empty. Assertions target
    // the label span's inline tone color and the "/soll" separator, never the
    // decimal digits — de-DE number formatting is locale-data dependent and
    // renders "19.5" instead of "19,5" on a bare Node (see chart.test.tsx note).
    const kw38 = ovKey("2026-09-14", "2026-09-18");
    configureSWR({
      periods: [PERIOD_6W],
      overviewByKey: allWeeksReady({
        [kw38]: overview("2026-09-14", "2026-09-18", [
          summary("1", "2026-09-14", 1170, 1215), // under → red
          summary("2", "2026-09-14", 1320, 1215), // over → amber
          summary("3", "2026-09-14", 1215, 1215), // exact → neutral
          summary("4", "2026-09-14", 600, null), // no target → sum only
        ]),
      }),
    });

    renderGrid();

    // Summary label span (tabular-nums) inside a week cell, by staff.
    const labelSpan = (name: string): HTMLElement => {
      const button = screen.getByLabelText(`Woche 38 öffnen, ${name}`);
      const span = button.querySelector("span.tabular-nums");
      if (!(span instanceof HTMLElement)) {
        throw new Error(`no summary label for ${name}`);
      }
      return span;
    };

    await screen.findByLabelText("Woche 38 öffnen, Ada Krause");

    const under = labelSpan("Ada Krause");
    expect(under).toHaveStyle({ color: "#FF3130" });
    expect(under.textContent).toContain("/");

    expect(labelSpan("Bruno Yilmaz")).toHaveStyle({ color: "#EAB308" });

    const neutral = labelSpan("Carla Meyer");
    expect(neutral).not.toHaveStyle({ color: "#FF3130" });
    expect(neutral).not.toHaveStyle({ color: "#EAB308" });
    expect(neutral.textContent).toContain("/");

    // No target → just the planned sum, no "/soll" separator, neutral tone.
    const noTarget = labelSpan("Dora Schmidt");
    expect(noTarget).not.toHaveStyle({ color: "#FF3130" });
    expect(noTarget).not.toHaveStyle({ color: "#EAB308" });
    expect(noTarget.textContent).not.toContain("/");
  });

  it("leaves a week with zero planned minutes as an empty cell", async () => {
    const kw38 = ovKey("2026-09-14", "2026-09-18");
    configureSWR({
      periods: [PERIOD_6W],
      overviewByKey: allWeeksReady({
        [kw38]: overview("2026-09-14", "2026-09-18", [
          summary("1", "2026-09-14", 600, null),
        ]),
      }),
    });

    renderGrid({ staff: STAFF_ADA });

    // The one non-empty week is clickable…
    expect(
      await screen.findByLabelText("Woche 38 öffnen, Ada Krause"),
    ).toBeInTheDocument();
    // …every empty week renders no cell button and no "0" noise.
    expect(
      screen.queryByLabelText("Woche 37 öffnen, Ada Krause"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("0 h")).not.toBeInTheDocument();
    expect(screen.queryByText("0")).not.toBeInTheDocument();
  });

  it("navigates into the week via onWeekClick with the Monday of the cell", async () => {
    const kw38 = ovKey("2026-09-14", "2026-09-18");
    configureSWR({
      periods: [PERIOD_6W],
      overviewByKey: allWeeksReady({
        [kw38]: overview("2026-09-14", "2026-09-18", [
          summary("1", "2026-09-14", 600, null),
        ]),
      }),
    });

    const { onWeekClick } = renderGrid({ staff: STAFF_ADA });

    const cell = await screen.findByLabelText("Woche 38 öffnen, Ada Krause");
    fireEvent.click(cell);
    expect(onWeekClick).toHaveBeenCalledWith("2026-09-14");
  });

  it("shows a retry placeholder for a failed week column and re-triggers its load", async () => {
    const kw38 = ovKey("2026-09-14", "2026-09-18");
    const retry = vi.fn();
    configureSWR({
      periods: [PERIOD_6W],
      overviewByKey: allWeeksReady({ [kw38]: "error" }),
      weekMutates: { [kw38]: retry },
    });

    renderGrid({ staff: STAFF_ADA });

    const retryButton = await screen.findByLabelText("Woche 38 erneut laden");
    expect(retryButton).toBeInTheDocument();
    fireEvent.click(retryButton);
    expect(retry).toHaveBeenCalledTimes(1);
  });

  it("sums client-side from shifts and excludes cancelled ones on the reduced path", async () => {
    const week1 = shKey("2026-09-07", "2026-09-13");
    configureSWR({
      periods: [PERIOD_6W],
      shiftsByKey: {
        [week1]: [
          shift({
            id: "1",
            staffId: "1",
            startTime: "08:00",
            endTime: "12:00",
          }), // 240 min counts
          shift({
            id: "2",
            staffId: "1",
            startTime: "13:00",
            endTime: "16:00",
            cancelled: true,
          }), // 180 min excluded (B6)
          shift({
            id: "3",
            staffId: "1",
            date: "2026-09-12",
            startTime: "10:00",
            endTime: "12:00",
          }), // Saturday is part of the weekly total
        ],
      },
    });

    renderGrid({ staff: STAFF_ADA, reducedPath: true });

    // Both non-cancelled weekday + weekend shifts contribute: 360 min = 6 h.
    const cell = await screen.findByText("6 h");
    expect(cell).toBeInTheDocument();
    expect(screen.queryByText("4 h")).not.toBeInTheDocument();
    expect(screen.queryByText("7 h")).not.toBeInTheDocument();
    // Reduced path has no target → neutral, never red/amber.
    expect(cell).not.toHaveStyle({ color: "#FF3130" });
    expect(cell).not.toHaveStyle({ color: "#EAB308" });
  });

  it("shows the no-period empty state with a jump to the calendar periods", () => {
    configureSWR({ periods: [] });

    renderGrid({ staff: [] });

    expect(
      screen.getByText("Kein Planungszeitraum für dieses Datum"),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Zu den Planungszeiträumen" }),
    );
    expect(mocks.push).toHaveBeenCalledWith("/calendar-periods");
  });
});

// #2032: Wochen mit Schließtagen sind in der Halbjahres-Sicht erkennbar.
describe("DienstplanHalbjahrGrid closing days", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("marks the week column containing closing days with their count and days", () => {
    configureSWR({ periods: [PERIOD_6W], overviewByKey: allWeeksReady() });

    renderGrid({
      closingDayRanges: [
        {
          startDate: "2026-09-16",
          endDate: "2026-09-17",
          reason: "Pädagogischer Tag",
        },
      ],
    });

    const marked = screen.getByTitle(
      "2 Schließtage: 16.09. Pädagogischer Tag, 17.09. Pädagogischer Tag",
    );
    // KW 38 (Mo 14.09.) ist die Woche, in die beide Tage fallen.
    expect(marked.closest("th")).toHaveTextContent("KW 38");
    // Keine andere Woche trägt eine Kennzeichnung.
    expect(screen.getAllByText(/Schließtag/).length).toBe(1);
  });

  it("keeps every column unmarked without closing days", () => {
    configureSWR({ periods: [PERIOD_6W], overviewByKey: allWeeksReady() });

    renderGrid();

    expect(screen.queryByText(/Schließtag/)).not.toBeInTheDocument();
  });
});
