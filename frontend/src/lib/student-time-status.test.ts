import { describe, expect, it } from "vitest";
import {
  areStudentDayTimesResolved,
  combineTimeNotes,
  getStudentAbsence,
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
    expect(status.textColor).toBe(LOCATION_COLORS.DANGER);
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
    expect(veryOverdue.iconColor).toBe(LOCATION_COLORS.DANGER);
    expect(veryOverdue.textColor).toBe(LOCATION_COLORS.DANGER);

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

describe("getStudentTimeStatus edge cases", () => {
  const now = new Date("2025-01-15T14:00:00");

  it("treats actual time before planned as done-on-time and reports negative diff", () => {
    // Boundary: diff of exactly 0 must NOT be classified as 'done-late' —
    // otherwise an on-time arrival would render with the late color.
    const status = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "14:00",
      now,
    });

    expect(status.state).toBe("done-on-time");
    expect(status.diffMinutes).toBe(0);
    // 0-min diff has no annotation — would render as "+0 min" otherwise.
    expect(status.detailAnnotation).toBeUndefined();
  });

  it("classifies a 31-minute lead as planned, not approaching", () => {
    // The threshold is 30 minutes; 31 min must fall on the 'planned' side.
    const status = getStudentTimeStatus({
      plannedTime: "14:31",
      now,
    });

    expect(status.state).toBe("planned");
  });

  it("classifies a 30-minute lead as approaching (inclusive boundary)", () => {
    const status = getStudentTimeStatus({
      plannedTime: "14:30",
      now,
    });

    expect(status.state).toBe("approaching");
  });

  it("classifies exactly-30-min late as very-overdue (exclusive boundary)", () => {
    // diff = -30. The slightly-overdue branch is `diff > -30`, so -30 must
    // tip into very-overdue.
    const status = getStudentTimeStatus({
      plannedTime: "13:30",
      now,
    });

    expect(status.state).toBe("very-overdue");
  });

  it("ignores actualTime that has no displayable form and falls through to planned", () => {
    // Empty actualTime string must be treated as 'no actual yet', not as
    // 'resolved with empty value' — otherwise the card would dim out.
    const status = getStudentTimeStatus({
      plannedTime: "14:30",
      actualTime: "",
      now,
    });

    expect(status.state).toBe("approaching");
    expect(status.displayTime).toBe("14:30");
    expect(status.isResolved).toBe(false);
  });

  it("trims HH:MM:SS planned input down to HH:MM for display", () => {
    const status = getStudentTimeStatus({
      plannedTime: "15:00:00",
      now,
    });

    expect(status.displayTime).toBe("15:00");
  });

  it("computes done-late diff correctly when actual is hours after planned", () => {
    const status = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "15:30",
      now,
    });

    expect(status.state).toBe("done-late");
    expect(status.diffMinutes).toBe(90);
    expect(status.detailAnnotation).toBe("+90 min");
  });
});

describe("getStudentTimeStatus sick/excused", () => {
  // now is well past the planned time, so without the sick/excused signal
  // these inputs would all classify as very-overdue (red).
  const now = new Date("2025-01-15T15:00:00");

  it("neutralizes an overdue planned arrival when the student is sick", () => {
    const status = getStudentTimeStatus({
      plannedTime: "08:00",
      now,
      sick: true,
    });

    expect(status.state).toBe("absent-excused");
    // No red overdue color — that is the whole point of the fix.
    expect(status.textColor).toBeUndefined();
    expect(status.iconColor).toBe(LOCATION_COLORS.UNKNOWN);
    // A sick child is closed out for the day, not an open urgency.
    expect(status.isResolved).toBe(true);
    expect(status.detailAnnotation).not.toContain("Überfällig");
  });

  it("neutralizes an overdue planned pickup when the student is excused", () => {
    const status = getStudentTimeStatus({
      plannedTime: "13:00",
      now,
      excused: true,
    });

    expect(status.state).toBe("absent-excused");
    expect(status.textColor).toBeUndefined();
  });

  it("lets a recorded actual time win over a stale sick flag (precedence)", () => {
    // A child marked sick who nevertheless checked in (RFID) must show the
    // real arrival, not 'absent'. actual time always wins.
    const status = getStudentTimeStatus({
      plannedTime: "08:00",
      actualTime: "08:05",
      now,
      sick: true,
    });

    expect(status.state).toBe("done-late");
    expect(status.displayTime).toBe("08:05");
  });

  it("ignores sick/excused when neither planned nor actual time exists", () => {
    const status = getStudentTimeStatus({ now, sick: true });

    // Nothing to neutralize — without a time there is no red to suppress.
    expect(status.state).toBe("absent-excused");
  });

  it("ranks absent-excused out of the urgency band but above 'none'", () => {
    const absentExcused = getStudentTimeStatus({
      plannedTime: "08:00",
      now,
      sick: true,
    });
    const veryOverdue = getStudentTimeStatus({ plannedTime: "08:00", now });
    const approaching = getStudentTimeStatus({ plannedTime: "15:15", now });
    const planned = getStudentTimeStatus({ plannedTime: "16:00", now });
    const none = getStudentTimeStatus({ now });

    expect(getTimeStatusSortRank(absentExcused)).toBeGreaterThan(
      getTimeStatusSortRank(veryOverdue),
    );
    expect(getTimeStatusSortRank(absentExcused)).toBeGreaterThan(
      getTimeStatusSortRank(approaching),
    );
    expect(getTimeStatusSortRank(absentExcused)).toBeGreaterThan(
      getTimeStatusSortRank(planned),
    );
    // Must not change the existing contract: 'none' stays last.
    expect(getTimeStatusSortRank(none)).toBe(6);
    expect(getTimeStatusSortRank(absentExcused)).toBeLessThan(
      getTimeStatusSortRank(none),
    );
  });
});

describe("getStudentAbsence", () => {
  it("returns null when the student is neither sick nor excused", () => {
    expect(getStudentAbsence({})).toBeNull();
    expect(getStudentAbsence({ sick: false, excused: false })).toBeNull();
  });

  it("labels a sick student without any date qualifier", () => {
    // No "seit <weekday>" — a weekday abbreviation is ambiguous past 7 days
    // and outright wrong past two weeks, so we keep a stable, date-free label.
    const absence = getStudentAbsence({ sick: true });

    expect(absence).not.toBeNull();
    expect(absence?.reason).toBe("sick");
    expect(absence?.label).toBe("krank gemeldet");
  });

  it("labels an excused student", () => {
    const absence = getStudentAbsence({ excused: true });

    expect(absence?.reason).toBe("excused");
    expect(absence?.label).toBe("entschuldigt");
  });

  it("prefers sick over excused when both are set", () => {
    const absence = getStudentAbsence({ sick: true, excused: true });

    expect(absence?.reason).toBe("sick");
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

  it("returns undefined when both notes and day notes are empty", () => {
    // Caller passes the result straight into JSX — an empty string would
    // render an extra row with no content, undefined skips the row entirely.
    expect(combineTimeNotes(undefined, undefined)).toBeUndefined();
    expect(combineTimeNotes("", [])).toBeUndefined();
  });

  it("filters falsy day note content but keeps the rest", () => {
    expect(
      combineTimeNotes(undefined, [{ content: "" }, { content: "Bus" }]),
    ).toBe("Bus");
  });

  it("ranks the 'none' state last so unresolved students still appear above closed ones", () => {
    // Sorting contract: 'none' (no times configured at all) should never
    // outrank a real status. If it did, students with no schedule would
    // bubble to the top of an urgency-sorted list.
    const none = getStudentTimeStatus({ now: new Date() });
    const planned = getStudentTimeStatus({
      plannedTime: "15:00",
      now: new Date("2025-01-15T08:00:00"),
    });
    const doneOnTime = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "13:55",
      now: new Date("2025-01-15T16:00:00"),
    });

    expect(getTimeStatusSortRank(none)).toBe(6);
    expect(getTimeStatusSortRank(planned)).toBeLessThan(
      getTimeStatusSortRank(none),
    );
    expect(getTimeStatusSortRank(doneOnTime)).toBeLessThan(
      getTimeStatusSortRank(none),
    );
  });

  it("ranks done-late ahead of done-on-time so flagged completions float up", () => {
    const doneLate = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "14:20",
      now: new Date("2025-01-15T15:00:00"),
    });
    const doneOnTime = getStudentTimeStatus({
      plannedTime: "14:00",
      actualTime: "13:55",
      now: new Date("2025-01-15T15:00:00"),
    });

    expect(getTimeStatusSortRank(doneLate)).toBeLessThan(
      getTimeStatusSortRank(doneOnTime),
    );
  });

  it("treats class trip as a resolved absence and keeps absence priority explicit", () => {
    const status = getStudentTimeStatus({
      plannedTime: "14:00",
      classTrip: true,
      now: new Date("2025-01-15T15:00:00"),
    });

    expect(status).toEqual(
      expect.objectContaining({
        state: "absent-excused",
        icon: "clock",
        iconColor: LOCATION_COLORS.UNKNOWN,
        isResolved: true,
      }),
    );
    expect(getStudentAbsence({ classTrip: true })).toEqual({
      reason: "class_trip",
      label: "Klassenfahrt",
    });
    expect(
      getStudentAbsence({ sick: true, classTrip: true, excused: true }),
    ).toEqual({ reason: "sick", label: "krank gemeldet" });
    expect(getStudentAbsence({ classTrip: true, excused: true })).toEqual({
      reason: "class_trip",
      label: "Klassenfahrt",
    });
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
