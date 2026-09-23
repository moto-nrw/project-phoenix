import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { StaffSchedule } from "~/lib/staff-api";
import type { DayProjection } from "~/lib/time-tracking-helpers";
import { setTestClock } from "~/test/clock";
import { ArbeitszeitmodellTab } from "./arbeitszeitmodell-tab";

const mocks = vi.hoisted(() => ({
  projection: undefined as ReadonlyMap<string, DayProjection> | undefined,
  projectionError: undefined as Error | undefined,
  projectionKey: "",
}));

const schedule: StaffSchedule = {
  mode: "custom",
  model: null,
  rotationLength: 1,
  rotationAnchorDate: "2026-08-10",
  entries: [0, 1, 2, 3, 4].map((dayOfWeek) => ({
    weekIndex: 0,
    dayOfWeek,
    targetMinutes: 480,
  })),
  weeklyTotals: [2400],
  validFrom: "2026-08-10",
};

vi.mock("swr", () => ({ useSWRConfig: () => ({ mutate: vi.fn() }) }));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    if (key === "staff-schedule-42") {
      return { data: schedule, isLoading: false, mutate: vi.fn() };
    }
    if (key?.startsWith("staff-schedule-targets-preview-")) {
      mocks.projectionKey = key;
      return {
        data: mocks.projection,
        error: mocks.projectionError,
        isLoading: false,
        mutate: vi.fn(),
      };
    }
    return { data: [], isLoading: false, mutate: vi.fn() };
  },
}));

vi.mock("~/lib/staff-api", () => ({
  staffScheduleService: { getSchedule: vi.fn(), updateSchedule: vi.fn() },
  workTimeModelService: { list: vi.fn() },
  staffMonthSummaryService: { getDailyProjection: vi.fn() },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
}));

function day(targetMinutes: number, isOverride = false): DayProjection {
  return {
    targetMinutes,
    creditMinutes: 0,
    actualMinutes: 0,
    balanceMinutes: 0,
    ...(isOverride ? { isOverride: true } : {}),
  };
}

// 14.09.–11.10.2026: KW 38 regular, KW 39 Sonderarbeitszeit 8,5 h, KW 40
// Schließtage with a 0-hour Sonderarbeitszeit on Monday, KW 41 regular.
function autumnProjection(): ReadonlyMap<string, DayProjection> {
  const map = new Map<string, DayProjection>();
  const start = new Date(2026, 8, 14);
  for (let offset = 0; offset < 28; offset++) {
    const date = new Date(start);
    date.setDate(start.getDate() + offset);
    const key = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
    const weekday = date.getDay();
    if (weekday === 0 || weekday === 6) {
      map.set(key, day(0));
    } else if (offset >= 7 && offset < 14) {
      map.set(key, day(510, true));
    } else if (offset === 14) {
      map.set(key, day(0, true));
    } else if (offset > 14 && offset < 21) {
      map.set(key, day(0));
    } else {
      map.set(key, day(480));
    }
  }
  return map;
}

function weekRow(label: string): HTMLElement {
  const row = screen.getByText(label).parentElement;
  if (!row) throw new Error(`week row ${label} missing`);
  return row;
}

describe("Arbeitszeitmodell preview", () => {
  beforeEach(() => {
    setTestClock("2026-09-16T10:00:00+02:00");
    mocks.projection = autumnProjection();
    mocks.projectionError = undefined;
  });

  it("reads the four weeks from the daily Soll the time tracking uses", () => {
    render(<ArbeitszeitmodellTab staffId="42" canEdit={false} />);

    expect(mocks.projectionKey).toBe(
      "staff-schedule-targets-preview-42-2026-09-14-2026-10-11",
    );
    expect(within(weekRow("KW 38")).getByText("40h")).toBeInTheDocument();
    expect(
      within(weekRow("KW 38")).queryByText("Sonderarbeitszeit"),
    ).toBeNull();
  });

  it("shows a Sonderarbeitszeit week with its own hours and marks it", () => {
    render(<ArbeitszeitmodellTab staffId="42" canEdit={false} />);

    const row = weekRow("KW 39");
    expect(within(row).getByText("Sonderarbeitszeit")).toBeInTheDocument();
    expect(within(row).getAllByText("8h 30min")).toHaveLength(5);
    expect(within(row).getByText("42h 30min")).toBeInTheDocument();
  });

  it("counts closing days as 0 and shows 0h on a 0-hour Sonderarbeitszeit", () => {
    render(<ArbeitszeitmodellTab staffId="42" canEdit={false} />);

    const row = weekRow("KW 40");
    expect(within(row).getByText("0h")).toBeInTheDocument();
    expect(within(row).getAllByText("-")).toHaveLength(4);
    expect(within(row).getByText("0min")).toBeInTheDocument();
  });

  it("invents no Soll from the current model while the daily Soll is missing", () => {
    mocks.projection = undefined;
    mocks.projectionError = new Error("offline");
    render(<ArbeitszeitmodellTab staffId="42" canEdit={false} />);

    expect(within(weekRow("KW 38")).queryByText("8h")).toBeNull();
    expect(within(weekRow("KW 38")).getAllByText("?").length).toBeGreaterThan(
      0,
    );
  });
});
