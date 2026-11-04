import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";
import { TEST_CLOCK_INSTANT, TEST_CLOCK_TODAY } from "~/test/clock";
import {
  PLANNED_WINDOW_SLACK_MS,
  nextPlannedWindowBoundary,
  usePlannedWindowRefresh,
} from "./planned-window";

// The frozen clock is 12:00 Berlin. The block starts at 12:30 and can be
// started 15 minutes before, so it enters „Als Nächstes“ at 12:15 (#5017).
function block(
  overrides: Partial<PlannedTimetableInstance> = {},
): PlannedTimetableInstance {
  return {
    id: "1",
    title: "Hausaufgaben",
    date: TEST_CLOCK_TODAY,
    startTime: "12:30:00",
    endTime: "13:30:00",
    roomId: "4",
    roomName: "Raum 4",
    status: "planned",
    isOverdue: false,
    minutesUntilStart: 30,
    expectedStudentsCount: 8,
    presentStudentsCount: 0,
    notScheduledStudentsCount: 0,
    assignedStaffIds: ["7"],
    isAssigned: true,
    isPrimary: true,
    isSubstitute: false,
    isAbsent: false,
    rosterPreview: [],
    startAvailableAt: "2026-09-09T12:15:00+02:00",
    ...overrides,
  } as PlannedTimetableInstance;
}

const at = (iso: string) => Date.parse(iso);
const now = () => TEST_CLOCK_INSTANT.getTime();

function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    get: () => state,
  });
}

afterEach(() => setVisibility("visible"));

describe("nextPlannedWindowBoundary", () => {
  it("names the moment the next planned block enters the window", () => {
    expect(nextPlannedWindowBoundary([block()], now())).toBe(
      at("2026-09-09T12:15:00+02:00"),
    );
  });

  it("uses a start lead wider than the horizon", () => {
    const early = block({ startAvailableAt: "2026-09-09T12:05:00+02:00" });
    expect(nextPlannedWindowBoundary([early], now())).toBe(
      at("2026-09-09T12:05:00+02:00"),
    );
  });

  it("names the end of a block already in the window", () => {
    const current = block({
      startTime: "11:50:00",
      endTime: "12:40:00",
      startAvailableAt: "2026-09-09T11:35:00+02:00",
    });
    expect(nextPlannedWindowBoundary([current], now())).toBe(
      at("2026-09-09T12:40:00+02:00"),
    );
  });

  it("ignores running, finished and cancelled blocks", () => {
    const blocks = (["active", "completed", "cancelled"] as const).map(
      (status) => block({ status }),
    );
    expect(nextPlannedWindowBoundary(blocks, now())).toBeNull();
  });
});

describe("usePlannedWindowRefresh", () => {
  it("reloads when the block becomes startable, not before", async () => {
    vi.useFakeTimers();
    const refresh = vi.fn();
    renderHook(() => usePlannedWindowRefresh([block()], refresh));

    await act(async () => {
      await vi.advanceTimersByTimeAsync(15 * 60_000);
    });
    expect(refresh).not.toHaveBeenCalled();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(PLANNED_WINDOW_SLACK_MS);
    });
    expect(refresh).toHaveBeenCalledTimes(1);

    // The next boundary of the same answer is the block's end at 13:30.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(75 * 60_000);
    });
    expect(refresh).toHaveBeenCalledTimes(2);
  });

  it("waits in a hidden tab and catches up once visible", async () => {
    vi.useFakeTimers();
    setVisibility("hidden");
    const refresh = vi.fn();
    renderHook(() => usePlannedWindowRefresh([block()], refresh));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(16 * 60_000);
    });
    expect(refresh).not.toHaveBeenCalled();

    setVisibility("visible");
    act(() => {
      document.dispatchEvent(new Event("visibilitychange"));
    });
    expect(refresh).toHaveBeenCalledTimes(1);
  });
});
