import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ABSENCES_REFRESH_EVENT } from "~/lib/absence-helpers";
import type { StaffAbsence } from "~/lib/time-tracking-helpers";

import { LeaveRequestsCard } from "./leave-requests-card";

const mocks = vi.hoisted(() => ({
  cancelAbsence: vi.fn(),
  getAbsences: vi.fn(),
  getVacationQuota: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onConfirm,
    children,
  }: {
    readonly isOpen: boolean;
    readonly onConfirm: () => void;
    readonly children: ReactNode;
  }) =>
    isOpen ? (
      <div>
        {children}
        <button type="button" onClick={onConfirm}>
          Stornierung bestätigen
        </button>
      </div>
    ) : null,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    error: mocks.toastError,
    success: mocks.toastSuccess,
  }),
}));

vi.mock("~/lib/hooks/use-current-timestamp", () => ({
  useCurrentTimestamp: () => new Date("2027-06-01T10:00:00Z").getTime(),
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: {
    cancelAbsence: mocks.cancelAbsence,
    getAbsences: mocks.getAbsences,
    getVacationQuota: mocks.getVacationQuota,
    resubmitAbsence: vi.fn(),
  },
}));

vi.mock("./vacation-request-modal", () => ({
  VacationRequestModal: () => null,
}));

function resubmittedVacation(): StaffAbsence {
  return {
    id: "17",
    staffId: "42",
    absenceType: "vacation",
    dateStart: "2027-07-10",
    dateEnd: "2027-07-11",
    halfDay: false,
    startHalfDay: false,
    endHalfDay: false,
    note: "Vertretung ist geklärt",
    status: "requested",
    approvedBy: null,
    approvedAt: null,
    createdBy: "42",
    createdAt: "2027-06-01T08:00:00Z",
    updatedAt: "2027-06-02T08:00:00Z",
    durationDays: 2,
    workingDays: 2,
    decisionNote: "Wer übernimmt die Frühschicht?",
    requestedAt: "2027-06-02T08:00:00Z",
    substituteStaffId: null,
  };
}

describe("LeaveRequestsCard resubmitted requests", () => {
  beforeEach(() => {
    mocks.cancelAbsence.mockReset();
    mocks.getAbsences.mockReset();
    mocks.getVacationQuota.mockReset();
    mocks.toastError.mockReset();
    mocks.toastSuccess.mockReset();
    mocks.cancelAbsence.mockResolvedValue(undefined);
    mocks.getAbsences.mockResolvedValue([resubmittedVacation()]);
    mocks.getVacationQuota.mockResolvedValue({
      staff_id: 42,
      year: 2027,
      entitled_days: 20,
      carryover_days: 0,
      taken_days: 0,
      reserved_days: 2,
      remaining_days: 18,
    });
  });

  it("labels the retained question and refreshes the badge after cancellation", async () => {
    const listener = vi.fn();
    window.addEventListener(ABSENCES_REFRESH_EVENT, listener);

    render(<LeaveRequestsCard />);

    expect(
      await screen.findByText("Vorherige Rückfrage der Leitung:"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Stornieren" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Stornierung bestätigen" }),
    );

    await waitFor(() => {
      expect(mocks.cancelAbsence).toHaveBeenCalledWith("17");
      expect(listener).toHaveBeenCalledTimes(1);
    });
    window.removeEventListener(ABSENCES_REFRESH_EVENT, listener);
  });
});
