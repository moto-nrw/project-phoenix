import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ABSENCES_REFRESH_EVENT } from "~/lib/absence-helpers";
import type { StaffAbsence } from "~/lib/time-tracking-helpers";

import { LeaveRequestsCard } from "./leave-requests-card";

const mocks = vi.hoisted(() => ({
  cancelAbsence: vi.fn(),
  getAbsences: vi.fn(),
  getQuestionedAbsences: vi.fn(),
  getVacationQuota: vi.fn(),
  resubmitAbsence: vi.fn(),
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
    getQuestionedAbsences: mocks.getQuestionedAbsences,
    getVacationQuota: mocks.getVacationQuota,
    resubmitAbsence: mocks.resubmitAbsence,
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
    mocks.getQuestionedAbsences.mockReset();
    mocks.getVacationQuota.mockReset();
    mocks.resubmitAbsence.mockReset();
    mocks.toastError.mockReset();
    mocks.toastSuccess.mockReset();
    mocks.cancelAbsence.mockResolvedValue(undefined);
    mocks.getAbsences.mockResolvedValue([resubmittedVacation()]);
    mocks.getQuestionedAbsences.mockResolvedValue([]);
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

  it("keeps old and cross-year questions outside the eight-request limit", async () => {
    const currentYear = new Date().getFullYear();
    const oldQuestion: StaffAbsence = {
      ...resubmittedVacation(),
      id: "question-old",
      dateStart: `${currentYear}-02-10`,
      dateEnd: `${currentYear}-02-11`,
      note: "Alte Rückfrage",
      status: "question",
      requestedAt: `${currentYear}-01-01T08:00:00Z`,
    };
    const crossYearQuestion: StaffAbsence = {
      ...oldQuestion,
      id: "question-next-year",
      dateStart: `${currentYear + 1}-02-10`,
      dateEnd: `${currentYear + 1}-02-11`,
      note: "Rückfrage im Folgejahr",
      requestedAt: `${currentYear}-01-02T08:00:00Z`,
    };
    const recentRequests = Array.from({ length: 9 }, (_, index) => ({
      ...resubmittedVacation(),
      id: `recent-${index}`,
      dateStart: `${currentYear}-07-${String(index + 1).padStart(2, "0")}`,
      dateEnd: `${currentYear}-07-${String(index + 1).padStart(2, "0")}`,
      note: `Neuer Antrag ${index + 1}`,
      decisionNote: "",
      requestedAt: `${currentYear}-06-${String(index + 1).padStart(2, "0")}T08:00:00Z`,
    }));
    mocks.getAbsences.mockResolvedValue([...recentRequests, oldQuestion]);
    mocks.getQuestionedAbsences.mockResolvedValue([
      oldQuestion,
      crossYearQuestion,
    ]);

    render(<LeaveRequestsCard />);

    expect(await screen.findByText("Rückfragen")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Alte Rückfrage")).toBeInTheDocument();
    expect(
      screen.getByDisplayValue("Rückfrage im Folgejahr"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", {
        name: "Antwort senden & erneut einreichen",
      }),
    ).toHaveLength(2);
    expect(screen.getAllByRole("listitem")).toHaveLength(10);
    expect(screen.queryByText("Neuer Antrag 1")).not.toBeInTheDocument();
    expect(screen.getByText("Neuer Antrag 9")).toBeInTheDocument();
  });
});
