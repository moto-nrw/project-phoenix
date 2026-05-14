import { describe, expect, it } from "vitest";

import type { StaffAbsenceRow, StaffSchedule } from "./staff-api";
import { computeStaffMetrics } from "./staff-metrics-helpers";

const schedule: StaffSchedule = {
  mode: "custom",
  model: null,
  rotationLength: 1,
  rotationAnchorDate: "2026-06-01",
  validFrom: "2026-06-01",
  weeklyTotals: [2400],
  entries: [0, 1, 2, 3, 4].map((dayOfWeek) => ({
    weekIndex: 0,
    dayOfWeek,
    targetMinutes: 480,
  })),
};

function absence(overrides: Partial<StaffAbsenceRow>): StaffAbsenceRow {
  return {
    id: 100,
    staff_id: 100,
    absence_type: "vacation",
    date_start: "2026-06-01",
    date_end: "2026-06-05",
    half_day: false,
    note: "",
    status: "approved",
    ...overrides,
  };
}

describe("computeStaffMetrics absence credit", () => {
  it("uses boundary half days instead of halving the whole range", () => {
    const metrics = computeStaffMetrics(
      schedule,
      [],
      [
        absence({
          half_day: true,
          start_half_day: true,
          end_half_day: false,
        }),
      ],
      new Date(2026, 5, 5),
    );

    expect(metrics.weekIst).toBe(2160);
    expect(metrics.weekDelta).toBe(-240);
  });

  it("does not credit pending, declined, or canceled absences", () => {
    const metrics = computeStaffMetrics(
      schedule,
      [],
      [
        absence({ status: "requested" }),
        absence({ status: "declined" }),
        absence({ status: "canceled" }),
      ],
      new Date(2026, 5, 5),
    );

    expect(metrics.weekIst).toBe(0);
    expect(metrics.weekDelta).toBe(-2400);
  });
});
