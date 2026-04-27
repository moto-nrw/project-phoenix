import { describe, expect, it } from "vitest";
import {
  areStudentDayTimesResolved,
  combineTimeNotes,
  getStudentTimeStatus,
  getTimeStatusSortRank,
} from "./student-time-status";
import { LOCATION_COLORS } from "./location-helper";

describe("getStudentTimeStatus", () => {
  const now = new Date("2025-01-15T14:00:00");

  it("returns planned when the event is more than 30 minutes away", () => {
    const status = getStudentTimeStatus({
      plannedTime: "15:00",
      now,
    });

    expect(status.state).toBe("planned");
    expect(status.iconColor).toBe(LOCATION_COLORS.UNKNOWN);
    expect(status.displayTime).toBe("15:00");
  });

  it("returns approaching within the 30 minute window", () => {
    const status = getStudentTimeStatus({
      plannedTime: "14:15",
      now,
    });

    expect(status.state).toBe("approaching");
    expect(status.iconColor).toBe(LOCATION_COLORS.SCHOOLYARD);
    expect(status.textColor).toBeUndefined();
  });

  it("returns slightly overdue when less than 30 minutes late", () => {
    const status = getStudentTimeStatus({
      plannedTime: "13:45",
      now,
    });

    expect(status.state).toBe("slightly-overdue");
    expect(status.textColor).toBe(LOCATION_COLORS.SCHOOLYARD);
    expect(status.detailAnnotation).toBe("Überfällig seit 15 min");
  });

  it("returns very overdue when 30 minutes or more late", () => {
    const status = getStudentTimeStatus({
      plannedTime: "13:30",
      now,
    });

    expect(status.state).toBe("very-overdue");
    expect(status.textColor).toBe(LOCATION_COLORS.HOME);
    expect(status.detailAnnotation).toBe("Überfällig seit 30 min");
  });

  it("returns done-on-time when actual time is equal to or before planned", () => {
    const status = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "13:55",
      now,
    });

    expect(status.state).toBe("done-on-time");
    expect(status.iconColor).toBe(LOCATION_COLORS.GROUP_ROOM);
    expect(status.textColor).toBeUndefined();
    expect(status.detailAnnotation).toBe("-5 min früh");
  });

  it("returns done-late when actual time is after planned", () => {
    const status = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "14:14",
      now,
    });

    expect(status.state).toBe("done-late");
    expect(status.iconColor).toBe(LOCATION_COLORS.SCHOOLYARD);
    expect(status.textColor).toBeUndefined();
    expect(status.detailAnnotation).toBe("+14 min");
  });

  it("returns none when neither planned nor actual time exists", () => {
    const status = getStudentTimeStatus({ now });

    expect(status.state).toBe("none");
    expect(status.displayTime).toBeUndefined();
  });

  it("uses brand colors for every state's icon", () => {
    const veryOverdue = getStudentTimeStatus({ plannedTime: "13:00", now });
    expect(veryOverdue.state).toBe("very-overdue");
    expect(veryOverdue.icon).toBe("warning");
    expect(veryOverdue.iconColor).toBe(LOCATION_COLORS.HOME);
    expect(veryOverdue.textColor).toBe(LOCATION_COLORS.HOME);

    const slightlyOverdue = getStudentTimeStatus({ plannedTime: "13:45", now });
    expect(slightlyOverdue.state).toBe("slightly-overdue");
    expect(slightlyOverdue.icon).toBe("warning");
    expect(slightlyOverdue.iconColor).toBe(LOCATION_COLORS.SCHOOLYARD);
    expect(slightlyOverdue.textColor).toBe(LOCATION_COLORS.SCHOOLYARD);

    const approaching = getStudentTimeStatus({ plannedTime: "14:15", now });
    expect(approaching.state).toBe("approaching");
    expect(approaching.icon).toBe("clock");
    expect(approaching.iconColor).toBe(LOCATION_COLORS.SCHOOLYARD);

    const planned = getStudentTimeStatus({ plannedTime: "15:00", now });
    expect(planned.state).toBe("planned");
    expect(planned.icon).toBe("clock");
    expect(planned.iconColor).toBe(LOCATION_COLORS.UNKNOWN);

    const doneOnTime = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "13:55",
      now,
    });
    expect(doneOnTime.state).toBe("done-on-time");
    expect(doneOnTime.icon).toBe("check");
    expect(doneOnTime.iconColor).toBe(LOCATION_COLORS.GROUP_ROOM);

    const doneLate = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "14:14",
      now,
    });
    expect(doneLate.state).toBe("done-late");
    expect(doneLate.icon).toBe("check");
    expect(doneLate.iconColor).toBe(LOCATION_COLORS.SCHOOLYARD);

    const none = getStudentTimeStatus({ now });
    expect(none.state).toBe("none");
    expect(none.icon).toBe("clock");
    expect(none.iconColor).toBe(LOCATION_COLORS.UNKNOWN);
  });
});

describe("student-time-status helpers", () => {
  it("combines notes and day notes", () => {
    expect(
      combineTimeNotes("Termin", [
        { content: "Früher abholen" },
        { content: "Sportsachen" },
      ]),
    ).toBe("Termin, Früher abholen, Sportsachen");
  });

  it("marks a student closed out only when both rows are resolved", () => {
    const arrival = getStudentTimeStatus({
      plannedTime: "08:00",
      actualTime: "07:58",
      now: new Date("2025-01-15T10:00:00"),
    });
    const pickup = getStudentTimeStatus({
      plannedTime: "15:00",
      actualTime: "15:12",
      now: new Date("2025-01-15T16:00:00"),
    });
    const unresolvedPickup = getStudentTimeStatus({
      plannedTime: "15:00",
      now: new Date("2025-01-15T14:00:00"),
    });

    expect(areStudentDayTimesResolved(arrival, pickup)).toBe(true);
    expect(areStudentDayTimesResolved(arrival, unresolvedPickup)).toBe(false);
  });

  it("sorts the most urgent unresolved states first", () => {
    const veryOverdue = getStudentTimeStatus({
      plannedTime: "13:00",
      now: new Date("2025-01-15T14:00:00"),
    });
    const approaching = getStudentTimeStatus({
      plannedTime: "14:15",
      now: new Date("2025-01-15T14:00:00"),
    });
    const done = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "14:02",
      now: new Date("2025-01-15T14:30:00"),
    });

    expect(getTimeStatusSortRank(veryOverdue)).toBeLessThan(
      getTimeStatusSortRank(approaching),
    );
    expect(getTimeStatusSortRank(approaching)).toBeLessThan(
      getTimeStatusSortRank(done),
    );
  });
});
