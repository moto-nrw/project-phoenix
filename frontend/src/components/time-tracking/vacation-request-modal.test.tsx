import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import type { DateRange, Matcher } from "react-day-picker";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ABSENCES_REFRESH_EVENT } from "~/lib/absence-helpers";
import type { StaffAbsence } from "~/lib/time-tracking-helpers";

import { VacationRequestModal } from "./vacation-request-modal";

interface CapturedCalendarProps {
  readonly onChange: (range: DateRange | undefined) => void;
  readonly modifiers?: Record<string, Matcher | Matcher[]>;
  readonly modifiersClassNames?: Record<string, string>;
}

const mocks = vi.hoisted(() => ({
  calendarProps: null as CapturedCalendarProps | null,
  requestVacation: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}));

vi.mock("~/components/ui/date-range-picker", () => ({
  RangeCalendarInline: (props: CapturedCalendarProps) => {
    mocks.calendarProps = props;
    return (
      <button
        type="button"
        onClick={() =>
          props.onChange({
            from: new Date(2027, 6, 10),
            to: new Date(2027, 6, 11),
          })
        }
      >
        Test-Zeitraum wählen
      </button>
    );
  },
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    children,
    footer,
  }: {
    readonly isOpen: boolean;
    readonly children: ReactNode;
    readonly footer?: ReactNode;
  }) =>
    isOpen ? (
      <div>
        {children}
        {footer}
      </div>
    ) : null,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    error: mocks.toastError,
    success: mocks.toastSuccess,
  }),
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: {
    requestVacation: mocks.requestVacation,
  },
}));

function questionedVacation(): StaffAbsence {
  return {
    id: "17",
    staffId: "42",
    absenceType: "vacation",
    dateStart: "2027-07-10",
    dateEnd: "2027-07-12",
    halfDay: false,
    startHalfDay: false,
    endHalfDay: false,
    note: "Sommerurlaub",
    status: "question",
    approvedBy: null,
    approvedAt: null,
    createdBy: "42",
    createdAt: "2027-06-01T08:00:00Z",
    updatedAt: "2027-06-02T08:00:00Z",
    durationDays: 3,
    workingDays: 1,
    decisionNote: "Bitte Vertretung klären",
    requestedAt: "2027-06-01T08:00:00Z",
    substituteStaffId: null,
  };
}

describe("VacationRequestModal questioned absences", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date(2027, 6, 1, 10, 0, 0));
    mocks.calendarProps = null;
    mocks.requestVacation.mockReset();
    mocks.toastError.mockReset();
    mocks.toastSuccess.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("marks questioned vacation dates with their own calendar modifier", () => {
    render(
      <VacationRequestModal
        isOpen
        onClose={vi.fn()}
        onSubmitted={vi.fn()}
        remainingDays={20}
        existingVacations={[questionedVacation()]}
      />,
    );

    expect(mocks.calendarProps?.modifiers?.questionVacation).toEqual([
      {
        from: new Date(2027, 6, 10),
        to: new Date(2027, 6, 12),
      },
    ]);
    expect(
      mocks.calendarProps?.modifiersClassNames?.questionVacation,
    ).toContain("#7C3AED");
    expect(screen.getByText("Rückfrage")).toBeInTheDocument();
  });

  it("blocks a range that overlaps a questioned vacation", () => {
    render(
      <VacationRequestModal
        isOpen
        onClose={vi.fn()}
        onSubmitted={vi.fn()}
        remainingDays={20}
        existingVacations={[questionedVacation()]}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Test-Zeitraum wählen" }),
    );

    expect(
      screen.getByText(
        /Dieser Zeitraum überschneidet sich mit Urlaub vom .* \(Rückfrage\)\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Antrag senden" }),
    ).toBeDisabled();
    expect(mocks.requestVacation).not.toHaveBeenCalled();
  });

  it("refreshes the approver badge after a successful request", async () => {
    mocks.requestVacation.mockResolvedValue({});
    const listener = vi.fn();
    window.addEventListener(ABSENCES_REFRESH_EVENT, listener);

    render(
      <VacationRequestModal
        isOpen
        onClose={vi.fn()}
        onSubmitted={vi.fn()}
        remainingDays={20}
        existingVacations={[]}
      />,
    );

    act(() => {
      mocks.calendarProps?.onChange({
        from: new Date(2027, 6, 5),
        to: new Date(2027, 6, 6),
      });
    });
    fireEvent.click(screen.getByRole("button", { name: "Antrag senden" }));

    await waitFor(() => {
      expect(mocks.requestVacation).toHaveBeenCalledTimes(1);
      expect(listener).toHaveBeenCalledTimes(1);
    });
    window.removeEventListener(ABSENCES_REFRESH_EVENT, listener);
  });
});
