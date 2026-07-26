import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { StaffAbsenceRow } from "~/lib/staff-api";

import { AbsenceRequestRow } from "./absence-request-row";

function requestedRow(): StaffAbsenceRow {
  return {
    id: 17,
    staff_id: 42,
    absence_type: "vacation",
    date_start: "2027-07-10",
    date_end: "2027-07-11",
    half_day: false,
    note: "Vertretung ist geklärt",
    status: "requested",
    decision_note: "Wer übernimmt die Frühschicht?",
    working_days: 2,
    requested_at: "2027-06-02T08:00:00Z",
  };
}

describe("AbsenceRequestRow resubmitted requests", () => {
  it("shows the retained manager question as previous context", () => {
    render(
      <AbsenceRequestRow
        row={requestedRow()}
        staffName="Mila Muster"
        isBusy={false}
        showActions
        onApprove={vi.fn()}
        onDeny={vi.fn()}
        onQuestion={vi.fn()}
      />,
    );

    expect(screen.getByText("Vorherige Rückfrage:")).toBeInTheDocument();
    expect(
      screen.getByText("Wer übernimmt die Frühschicht?"),
    ).toBeInTheDocument();
  });
});
