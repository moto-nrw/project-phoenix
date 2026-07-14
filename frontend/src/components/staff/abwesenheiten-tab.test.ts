import { describe, expect, it } from "vitest";

import type { StaffAbsenceRow } from "~/lib/staff-api";

import { isVacationWorkflowAbsence } from "./abwesenheiten-tab";

function absence(overrides: Partial<StaffAbsenceRow>): StaffAbsenceRow {
  return {
    id: 1,
    staff_id: 2,
    absence_type: "sick",
    date_start: "2026-07-14",
    date_end: "2026-07-14",
    half_day: false,
    note: "",
    status: "reported",
    ...overrides,
  };
}

describe("isVacationWorkflowAbsence", () => {
  it.each(["requested", "approved", "declined", "canceled"])(
    "identifies %s vacation rows as workflow-managed",
    (status) => {
      expect(
        isVacationWorkflowAbsence(
          absence({ absence_type: "vacation", status }),
        ),
      ).toBe(true);
    },
  );

  it("keeps reported vacation and non-vacation rows deletable", () => {
    expect(
      isVacationWorkflowAbsence(
        absence({ absence_type: "vacation", status: "reported" }),
      ),
    ).toBe(false);
    expect(
      isVacationWorkflowAbsence(
        absence({ absence_type: "sick", status: "approved" }),
      ),
    ).toBe(false);
  });
});
