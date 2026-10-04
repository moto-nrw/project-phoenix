import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { StaffShift } from "~/lib/shift-helpers";
import type { DayProjection } from "~/lib/time-tracking-helpers";

interface SwrResult {
  data: unknown;
  error: Error | undefined;
  isLoading: boolean;
  mutate: () => Promise<unknown>;
}

const state = vi.hoisted(() => ({
  shifts: undefined as StaffShift[] | undefined,
  shiftsError: undefined as Error | undefined,
  // useSWRAuth meldet isLoading=false, solange es den Abruf bis zur Session
  // zurückhält.
  shiftsHeldBack: false,
  projection: undefined as ReadonlyMap<string, DayProjection> | undefined,
  timetableEnabled: true,
  day: null as string | null,
  keys: [] as (string | null)[],
  updateParams: vi.fn(),
  mutate: vi.fn(() => Promise.resolve(undefined)),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null): SwrResult => {
    state.keys.push(key);
    if (key?.startsWith("time-tracking-own-shifts-week-")) {
      return {
        data: state.shifts,
        error: state.shiftsError,
        isLoading:
          state.shifts === undefined &&
          !state.shiftsError &&
          !state.shiftsHeldBack,
        mutate: state.mutate,
      };
    }
    if (key?.startsWith("time-tracking-schedule-targets-")) {
      return {
        data: state.projection,
        error: undefined,
        isLoading: false,
        mutate: state.mutate,
      };
    }
    return {
      data: undefined,
      error: undefined,
      isLoading: false,
      mutate: state.mutate,
    };
  },
}));
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-09-09",
}));
vi.mock("~/lib/hooks/use-url-params", () => ({
  useUrlParams: () => ({
    params: { d: state.day },
    updateParams: state.updateParams,
  }),
}));
vi.mock("~/lib/hooks/use-closing-days", () => ({
  useClosingDaysState: () => ({
    closingDays: new Map([["2026-09-11", "Brückentag"]]),
    closingDayRanges: [],
    isLoading: false,
  }),
}));
vi.mock("~/lib/tenant-context", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/tenant-context")>()),
  useTimetableEnabled: () => state.timetableEnabled,
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));
vi.mock("~/lib/shift-api", () => ({
  ownShiftService: { getOwnShifts: vi.fn() },
}));
vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: { getDailyProjection: vi.fn() },
}));

import { OwnDienstplanView } from "./own-dienstplan-view";

function shift(overrides: Partial<StaffShift> = {}): StaffShift {
  return {
    id: "11",
    staffId: "42",
    date: "2026-09-07",
    startTime: "08:00",
    endTime: "12:00",
    breakMinutes: 0,
    shiftTypeId: "21",
    shiftTypeName: "Frühdienst",
    shiftTypeColor: "#83CD2D",
    notes: "",
    seriesId: null,
    detached: false,
    cancelled: false,
    changeReason: null,
    originShiftId: null,
    ...overrides,
  };
}

function target(minutes: number): DayProjection {
  return {
    targetMinutes: minutes,
    creditMinutes: 0,
    actualMinutes: 0,
    balanceMinutes: 0,
  };
}

const WORKDAYS = [
  "2026-09-07",
  "2026-09-08",
  "2026-09-09",
  "2026-09-10",
  "2026-09-11",
];

describe("OwnDienstplanView", () => {
  beforeEach(() => {
    state.shifts = undefined;
    state.shiftsError = undefined;
    state.shiftsHeldBack = false;
    state.projection = undefined;
    state.timetableEnabled = true;
    state.day = null;
    state.keys = [];
    state.updateParams.mockClear();
    state.mutate.mockClear();
  });

  it("shows the own week read-only with sums by Schichtart and the Soll", () => {
    state.shifts = [
      shift(),
      shift({
        id: "12",
        startTime: "12:30",
        endTime: "16:00",
        breakMinutes: 30,
        shiftTypeId: "22",
        shiftTypeName: "Spätdienst",
        shiftTypeColor: "#5080D8",
      }),
      shift({ id: "13", date: "2026-09-08" }),
      shift({ id: "14", date: "2026-09-09", cancelled: true }),
    ];
    state.projection = new Map(
      WORKDAYS.map((day) => [day, target(240)] as const),
    );

    render(<OwnDienstplanView />);

    expect(
      screen.getByRole("heading", { name: "Mein Dienstplan" }),
    ).toBeInTheDocument();
    // 4 h + 3 h + 4 h geplant (die ausgefallene Schicht zählt nicht), Soll
    // 5 × 4 h.
    const stats = screen.getByText("geplant").parentElement?.parentElement;
    expect(stats?.textContent).toBe("11 hgeplant·20 hSoll·−9 hDifferenz");

    // Lesend: kein Anlegen, kein Bearbeiten.
    expect(
      screen.queryByRole("button", { name: /Schicht anlegen/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getAllByRole("group", { name: "08:00–12:00 Frühdienst" }),
    ).toHaveLength(2);
    expect(screen.getByText(/Nur zur Information/)).toBeInTheDocument();

    const hours = screen.getByTestId("own-week-hours-card");
    expect(hours).toHaveTextContent("Frühdienst8 h");
    expect(hours).toHaveTextContent("Spätdienst3 h");
    expect(hours).toHaveTextContent("Gesamt11 h");

    // Wochenend-Spalten nur bei Wochenend-Diensten; der Schließtag ist markiert.
    expect(screen.queryByText(/^Sa /)).not.toBeInTheDocument();
    expect(screen.getByTitle("Schließtag: Brückentag")).toBeInTheDocument();
  });

  it("loads Monday to Sunday so weekend shifts count and show", () => {
    state.shifts = [shift({ id: "15", date: "2026-09-13" })];

    render(<OwnDienstplanView />);

    expect(state.keys).toContain(
      "time-tracking-own-shifts-week-2026-09-07-2026-09-13",
    );
    expect(state.keys).toContain(
      "time-tracking-schedule-targets-2026-09-07-2026-09-13",
    );
    expect(screen.getByText("So 13.09.")).toBeInTheDocument();
    expect(screen.getByText("Sa 12.09.")).toBeInTheDocument();
  });

  it("omits Soll and Differenz while no target is known", () => {
    state.shifts = [shift()];

    render(<OwnDienstplanView />);

    expect(screen.getByText("geplant")).toBeInTheDocument();
    expect(screen.queryByText("Soll")).not.toBeInTheDocument();
    expect(screen.queryByText("Differenz")).not.toBeInTheDocument();
  });

  it("explains an empty week and who plans it", () => {
    state.shifts = [];

    render(<OwnDienstplanView />);

    expect(
      screen.getByText(/Keine Schichten in dieser Woche\./),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Ihre Schichten plant die Leitung im Dienstplan/),
    ).toBeInTheDocument();
  });

  it("keeps loading instead of claiming an empty week before the session", () => {
    state.shiftsHeldBack = true;

    render(<OwnDienstplanView />);

    expect(
      screen.queryByText(/Keine Schichten in dieser Woche/),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("geplant")).not.toBeInTheDocument();
  });

  it("offers a retry when the shifts fail to load", () => {
    state.shiftsError = new Error("boom");

    render(<OwnDienstplanView />);

    expect(
      screen.getByText(/Ihr Dienstplan konnte nicht geladen werden/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));
    expect(state.mutate).toHaveBeenCalled();
  });

  it("navigates by week through the d parameter", () => {
    state.shifts = [shift()];

    render(<OwnDienstplanView />);

    fireEvent.click(screen.getByRole("button", { name: "Nächste Woche" }));
    expect(state.updateParams).toHaveBeenCalledWith({ d: "2026-09-14" });
    fireEvent.click(screen.getByRole("button", { name: "Vorherige Woche" }));
    expect(state.updateParams).toHaveBeenCalledWith({ d: "2026-08-31" });
  });

  it("shows another week from the URL and offers the way back", () => {
    state.day = "2026-09-16";
    state.shifts = [shift({ date: "2026-09-14" })];

    render(<OwnDienstplanView />);

    expect(state.keys).toContain(
      "time-tracking-own-shifts-week-2026-09-14-2026-09-20",
    );
    fireEvent.click(screen.getByRole("button", { name: "Diese Woche" }));
    expect(state.updateParams).toHaveBeenCalledWith({ d: null });
  });

  it("says so when the school does not plan shifts in moto", () => {
    state.timetableEnabled = false;

    render(<OwnDienstplanView />);

    expect(
      screen.getByText(/Der Dienstplan ist nicht verfügbar/),
    ).toBeInTheDocument();
    expect(
      state.keys.filter((key) => key?.startsWith("time-tracking-own-shifts")),
    ).toEqual([]);
  });
});
