import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type {
  CarePlanDay,
  CarePlanInstance,
} from "~/lib/student-care-plan-api";

import { CarePlanDayTimeline } from "./care-plan-day";

function instance(over: Partial<CarePlanInstance> = {}): CarePlanInstance {
  return {
    id: "1",
    title: "Mittagessen",
    startTime: "12:00",
    endTime: "13:00",
    roomId: "1",
    status: "planned",
    activeGroupId: null,
    attendance: {
      status: "expected",
      substatus: null,
      note: null,
      checkedInAt: null,
      checkedOutAt: null,
      isUnplanned: false,
    },
    ...over,
  };
}

function day(over: Partial<CarePlanDay> = {}): CarePlanDay {
  return {
    studentId: "1",
    date: "2026-07-22",
    weekday: 3,
    arrival: { expectedTime: "08:00", source: "schedule" },
    instances: [],
    pickup: { expectedTime: "15:30", source: "schedule" },
    ...over,
  };
}

describe("CarePlanDayTimeline", () => {
  it("renders arrival and pickup anchors", () => {
    render(<CarePlanDayTimeline day={day()} deviation={null} />);
    expect(screen.getByText("Ankunft")).toBeInTheDocument();
    expect(screen.getByText("08:00")).toBeInTheDocument();
    expect(screen.getByText("Abholung")).toBeInTheDocument();
    expect(screen.getByText("15:30")).toBeInTheDocument();
  });

  it("renders a Freie Betreuung band for the empty arrival→pickup window", () => {
    render(<CarePlanDayTimeline day={day()} deviation={null} />);
    expect(screen.getByText("Freie Betreuung")).toBeInTheDocument();
    expect(screen.getByText("08:00–15:30")).toBeInTheDocument();
  });

  it("renders an activity block with its time and attendance state", () => {
    render(
      <CarePlanDayTimeline
        day={day({
          instances: [
            instance({
              attendance: {
                status: "present",
                substatus: null,
                note: null,
                checkedInAt: null,
                checkedOutAt: null,
                isUnplanned: false,
              },
            }),
          ],
        })}
        deviation={null}
      />,
    );
    expect(screen.getByText("Mittagessen")).toBeInTheDocument();
    expect(screen.getByText("12:00–13:00")).toBeInTheDocument();
    expect(screen.getByText(/anwesend/)).toBeInTheDocument();
  });

  it("marks a cancelled instance as abgesagt", () => {
    render(
      <CarePlanDayTimeline
        day={day({ instances: [instance({ status: "cancelled" })] })}
        deviation={null}
      />,
    );
    expect(screen.getByText(/abgesagt/)).toBeInTheDocument();
  });

  it("renders a deviation banner above the timeline", () => {
    render(
      <CarePlanDayTimeline
        day={day()}
        deviation={{ kind: "sick", label: "Krank", note: "Fieber" }}
      />,
    );
    expect(screen.getByText("Krank")).toBeInTheDocument();
    expect(screen.getByText(/Fieber/)).toBeInTheDocument();
  });

  it("surfaces an exception arrival as abweichend with its reason", () => {
    render(
      <CarePlanDayTimeline
        day={day({
          arrival: {
            expectedTime: "09:00",
            source: "exception",
            reason: "Arzttermin",
          },
        })}
        deviation={null}
      />,
    );
    expect(screen.getByText("abweichend")).toBeInTheDocument();
    expect(screen.getByText(/Arzttermin/)).toBeInTheDocument();
  });

  it("renders a no-time arrival exception as an absence and suppresses the plan", () => {
    render(
      <CarePlanDayTimeline
        day={day({
          arrival: {
            expectedTime: null,
            source: "exception",
            reason: "krank gemeldet",
          },
          instances: [instance()],
        })}
        deviation={null}
      />,
    );
    expect(screen.getByText("Kommt heute nicht")).toBeInTheDocument();
    expect(screen.getByText(/krank gemeldet/)).toBeInTheDocument();
    // The normal plan is suppressed for an absence day.
    expect(screen.queryByText("Mittagessen")).not.toBeInTheDocument();
    expect(screen.queryByText("Abholung")).not.toBeInTheDocument();
    expect(screen.queryByText("Freie Betreuung")).not.toBeInTheDocument();
  });

  it("treats a no-time pickup exception as an absence too (arrival present)", () => {
    render(
      <CarePlanDayTimeline
        day={day({
          arrival: { expectedTime: "08:00", source: "schedule" },
          pickup: {
            expectedTime: null,
            source: "exception",
            reason: "Krank",
          },
          instances: [instance()],
        })}
        deviation={null}
      />,
    );
    expect(screen.getByText("Kommt heute nicht")).toBeInTheDocument();
    expect(screen.getByText(/Krank/)).toBeInTheDocument();
    // Plan (incl. the arrival anchor) is suppressed for the absence day.
    expect(screen.queryByText("Ankunft")).not.toBeInTheDocument();
    expect(screen.queryByText("Mittagessen")).not.toBeInTheDocument();
  });

  it("lets the deviation banner stand in for the absence notice when both apply", () => {
    render(
      <CarePlanDayTimeline
        day={day({
          arrival: { expectedTime: null, source: "exception" },
          instances: [instance()],
        })}
        deviation={{ kind: "sick", label: "Krank", note: null }}
      />,
    );
    expect(screen.getByText("Krank")).toBeInTheDocument();
    // No duplicate absence notice, and the plan stays suppressed.
    expect(screen.queryByText("Kommt heute nicht")).not.toBeInTheDocument();
    expect(screen.queryByText("Mittagessen")).not.toBeInTheDocument();
  });

  it("shows an empty state when the day has no plan", () => {
    render(
      <CarePlanDayTimeline
        day={day({
          arrival: { expectedTime: null, source: "none" },
          pickup: { expectedTime: null, source: "none" },
        })}
        deviation={null}
      />,
    );
    expect(
      screen.getByText("Kein Betreuungsplan für diesen Tag."),
    ).toBeInTheDocument();
  });

  it("renders null day (no data) as the empty state", () => {
    render(<CarePlanDayTimeline day={null} deviation={null} />);
    expect(
      screen.getByText("Kein Betreuungsplan für diesen Tag."),
    ).toBeInTheDocument();
  });

  it("fires onEditSchedule from the cross-link in the full (non-compact) view", () => {
    const onEditSchedule = vi.fn();
    render(
      <CarePlanDayTimeline
        day={day()}
        deviation={null}
        onEditSchedule={onEditSchedule}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Zeiten bearbeiten" }));
    expect(onEditSchedule).toHaveBeenCalledTimes(1);
  });

  it("hides the cross-link in compact (week-column) mode", () => {
    render(
      <CarePlanDayTimeline
        day={day()}
        deviation={null}
        compact
        onEditSchedule={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("button", { name: "Zeiten bearbeiten" }),
    ).not.toBeInTheDocument();
  });
});
