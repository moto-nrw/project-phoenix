import { describe, expect, it } from "vitest";

import {
  assignBlockLanes,
  chunkDateRange,
  comparePlanningInstances,
  computeTimetableSetup,
  countPlannedStaff,
  deviationEventLabel,
  formatDayHeader,
  formatMonthLabel,
  formatWeekLabel,
  formatYearLabel,
  getActivityColor,
  getActivityLightTint,
  getActivityTypeBadge,
  getCurrentTimeOffset,
  getEventBlockPosition,
  getGermanWeekdayAdverb,
  getGermanWeekdayLong,
  getGermanWeekdayShort,
  getMonthDays,
  getMonthRange,
  getStatusLabel,
  getWeekRange,
  getWeekdays,
  getYearMonths,
  getYearRange,
  groupInstancesByDate,
  mapApplyDeviations,
  mapMoveStaff,
  mapStaffPool,
  mapAttendance,
  mapConflictCheckResult,
  mapDeviationHistory,
  mapCreateTemplateResult,
  mapGaps,
  mapInstance,
  mapInstanceStatusResult,
  mapMaterializeResult,
  materializedRecurrenceDates,
  mapCombinedOfferingCounts,
  mapOfferingSourceOptions,
  mapReplanWeekResult,
  mapSplitTemplateResult,
  mapStartInstanceResult,
  mapShiftCoverageCheckResult,
  mapTemplates,
  mapWeeklyInstances,
  firstSchoolDayInPeriod,
  nextWorkdayISO,
  offeringPhaseStartWarning,
  parseTimeToMinutes,
  resolveTemplateCalendarPeriodId,
  toISODate,
  weekdayDatesInRange,
} from "./timetable-helpers";
import type { CalendarPeriod } from "./calendar-period-helpers";
import type { EnrichedInstance, TimetableTemplate } from "./timetable-types";

// Minimal helper — assignBlockLanes only inspects startTime/endTime.
function fakeInstance(
  id: string,
  startTime: string,
  endTime: string,
): EnrichedInstance {
  return {
    id,
    date: "2026-04-29",
    startTime,
    endTime,
    title: id,
    description: undefined,
    notes: undefined,
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "activity",
    roomId: "1",
    roomName: "Raum",
    staff: [],
    students: [],
    studentIds: [],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    presentStudentsCount: 0,
    conflictWarnings: [],
  } as unknown as EnrichedInstance;
}

describe("parseTimeToMinutes", () => {
  it("returns minutes since midnight for valid HH:MM", () => {
    expect(parseTimeToMinutes("00:00")).toBe(0);
    expect(parseTimeToMinutes("09:30")).toBe(570);
    expect(parseTimeToMinutes("23:59")).toBe(23 * 60 + 59);
  });

  it("returns NaN for malformed input", () => {
    expect(Number.isNaN(parseTimeToMinutes("not-a-time"))).toBe(true);
    expect(Number.isNaN(parseTimeToMinutes(""))).toBe(true);
    expect(Number.isNaN(parseTimeToMinutes("12:30:45"))).toBe(true);
    expect(Number.isNaN(parseTimeToMinutes("24:00"))).toBe(true);
    expect(Number.isNaN(parseTimeToMinutes("09:60"))).toBe(true);
    expect(Number.isNaN(parseTimeToMinutes("-1:30"))).toBe(true);
  });
});

describe("offeringPhaseStartWarning", () => {
  const offering = {
    phaseName: "Schuljahr 2026/27",
    phaseServiceStart: "2026-08-13",
  };

  it("explains an occurrence before the care service starts", () => {
    expect(offeringPhaseStartWarning(offering, "2026-08-10")).toBe(
      "Die Betreuung aus „Schuljahr 2026/27“ beginnt am 13.08.2026. Termine vor diesem Datum haben noch keine Kinder aus dem Angebot.",
    );
  });

  it("does not warn on the start date or without a usable date", () => {
    expect(offeringPhaseStartWarning(offering, "2026-08-13")).toBeNull();
    expect(offeringPhaseStartWarning(offering, "")).toBeNull();
    expect(offeringPhaseStartWarning(undefined, "2026-08-10")).toBeNull();
  });
});

describe("date and range helpers", () => {
  it("anchors week ranges to Monday through Sunday", () => {
    const { from, to } = getWeekRange(new Date("2026-05-06T12:00:00"), 0);

    expect(toISODate(from)).toBe("2026-05-04");
    expect(toISODate(to)).toBe("2026-05-10");
    expect(getWeekdays(from).map(toISODate)).toEqual([
      "2026-05-04",
      "2026-05-05",
      "2026-05-06",
      "2026-05-07",
      "2026-05-08",
      "2026-05-09",
      "2026-05-10",
    ]);
  });

  it("treats Sunday as part of the current visible week", () => {
    const { from, to } = getWeekRange(new Date("2026-05-10T12:00:00"), 0);

    expect(toISODate(from)).toBe("2026-05-04");
    expect(toISODate(to)).toBe("2026-05-10");
  });

  it("expands a month to full calendar weeks", () => {
    const { from, to } = getMonthRange(new Date("2026-05-15T00:00:00"));
    const days = getMonthDays(new Date("2026-05-15T00:00:00"));

    expect(toISODate(from)).toBe("2026-04-27");
    expect(toISODate(to)).toBe("2026-05-31");
    expect(days).toHaveLength(35);
    expect(days.at(0)).toEqual(from);
    expect(days.at(-1)).toEqual(to);
  });

  it("returns the civil year range and month anchors", () => {
    const { from, to } = getYearRange(new Date("2026-05-15T00:00:00"));

    expect(toISODate(from)).toBe("2026-01-01");
    expect(toISODate(to)).toBe("2026-12-31");
    expect(getYearMonths(from).map((d) => d.getMonth())).toEqual([
      0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
    ]);
  });

  it("chunks inclusive date ranges and rejects invalid ranges", () => {
    expect(chunkDateRange("2026-05-01", "2026-05-05", 2)).toEqual([
      { from: "2026-05-01", to: "2026-05-02" },
      { from: "2026-05-03", to: "2026-05-04" },
      { from: "2026-05-05", to: "2026-05-05" },
    ]);
    expect(chunkDateRange("bad", "2026-05-05", 2)).toEqual([]);
    expect(chunkDateRange("2026-05-06", "2026-05-05", 2)).toEqual([]);
  });

  it("expands selected weekdays across DST without implementing A/B rules", () => {
    expect(weekdayDatesInRange("2026-03-23", "2026-04-08", [1, 3])).toEqual([
      "2026-03-23",
      "2026-03-25",
      "2026-03-30",
      "2026-04-01",
      "2026-04-06",
      "2026-04-08",
    ]);
    expect(weekdayDatesInRange("bad", "2026-04-08", [1])).toEqual([]);
    expect(weekdayDatesInRange("2026-04-09", "2026-04-08", [1])).toEqual([]);
    expect(weekdayDatesInRange("2026-04-06", "2026-04-08", [0, 8])).toEqual([]);
  });

  it("bounds materialized recurrence dates by segment validity and A/B week", () => {
    const period = {
      id: "5",
      startDate: "2026-05-01",
      endDate: "2026-06-30",
      weekCycleLength: 2,
      weekCycleAnchor: "2026-05-04",
    } as CalendarPeriod;

    expect(
      materializedRecurrenceDates({
        period,
        fromISO: "2026-05-01",
        weekdays: [1],
        weekPattern: 1,
        validFrom: "2026-05-11",
        validUntil: "2026-06-01",
      }),
    ).toEqual(["2026-05-18"]);
  });

  it("snaps weekend dates to the following Monday, weekdays pass through", () => {
    expect(nextWorkdayISO("2026-07-18")).toBe("2026-07-20"); // Sa -> Mo
    expect(nextWorkdayISO("2026-07-19")).toBe("2026-07-20"); // So -> Mo
    expect(nextWorkdayISO("2026-07-15")).toBe("2026-07-15"); // Mi bleibt
    expect(nextWorkdayISO("2026-07-20")).toBe("2026-07-20"); // Mo bleibt
    expect(nextWorkdayISO("2026-07-17")).toBe("2026-07-17"); // Fr bleibt
    expect(nextWorkdayISO("2026-08-01")).toBe("2026-08-03"); // Monatswechsel
    expect(nextWorkdayISO("2027-01-03")).toBe("2027-01-04"); // Jahreswechsel
  });

  it("snaps a weekend jump target to the nearest school day inside the period", () => {
    const start = "2026-08-01"; // Samstag
    const end = "2027-07-31";
    // Sa/So am Zeitraumstart -> folgender Montag im Zeitraum.
    expect(firstSchoolDayInPeriod(start, end, "2026-08-01")).toBe("2026-08-03");
    expect(firstSchoolDayInPeriod(start, end, "2026-08-02")).toBe("2026-08-03");
    // Wochentage bleiben unverändert.
    expect(firstSchoolDayInPeriod(start, end, "2026-08-03")).toBe("2026-08-03");
    expect(firstSchoolDayInPeriod(start, end, "2026-08-07")).toBe("2026-08-07");
    // Sa/So am Zeitraumende -> zurück auf den vorangehenden Freitag.
    expect(
      firstSchoolDayInPeriod("2026-08-03", "2026-08-09", "2026-08-08"),
    ).toBe("2026-08-07");
    expect(
      firstSchoolDayInPeriod("2026-08-03", "2026-08-09", "2026-08-09"),
    ).toBe("2026-08-07");
    // Zeitraum umfasst nur das Wochenende -> Ziel bleibt stehen.
    expect(
      firstSchoolDayInPeriod("2026-08-01", "2026-08-02", "2026-08-01"),
    ).toBe("2026-08-01");
  });

  it("formats German labels", () => {
    const monday = new Date("2026-05-04T00:00:00");
    const sunday = new Date("2026-05-10T00:00:00");
    const weekOne2027 = new Date("2027-01-04T00:00:00");
    const weekOneSunday2027 = new Date("2027-01-10T00:00:00");

    expect(formatWeekLabel(monday, sunday)).toContain("KW 19");
    expect(formatWeekLabel(weekOne2027, weekOneSunday2027)).toContain("KW 1");
    expect(formatWeekLabel(monday, sunday)).toContain("04.05");
    expect(formatWeekLabel(monday, sunday)).toContain("10.05.2026");
    expect(formatDayHeader(monday)).toBe("Mo 04.05.");
    expect(formatMonthLabel(monday)).toMatch(/Mai 2026/i);
    expect(formatYearLabel(monday)).toBe("2026");
    expect(getGermanWeekdayLong(monday)).toBe("Montag");
    expect(getGermanWeekdayShort(monday)).toBe("Mo");
  });

  it.each([
    ["Montag", "montags"],
    ["Dienstag", "dienstags"],
    ["Mittwoch", "mittwochs"],
    ["Donnerstag", "donnerstags"],
    ["Freitag", "freitags"],
    ["Samstag", "samstags"],
    ["Sonntag", "sonntags"],
    ["", ""],
  ])("formats %j as the recurring weekday adverb %j", (weekday, expected) => {
    expect(getGermanWeekdayAdverb(weekday)).toBe(expected);
  });
});

describe("activity/status helpers", () => {
  it("resolves brand colors and badges per activity type", () => {
    expect(getActivityColor("care")).toBe("#5080D8");
    expect(getActivityColor("activity")).toBe("#83CD2D");
    expect(getActivityColor("external")).toBe("#F78C10");
    expect(getActivityLightTint("care")).toBe("#EBF0FB");
    expect(getActivityLightTint("activity")).toBe("#ECF7DA");
    expect(getActivityLightTint("external")).toBe("#FCEFD9");
    expect(getActivityTypeBadge("care")).toBeNull();
    expect(getActivityTypeBadge("activity")).toEqual({
      label: "AG",
      bg: "#83CD2D",
    });
    expect(getActivityTypeBadge("external")).toEqual({
      label: "EXTERN",
      bg: "#F78C10",
    });
  });

  it("maps all lifecycle labels", () => {
    expect(getStatusLabel("planned")).toBe("Geplant");
    expect(getStatusLabel("active")).toBe("LÄUFT");
    expect(getStatusLabel("completed")).toBe("Abgeschlossen");
    expect(getStatusLabel("cancelled")).toBe("Abgesagt");
  });
});

describe("backend mappers", () => {
  it("maps deviation history identifiers and nullable display fields", () => {
    expect(deviationEventLabel("cancellation")).toBe("Block abgesagt");
    expect(deviationEventLabel("future_event")).toBe("future_event");

    expect(
      mapDeviationHistory({
        events: [
          {
            id: 42,
            activity_group_id: 7,
            occurrence_date: "2026-05-04",
            start_time: "12:00:00",
            instance_id: 9,
            event_type: "substitution",
            subject_staff_id: 11,
            subject_staff_name: "Ada",
            related_staff_id: 12,
            related_staff_name: "Berta",
            actor_account_id: 13,
            actor_name: "Planung",
            reason: "Krankheit",
            occurred_at: "2026-05-03T12:00:00Z",
          },
          {
            id: 43,
            occurrence_date: "2026-05-05",
            start_time: "13:00:00",
            event_type: "deviation_dropped_by_replan",
            occurred_at: "2026-05-03T13:00:00Z",
          },
        ],
      }),
    ).toEqual({
      events: [
        expect.objectContaining({
          id: "42",
          activityGroupId: "7",
          instanceId: "9",
          subjectStaffId: "11",
          relatedStaffId: "12",
          actorAccountId: "13",
          reason: "Krankheit",
        }),
        expect.objectContaining({
          id: "43",
          activityGroupId: undefined,
          instanceId: undefined,
          subjectStaffId: undefined,
          relatedStaffId: undefined,
          actorAccountId: undefined,
          reason: undefined,
        }),
      ],
    });
    expect(mapDeviationHistory({ events: null }).events).toEqual([]);
  });

  it("maps deviation history old_value/new_value when the backend sends them", () => {
    expect(
      mapDeviationHistory({
        events: [
          {
            id: 44,
            activity_group_id: 7,
            occurrence_date: "2026-05-04",
            start_time: "12:00:00",
            event_type: "substitution",
            old_value: { is_absent: false },
            new_value: { is_absent: true, substitute_staff_id: 12 },
            occurred_at: "2026-05-03T12:00:00Z",
          },
        ],
      }),
    ).toEqual({
      events: [
        expect.objectContaining({
          id: "44",
          oldValue: { is_absent: false },
          newValue: { is_absent: true, substitute_staff_id: 12 },
        }),
      ],
    });
  });

  it("maps deviation history events without old_value/new_value to undefined", () => {
    expect(
      mapDeviationHistory({
        events: [
          {
            id: 45,
            occurrence_date: "2026-05-05",
            start_time: "13:00:00",
            event_type: "deviation_dropped_by_replan",
            occurred_at: "2026-05-03T13:00:00Z",
          },
        ],
      }),
    ).toEqual({
      events: [
        expect.objectContaining({
          id: "45",
          oldValue: undefined,
          newValue: undefined,
        }),
      ],
    });
  });

  it("maps weekly instances, preferring detailed student rows over id fallback", () => {
    const result = mapWeeklyInstances({
      from: "2026-05-04",
      to: "2026-05-08",
      instances: [
        {
          id: 42,
          date: "2026-05-04",
          start_time: "12:00",
          end_time: "13:00",
          title: "Mensa",
          description: "Mittag",
          notes: "ohne Nuesse",
          status: "planned",
          is_spontaneous: false,
          is_live: false,
          activity_group_id: 7,
          activity_type: "care",
          room_id: 3,
          room_name: "Mensa",
          staff: [
            {
              staff_id: 11,
              is_primary: true,
              is_absent: false,
              is_substitute: false,
            },
          ],
          student_ids: [99],
          students: [
            {
              student_id: 21,
              status: "present",
              checked_in_at: "2026-05-04T12:03:00Z",
            },
          ],
          staff_count: 1,
          absent_staff_count: 0,
          expected_students_count: 1,
          present_students_count: 1,
          required_staff_count: 1,
          assigned_staff_count: 1,
          conflict_warnings: [
            {
              kind: "staff",
              resource_id: 3,
              message: "Doppelt",
              can_override: true,
            },
          ],
        },
      ],
    });

    expect(result.instances[0]).toMatchObject({
      id: "42",
      activityGroupId: "7",
      roomId: "3",
      studentIds: ["21"],
      staff: [{ staffId: "11", isPrimary: true }],
      conflictWarnings: [{ resourceId: "3", canOverride: true }],
    });
  });

  it("maps empty optional collections without throwing", () => {
    const result = mapWeeklyInstances({
      from: "2026-05-04",
      to: "2026-05-08",
      instances: [
        {
          id: 43,
          date: "2026-05-04",
          start_time: "14:00",
          end_time: "15:00",
          title: "Spontan",
          status: "planned",
          is_spontaneous: true,
          is_live: false,
          activity_type: "external",
          room_id: 4,
          room_name: "Musik",
          staff: [],
          staff_count: 0,
          absent_staff_count: 0,
          expected_students_count: 0,
          present_students_count: 0,
          required_staff_count: 0,
          assigned_staff_count: 0,
        },
      ],
    });

    expect(result.instances[0]).toMatchObject({
      id: "43",
      activityGroupId: undefined,
      studentIds: [],
      students: [],
      conflictWarnings: [],
    });
  });

  it("maps materialization and re-plan results", () => {
    expect(
      mapMaterializeResult({
        from: "2026-05-04",
        to: "2026-05-08",
        instances_created: 3,
        candidates_skipped_existing: 1,
        warnings: [{ code: "period_missing", message: "keine Periode" }],
        duration_ms: 12,
      }),
    ).toEqual({
      from: "2026-05-04",
      to: "2026-05-08",
      instancesCreated: 3,
      candidatesSkippedExisting: 1,
      warnings: [{ code: "period_missing", message: "keine Periode" }],
      durationMs: 12,
    });

    expect(
      mapReplanWeekResult({
        from: "2026-05-04",
        to: "2026-05-08",
        deleted_instances: 2,
        candidates_skipped_existing: 1,
        instances_created: 3,
        instance_students_created: 4,
        instance_staff_created: 5,
        warnings: [{ code: "staff_missing", message: "Personal fehlt" }],
        duration_ms: 13,
      }),
    ).toMatchObject({
      deletedInstances: 2,
      instancesCreated: 3,
      instanceStudentsCreated: 4,
      instanceStaffCreated: 5,
      warnings: [{ code: "staff_missing" }],
      durationMs: 13,
    });
  });

  it("maps quality, lifecycle, template and attendance DTOs", () => {
    expect(
      mapGaps({
        from: "2026-05-04",
        to: "2026-05-08",
        gaps: [
          {
            instance_id: 42,
            date: "2026-05-04",
            title: "Mensa",
            start_time: "12:00",
            end_time: "13:00",
            room_id: 3,
            status: "planned",
            assigned_staff_count: 1,
            absent_staff_count: 1,
          },
        ],
      }).gaps[0],
    ).toMatchObject({ instanceId: "42", roomId: "3" });

    expect(
      mapStartInstanceResult({
        instance_id: 42,
        status: "active",
        active_group_id: 99,
        started_at: "2026-05-04T12:00:00Z",
        warnings: [
          {
            kind: "staff",
            resource_id: 11,
            message: "Doppelt",
            can_override: true,
          },
        ],
      }),
    ).toMatchObject({ instanceId: "42", activeGroupId: "99" });
    expect(
      mapInstanceStatusResult({
        instance_id: 42,
        status: "completed",
        completed_at: "2026-05-04T13:00:00Z",
      }),
    ).toMatchObject({ instanceId: "42", status: "completed" });
    expect(
      mapCreateTemplateResult({
        template_id: 7,
        timeframe_id: 4,
        schedule_ids: [9],
        instances_created: 1,
      }),
    ).toMatchObject({ templateId: "7", scheduleIds: ["9"] });
    expect(
      mapAttendance({
        id: 100,
        instance_id: 42,
        student_id: 21,
        status: "absent",
        substatus: "sick",
        note: "krank",
      }),
    ).toMatchObject({ id: "100", studentId: "21" });
  });

  it("maps empty backend arrays from optional collections", () => {
    expect(
      mapMaterializeResult({
        from: "2026-05-04",
        to: "2026-05-08",
        instances_created: 0,
        candidates_skipped_existing: 0,
        duration_ms: 1,
      }),
    ).toMatchObject({ warnings: [] });

    expect(
      mapReplanWeekResult({
        from: "2026-05-04",
        to: "2026-05-08",
        deleted_instances: 0,
        candidates_skipped_existing: 0,
        instances_created: 0,
        instance_students_created: 0,
        instance_staff_created: 0,
        duration_ms: 1,
      }),
    ).toMatchObject({ warnings: [] });

    expect(
      mapGaps({
        from: "2026-05-04",
        to: "2026-05-08",
        gaps: undefined as never,
      }),
    ).toMatchObject({ gaps: [] });
  });

  it("maps gap staffing detail and the acknowledged bucket (#1840)", () => {
    const result = mapGaps({
      from: "2026-05-04",
      to: "2026-05-08",
      gaps: [
        {
          instance_id: 42,
          date: "2026-05-04",
          title: "Mensa",
          start_time: "12:00",
          end_time: "13:00",
          room_id: 3,
          status: "planned",
          assigned_staff_count: 2,
          absent_staff_count: 1,
          present_staff_count: 1,
          planned_staff_count: 2,
          understaffed_note: null,
        },
      ],
      acknowledged: [
        {
          instance_id: 43,
          date: "2026-05-04",
          title: "Freispiel",
          start_time: "13:00",
          end_time: "14:00",
          room_id: 4,
          status: "planned",
          assigned_staff_count: 0,
          absent_staff_count: 0,
          present_staff_count: 0,
          planned_staff_count: 1,
          understaffed_note: "bewusst offen",
        },
      ],
    });

    expect(result.gaps[0]).toMatchObject({
      instanceId: "42",
      roomId: "3",
      presentStaffCount: 1,
      plannedStaffCount: 2,
      understaffedNote: undefined,
    });
    expect(result.acknowledged[0]).toMatchObject({
      instanceId: "43",
      roomId: "4",
      understaffedNote: "bewusst offen",
    });
  });

  it("defaults the acknowledged bucket to an empty array (#1840)", () => {
    expect(
      mapGaps({
        from: "2026-05-04",
        to: "2026-05-08",
        gaps: [],
        acknowledged: undefined,
      }),
    ).toMatchObject({ gaps: [], acknowledged: [] });
  });

  it("maps an applied-deviations result with affected instances and warnings (#1840)", () => {
    expect(
      mapApplyDeviations({
        instance_id: 42,
        cancelled: false,
        understaffed_ack: true,
        affected_instances: [
          {
            instance_id: 43,
            title: "Mensa",
            start_time: "12:00",
            action: "marked_absent",
          },
        ],
        warnings: [
          {
            instance_id: 44,
            title: "AG",
            date: "2026-05-04",
            start_time: "14:00",
            end_time: "15:00",
          },
        ],
      }),
    ).toEqual({
      instanceId: "42",
      cancelled: false,
      understaffedAck: true,
      affectedInstances: [
        {
          instanceId: "43",
          title: "Mensa",
          startTime: "12:00",
          action: "marked_absent",
        },
      ],
      warnings: [
        {
          instanceId: "44",
          title: "AG",
          date: "2026-05-04",
          startTime: "14:00",
          endTime: "15:00",
        },
      ],
    });
  });

  it("maps applied-deviations with empty optional collections (#1840)", () => {
    expect(
      mapApplyDeviations({
        instance_id: 42,
        cancelled: true,
        understaffed_ack: false,
        affected_instances: undefined as never,
        warnings: undefined as never,
      }),
    ).toMatchObject({
      instanceId: "42",
      cancelled: true,
      affectedInstances: [],
      warnings: [],
    });
  });

  it("maps deviation fields on an instance: reason, ack and cancel note (#1840)", () => {
    const mapped = mapInstance({
      id: 42,
      date: "2026-05-04",
      start_time: "12:00",
      end_time: "13:00",
      title: "Mensa",
      status: "cancelled",
      is_spontaneous: false,
      is_live: false,
      activity_group_id: 7,
      activity_type: "care",
      room_id: 3,
      room_name: "Mensa",
      staff: [
        {
          staff_id: 11,
          is_primary: true,
          is_absent: true,
          is_substitute: false,
          absence_reason: "krank",
        },
      ],
      students: [],
      staff_count: 1,
      absent_staff_count: 1,
      understaffed_ack: true,
      understaffed_note: "keine Vertretung",
      cancel_reason: "Ausflug",
      expected_students_count: 0,
      present_students_count: 0,
      empty_roster_reason: {
        kind: "before_offering_start",
        phase_name: "Schuljahr 2026/27",
        service_start_date: "2026-08-13",
      },
      required_staff_count: 0,
      assigned_staff_count: 0,
    });

    expect(mapped.staff[0]?.absenceReason).toBe("krank");
    expect(mapped).toMatchObject({
      understaffedAck: true,
      understaffedNote: "keine Vertretung",
      cancelReason: "Ausflug",
      emptyRosterReason: {
        kind: "before_offering_start",
        phaseName: "Schuljahr 2026/27",
        serviceStartDate: "2026-08-13",
      },
    });
  });

  it("maps the Wochennotiz (series_notes) and Tagesnotiz (notes) independently (#1837)", () => {
    const base = {
      id: 42,
      date: "2026-05-04",
      start_time: "14:00",
      end_time: "15:00",
      title: "Betreuung",
      status: "planned" as const,
      is_spontaneous: false,
      is_live: false,
      activity_type: "care" as const,
      room_id: 3,
      room_name: "Aula",
      staff: [],
      students: [],
      staff_count: 0,
      absent_staff_count: 0,
      understaffed_ack: false,
      expected_students_count: 0,
      present_students_count: 0,
      required_staff_count: 0,
      assigned_staff_count: 0,
    };

    const withBoth = mapInstance({
      ...base,
      notes: "Heute ohne Herrn Müller",
      series_notes: "Raum erst ab 14 Uhr offen",
    });
    expect(withBoth.notes).toBe("Heute ohne Herrn Müller");
    expect(withBoth.seriesNotes).toBe("Raum erst ab 14 Uhr offen");

    const withNeither = mapInstance(base);
    expect(withNeither.seriesNotes).toBeUndefined();
  });

  it("maps templates", () => {
    expect(
      mapTemplates({
        templates: [
          {
            id: 7,
            name: "Yoga",
            type: "activity",
            category_id: 2,
            category_name: "AG",
            room_id: 3,
            room_name: "Turnhalle",
            is_open: true,
            max_participants: 12,
            target_group_type: "none",
            enrollment_count: 8,
            supervisor_count: 1,
            required_staff_count: 1,
            assigned_staff_count: 1,
            student_ids: [21],
            staff_ids: [11],
            primary_staff_id: 11,
            protected_student_assignments: [{ weekday: 1, student_ids: [21] }],
            schedules: [
              {
                id: 9,
                weekday: 1,
                start_time: "14:00",
                end_time: "15:00",
                week_pattern: 2,
                calendar_period_id: 5,
                valid_from: "2026-05-04",
                valid_until: "2026-06-01",
              },
            ],
          },
        ],
      }).templates[0],
    ).toMatchObject({
      id: "7",
      roomId: "3",
      studentIds: ["21"],
      staffIds: ["11"],
      primaryStaffId: "11",
      protectedStudentAssignments: [{ weekday: 1, studentIds: ["21"] }],
      schedules: [
        {
          id: "9",
          calendarPeriodId: "5",
          validFrom: "2026-05-04",
          validUntil: "2026-06-01",
        },
      ],
    });
  });

  it("maps the chain-resolved predecessor id onto a template (#2187)", () => {
    const tpl = mapTemplates({
      templates: [
        {
          id: 9,
          name: "Yoga",
          type: "activity",
          category_id: 2,
          category_name: "AG",
          is_open: true,
          max_participants: 12,
          target_group_type: "none",
          enrollment_count: 0,
          supervisor_count: 0,
          required_staff_count: 0,
          assigned_staff_count: 0,
          resolved_from_template_id: 87,
          schedules: [],
        },
      ],
    }).templates[0];

    expect(tpl?.resolvedFromTemplateId).toBe("87");
  });

  it("leaves the chain-resolved id undefined when absent or null (#2187)", () => {
    const base = {
      id: 9,
      name: "Yoga",
      type: "activity" as const,
      category_id: 2,
      category_name: "AG",
      is_open: true,
      max_participants: 12,
      target_group_type: "none" as const,
      enrollment_count: 0,
      supervisor_count: 0,
      required_staff_count: 0,
      assigned_staff_count: 0,
      schedules: [],
    };

    expect(
      mapTemplates({ templates: [base] }).templates[0]?.resolvedFromTemplateId,
    ).toBeUndefined();
    expect(
      mapTemplates({
        templates: [{ ...base, resolved_from_template_id: null }],
      }).templates[0]?.resolvedFromTemplateId,
    ).toBeUndefined();
  });

  it("maps the offering-source rule and per-weekday rosters (#2137/#2129)", () => {
    const tpl = mapTemplates({
      templates: [
        {
          id: 7,
          name: "Frühbetreuung Jg. 1",
          type: "care",
          category_id: 2,
          category_name: "Betreuung",
          is_open: true,
          max_participants: 20,
          target_group_type: "angebot",
          source_care_offering_ids: [12, 15],
          source_grade_levels: [1, 2],
          enrollment_count: 9,
          supervisor_count: 1,
          required_staff_count: 1,
          assigned_staff_count: 1,
          schedules: [],
          weekday_assignments: [
            {
              weekday: 1,
              staff_ids: [11, 12],
              student_ids: [21],
              primary_staff_id: 11,
            },
            // Dienstag ist bewusst leer und ohne feste Bezugsperson.
            { weekday: 2, primary_staff_id: null as never },
          ],
          protected_student_assignments: [
            { weekday: 1, student_ids: [21] },
            { weekday: 2 },
          ],
        },
      ],
    }).templates[0];

    expect(tpl).toMatchObject({
      sourceCareOfferingIds: ["12", "15"],
      sourceGradeLevels: [1, 2],
      weekdayAssignments: [
        {
          weekday: 1,
          staffIds: ["11", "12"],
          studentIds: ["21"],
          primaryStaffId: "11",
        },
        {
          weekday: 2,
          staffIds: [],
          studentIds: [],
          primaryStaffId: undefined,
        },
      ],
      protectedStudentAssignments: [
        { weekday: 1, studentIds: ["21"] },
        { weekday: 2, studentIds: [] },
      ],
    });
  });

  it("leaves the offering source undefined when the backend sends null (#2137)", () => {
    const tpl = mapTemplates({
      templates: [
        {
          id: 8,
          name: "Yoga",
          type: "activity",
          category_id: 2,
          category_name: "AG",
          is_open: true,
          max_participants: 12,
          target_group_type: "none",
          source_care_offering_ids: null as never,
          source_grade_levels: null as never,
          enrollment_count: 0,
          supervisor_count: 0,
          required_staff_count: 0,
          assigned_staff_count: 0,
          schedules: [],
        },
      ],
    }).templates[0];

    expect(tpl?.sourceCareOfferingIds).toBeUndefined();
    expect(tpl?.sourceGradeLevels).toBeUndefined();
  });

  it("maps the Wochennotiz and mapped shift type onto a template (#1837)", () => {
    const tpl = mapTemplates({
      templates: [
        {
          id: 7,
          name: "Betreuung Mo",
          type: "care",
          category_id: 2,
          category_name: "Betreuung",
          is_open: true,
          max_participants: 20,
          target_group_type: "none",
          enrollment_count: 0,
          supervisor_count: 0,
          required_staff_count: 0,
          assigned_staff_count: 0,
          notes: "Raum erst ab 14 Uhr offen",
          shift_type_name: "Betreuung / Zeit am Kind",
          shift_type_color: "#83CD2D",
          schedules: [],
        },
      ],
    }).templates[0];

    expect(tpl?.notes).toBe("Raum erst ab 14 Uhr offen");
    expect(tpl?.shiftTypeName).toBe("Betreuung / Zeit am Kind");
    expect(tpl?.shiftTypeColor).toBe("#83CD2D");
  });

  it("maps optional template fields without fallback ids", () => {
    expect(
      mapTemplates({
        templates: [
          {
            id: 8,
            name: "Offene Betreuung",
            type: "care",
            category_id: 3,
            category_name: "Betreuung",
            is_open: false,
            max_participants: undefined as never,
            target_group_type: "none",
            enrollment_count: undefined as never,
            supervisor_count: 0,
            required_staff_count: 0,
            assigned_staff_count: 0,
            schedules: undefined as never,
          },
        ],
      }).templates[0],
    ).toEqual({
      id: "8",
      name: "Offene Betreuung",
      type: "care",
      categoryId: "3",
      categoryName: "Betreuung",
      roomId: undefined,
      roomName: undefined,
      educationGroupId: undefined,
      educationGroupName: undefined,
      isOpen: false,
      maxParticipants: undefined,
      listKind: undefined,
      notes: undefined,
      shiftTypeName: undefined,
      shiftTypeColor: undefined,
      calendarPeriodId: undefined,
      targetGroupType: "none",
      targetGradeLevel: undefined,
      targetSchoolClass: undefined,
      enrollmentCount: undefined,
      supervisorCount: 0,
      requiredStaffCount: 0,
      assignedStaffCount: 0,
      requiredStaffOverride: undefined,
      studentIds: [],
      staffIds: [],
      primaryStaffId: undefined,
      schedules: [],
      // #2129: a template with no per-weekday deviations maps to an empty list.
      weekdayAssignments: [],
      protectedStudentAssignments: [],
    });
  });
});

describe("required staff override mapping (#1839)", () => {
  it("surfaces a manual instance override, leaving it undefined when derived", () => {
    const withOverride = mapWeeklyInstances({
      from: "2026-05-04",
      to: "2026-05-08",
      instances: [
        {
          id: 42,
          date: "2026-05-04",
          start_time: "14:00",
          end_time: "15:00",
          title: "Schulhof",
          status: "planned",
          is_spontaneous: false,
          is_live: false,
          activity_type: "care",
          room_id: 3,
          room_name: "Hof",
          staff: [],
          staff_count: 1,
          absent_staff_count: 0,
          expected_students_count: 20,
          present_students_count: 20,
          required_staff_count: 3,
          assigned_staff_count: 1,
          required_staff_override: 3,
        },
      ],
    }).instances[0]!;
    expect(withOverride.requiredStaffOverride).toBe(3);

    const derived = mapWeeklyInstances({
      from: "2026-05-04",
      to: "2026-05-08",
      instances: [
        {
          id: 43,
          date: "2026-05-04",
          start_time: "14:00",
          end_time: "15:00",
          title: "Lernzeit",
          status: "planned",
          is_spontaneous: false,
          is_live: false,
          activity_type: "care",
          room_id: 3,
          room_name: "Raum",
          staff: [],
          staff_count: 2,
          absent_staff_count: 0,
          expected_students_count: 10,
          present_students_count: 10,
          required_staff_count: 1,
          assigned_staff_count: 2,
        },
      ],
    }).instances[0]!;
    expect(derived.requiredStaffOverride).toBeUndefined();
  });

  it("passes a template's manual override through mapTemplates", () => {
    const template = mapTemplates({
      templates: [
        {
          id: 7,
          name: "Yoga",
          type: "activity",
          category_id: 2,
          category_name: "AG",
          is_open: true,
          max_participants: 12,
          target_group_type: "none",
          enrollment_count: 8,
          supervisor_count: 1,
          required_staff_count: 4,
          assigned_staff_count: 1,
          required_staff_override: 4,
          schedules: [],
        },
      ],
    }).templates[0]!;
    expect(template.requiredStaffOverride).toBe(4);
    expect(template.requiredStaffCount).toBe(4);
  });
});

describe("resolveTemplateCalendarPeriodId", () => {
  it("prefers the first schedule pin over the template-level pin", () => {
    const candidate = {
      calendarPeriodId: "5",
      schedules: [{ calendarPeriodId: "6" }],
    } as TimetableTemplate;

    expect(resolveTemplateCalendarPeriodId(candidate)).toBe("6");
    expect(
      resolveTemplateCalendarPeriodId({
        ...candidate,
        schedules: [{ calendarPeriodId: undefined }],
      } as TimetableTemplate),
    ).toBe("5");
  });
});

describe("groupInstancesByDate", () => {
  it("groups instances by date while preserving insertion order", () => {
    const a = fakeInstance("a", "10:00", "11:00");
    const b = { ...fakeInstance("b", "11:00", "12:00"), date: "2026-04-30" };
    const c = fakeInstance("c", "12:00", "13:00");

    const grouped = groupInstancesByDate([a, b, c]);

    expect([...grouped.keys()]).toEqual(["2026-04-29", "2026-04-30"]);
    expect(grouped.get("2026-04-29")?.map((i) => i.id)).toEqual(["a", "c"]);
  });
});

describe("getEventBlockPosition", () => {
  it("places a 1h event at the hour line with correct height", () => {
    const { top, height } = getEventBlockPosition("10:00", "11:00", 90, 9);
    expect(top).toBe(90);
    expect(height).toBe(90);
  });

  it("computes fractional positions for half-hour events", () => {
    const { top, height } = getEventBlockPosition("09:30", "10:15", 90, 9);
    expect(top).toBe(45);
    expect(height).toBe(67.5);
  });

  it("clamps tiny events to the minimum height", () => {
    const { height } = getEventBlockPosition("10:00", "10:05", 90, 9);
    expect(height).toBeGreaterThanOrEqual(24);
  });

  it("returns negative top for events before the visible day start", () => {
    const { top } = getEventBlockPosition("08:00", "09:00", 90, 9);
    expect(top).toBe(-90);
  });
});

describe("getCurrentTimeOffset", () => {
  it("returns null when now is outside the visible window", () => {
    const before = new Date("2026-04-29T05:00:00");
    expect(getCurrentTimeOffset(90, 9, 17, before)).toBeNull();

    const after = new Date("2026-04-29T20:00:00");
    expect(getCurrentTimeOffset(90, 9, 17, after)).toBeNull();
  });

  it("returns the correct pixel offset when now is inside the window", () => {
    const noon = new Date("2026-04-29T12:30:00");
    // 12:30 = 3.5h after 09:00 → 3.5 * 90 = 315
    expect(getCurrentTimeOffset(90, 9, 17, noon)).toBe(315);
  });
});

describe("assignBlockLanes", () => {
  it("returns empty array for empty input", () => {
    expect(assignBlockLanes([])).toEqual([]);
  });

  it("gives a single non-overlapping event the full column", () => {
    const result = assignBlockLanes([fakeInstance("a", "10:00", "11:00")]);
    expect(result).toHaveLength(1);
    expect(result[0]!.lane).toBe(0);
    expect(result[0]!.laneCount).toBe(1);
  });

  it("keeps two non-overlapping events in their own clusters at full width", () => {
    const result = assignBlockLanes([
      fakeInstance("a", "10:00", "11:00"),
      fakeInstance("b", "11:00", "12:00"),
    ]);
    expect(result).toHaveLength(2);
    for (const r of result) {
      expect(r.lane).toBe(0);
      expect(r.laneCount).toBe(1);
    }
  });

  it("splits two overlapping events into 2 lanes", () => {
    const result = assignBlockLanes([
      fakeInstance("a", "10:00", "12:00"),
      fakeInstance("b", "11:00", "13:00"),
    ]);
    expect(result).toHaveLength(2);
    const a = result.find((r) => r.instance.id === "a")!;
    const b = result.find((r) => r.instance.id === "b")!;
    expect(a.lane).toBe(0);
    expect(b.lane).toBe(1);
    expect(a.laneCount).toBe(2);
    expect(b.laneCount).toBe(2);
  });

  it("reuses a freed lane when an earlier event has ended within the cluster", () => {
    // a: 10–11   b: 10:30–12   c: 11–12 → c can reuse a's lane (0)
    const result = assignBlockLanes([
      fakeInstance("a", "10:00", "11:00"),
      fakeInstance("b", "10:30", "12:00"),
      fakeInstance("c", "11:00", "12:00"),
    ]);
    const a = result.find((r) => r.instance.id === "a")!;
    const b = result.find((r) => r.instance.id === "b")!;
    const c = result.find((r) => r.instance.id === "c")!;
    expect(a.lane).toBe(0);
    expect(b.lane).toBe(1);
    expect(c.lane).toBe(0);
    // Cluster size is 2 (max parallel at any time), all members agree.
    expect(a.laneCount).toBe(2);
    expect(b.laneCount).toBe(2);
    expect(c.laneCount).toBe(2);
  });
});

describe("comparePlanningInstances", () => {
  it("orders equal start times by planning track and then id", () => {
    const first = fakeInstance("2", "10:00", "11:00");
    const second = fakeInstance("10", "10:00", "11:00");
    first.planningTrackSortOrder = 1;
    second.planningTrackSortOrder = 2;
    expect(comparePlanningInstances(first, second)).toBeLessThan(0);

    first.planningTrackSortOrder = undefined;
    expect(comparePlanningInstances(first, second)).toBeGreaterThan(0);

    first.planningTrackSortOrder = 1;
    second.planningTrackSortOrder = undefined;
    expect(comparePlanningInstances(first, second)).toBeLessThan(0);

    first.planningTrackSortOrder = undefined;
    expect(comparePlanningInstances(first, second)).toBeLessThan(0);
  });
});

describe("mapSplitTemplateResult", () => {
  it("maps snake_case ints to camelCase strings", () => {
    expect(
      mapSplitTemplateResult({
        old_template_id: 7,
        new_template_id: 8,
        schedule_ids: [11, 12],
        deleted_instances: 4,
        instances_created: 6,
      }),
    ).toEqual({
      oldTemplateId: "7",
      newTemplateId: "8",
      scheduleIds: ["11", "12"],
      deletedInstances: 4,
      instancesCreated: 6,
    });
  });
});

describe("mapConflictCheckResult", () => {
  it("maps warnings and defaults missing warnings to []", () => {
    expect(
      mapConflictCheckResult({
        date: "2026-05-04",
        start_time: "12:00",
        end_time: "13:00",
        warnings: [
          {
            kind: "staff",
            resource_id: 11,
            message: "Person doppelt eingeplant",
            conflicting_instance_id: 42,
            conflicting_title: "Mensa",
          },
        ],
      }),
    ).toEqual({
      date: "2026-05-04",
      startTime: "12:00",
      endTime: "13:00",
      warnings: [
        {
          kind: "staff",
          resourceId: "11",
          message: "Person doppelt eingeplant",
          conflictingInstanceId: "42",
          conflictingTitle: "Mensa",
        },
      ],
    });

    expect(
      mapConflictCheckResult({
        date: "2026-05-04",
        start_time: "12:00",
        end_time: "13:00",
      }),
    ).toEqual({
      date: "2026-05-04",
      startTime: "12:00",
      endTime: "13:00",
      warnings: [],
    });
  });
});

describe("mapShiftCoverageCheckResult", () => {
  it("maps exact gap warnings and keeps the array non-null", () => {
    expect(
      mapShiftCoverageCheckResult({
        coverage_warnings: [
          {
            staff_id: 12,
            staff_name: "Ada Lovelace",
            date: "2026-05-06",
            start_time: "12:00",
            end_time: "13:00",
            uncovered_start_time: "12:30",
            uncovered_end_time: "13:00",
            message: "Mittwoch ist nicht vollständig abgedeckt.",
          },
        ],
      }),
    ).toEqual({
      coverageWarningCount: 1,
      coverageWarnings: [
        {
          staffId: "12",
          staffName: "Ada Lovelace",
          date: "2026-05-06",
          startTime: "12:00",
          endTime: "13:00",
          uncoveredStartTime: "12:30",
          uncoveredEndTime: "13:00",
          message: "Mittwoch ist nicht vollständig abgedeckt.",
        },
      ],
    });
    expect(mapShiftCoverageCheckResult({})).toEqual({
      coverageWarnings: [],
      coverageWarningCount: 0,
    });
  });
});

function planInstance(
  overrides: Partial<EnrichedInstance> = {},
): EnrichedInstance {
  return {
    id: "i1",
    date: "2026-06-15",
    startTime: "09:00",
    endTime: "10:00",
    title: "Mensa",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "care",
    roomId: "1",
    roomName: "Raum",
    staff: [],
    students: [],
    studentIds: [],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    presentStudentsCount: 0,
    conflictWarnings: [],
    ...overrides,
  } as unknown as EnrichedInstance;
}

describe("countPlannedStaff", () => {
  function staffMember(
    staffId: string,
    isAbsent = false,
  ): EnrichedInstance["staff"][number] {
    return { staffId, isPrimary: false, isAbsent, isSubstitute: false };
  }

  it("unions staff across blocks so a person in two blocks counts once", () => {
    expect(
      countPlannedStaff([
        planInstance({
          id: "a",
          staff: [staffMember("s1"), staffMember("s2")],
        }),
        planInstance({
          id: "b",
          staff: [staffMember("s2"), staffMember("s3")],
        }),
      ]),
    ).toBe(3);
  });

  it("excludes absent staff", () => {
    expect(
      countPlannedStaff([
        planInstance({
          id: "a",
          staff: [staffMember("s1", true), staffMember("s2")],
        }),
      ]),
    ).toBe(1);
  });

  it("excludes staff of cancelled instances", () => {
    expect(
      countPlannedStaff([
        planInstance({
          id: "a",
          status: "cancelled",
          staff: [staffMember("s1")],
        }),
      ]),
    ).toBe(0);
  });

  it("returns 0 for an empty list", () => {
    expect(countPlannedStaff([])).toBe(0);
  });
});

describe("computeTimetableSetup", () => {
  it("completes when period + plan are done (enrollment optional)", () => {
    const result = computeTimetableSetup({
      hasActivePeriod: true,
      careOfferingLink: "unlinked",
      hasPlan: true,
    });
    expect(result.setupComplete).toBe(true);
    expect(result.periodDone).toBe(true);
    expect(result.planDone).toBe(true);
    expect(result.totalSteps).toBe(3);
    expect(result.completedSteps).toBe(2);
    expect(result.progressPercent).toBe(67);
  });

  it("is incomplete for a fresh school (no period, no plan)", () => {
    const result = computeTimetableSetup({
      hasActivePeriod: false,
      careOfferingLink: "unlinked",
      hasPlan: false,
    });
    expect(result.setupComplete).toBe(false);
    expect(result.completedSteps).toBe(0);
    expect(result.progressPercent).toBe(0);
  });

  it("counts a linked care offering toward progress", () => {
    const result = computeTimetableSetup({
      hasActivePeriod: true,
      careOfferingLink: "linked",
      hasPlan: true,
    });
    expect(result.enrollmentDone).toBe(true);
    expect(result.completedSteps).toBe(3);
    expect(result.progressPercent).toBe(100);
  });

  // Issue #1651: an active enrollment phase whose offerings link to nothing
  // must not tick the "Mit der Anmeldung verknüpfen" step.
  it("leaves the enrollment step open when no care offering is linked", () => {
    const result = computeTimetableSetup({
      hasActivePeriod: true,
      careOfferingLink: "unlinked",
      hasPlan: true,
    });
    expect(result.enrollmentApplicable).toBe(true);
    expect(result.enrollmentDone).toBe(false);
    expect(result.completedSteps).toBe(2);
  });

  it("drops the enrollment step from progress when the linkage is unknown", () => {
    const result = computeTimetableSetup({
      hasActivePeriod: true,
      careOfferingLink: "unknown",
      hasPlan: false,
    });
    expect(result.enrollmentApplicable).toBe(false);
    expect(result.totalSteps).toBe(2);
    expect(result.completedSteps).toBe(1);
    expect(result.progressPercent).toBe(50);
  });
});

describe("staff pool + move mappers (#1884)", () => {
  it("labels the new deviation event types", () => {
    expect(deviationEventLabel("staff_moved")).toBe("Person verschoben");
    expect(deviationEventLabel("shift_moved")).toBe("Schicht verschoben");
  });

  it("maps a staff pool response including null collections", () => {
    expect(
      mapStaffPool({
        instance_id: 42,
        title: "Mensa",
        date: "2026-05-04",
        start_time: "12:30",
        end_time: "13:30",
        dienstplan_in_use: true,
        entries: [
          {
            staff_id: 7,
            display_name: "Ina Umzieherin",
            category: "assigned_elsewhere",
            on_shift: true,
            covers_window: true,
            shift_windows: ["08:00–16:00"],
            absence_reason: null,
            assignments: [
              {
                instance_id: 41,
                title: "Schulhof",
                start_time: "12:00",
                end_time: "14:00",
                is_substitute: false,
              },
            ],
          },
          {
            staff_id: 8,
            display_name: "Frido Verfuegbar",
            category: "on_shift_free",
            on_shift: true,
            covers_window: false,
            shift_windows: null,
            assignments: null,
          },
        ],
      }),
    ).toEqual({
      instanceId: "42",
      title: "Mensa",
      date: "2026-05-04",
      startTime: "12:30",
      endTime: "13:30",
      dienstplanInUse: true,
      entries: [
        {
          staffId: "7",
          displayName: "Ina Umzieherin",
          category: "assigned_elsewhere",
          onShift: true,
          coversWindow: true,
          shiftWindows: ["08:00–16:00"],
          absenceReason: undefined,
          assignments: [
            {
              instanceId: "41",
              title: "Schulhof",
              startTime: "12:00",
              endTime: "14:00",
              isSubstitute: false,
            },
          ],
        },
        {
          staffId: "8",
          displayName: "Frido Verfuegbar",
          category: "on_shift_free",
          onShift: true,
          coversWindow: false,
          shiftWindows: [],
          absenceReason: undefined,
          assignments: [],
        },
      ],
    });
  });

  it("maps a move result with advisory warnings", () => {
    expect(
      mapMoveStaff({
        target_instance_id: 42,
        source_instance_id: 41,
        action: "moved",
        time_conflicts: [
          {
            instance_id: 44,
            title: "AG",
            date: "2026-05-04",
            start_time: "13:00",
            end_time: "14:00",
          },
        ],
        coverage_warnings: [
          {
            staff_id: 7,
            staff_name: "Ina Umzieherin",
            date: "2026-05-04",
            start_time: "12:30",
            end_time: "13:30",
            uncovered_start_time: "13:00",
            uncovered_end_time: "13:30",
            message: "Keine Schicht deckt 13:00–13:30 ab",
          },
        ],
      }),
    ).toEqual({
      targetInstanceId: "42",
      sourceInstanceId: "41",
      action: "moved",
      timeConflicts: [
        {
          instanceId: "44",
          title: "AG",
          date: "2026-05-04",
          startTime: "13:00",
          endTime: "14:00",
        },
      ],
      coverageWarnings: [
        {
          staffId: "7",
          staffName: "Ina Umzieherin",
          date: "2026-05-04",
          startTime: "12:30",
          endTime: "13:30",
          uncoveredStartTime: "13:00",
          uncoveredEndTime: "13:30",
          message: "Keine Schicht deckt 13:00–13:30 ab",
        },
      ],
    });
  });

  it("maps a pool assign without source and defaults null warning lists", () => {
    expect(
      mapMoveStaff({
        target_instance_id: 42,
        source_instance_id: null,
        action: "assigned",
        time_conflicts: null,
        coverage_warnings: null,
      }),
    ).toEqual({
      targetInstanceId: "42",
      sourceInstanceId: undefined,
      action: "assigned",
      timeConflicts: [],
      coverageWarnings: [],
    });
  });
});

describe("mapOfferingSourceOptions (#2137)", () => {
  it("maps offerings, numeric Jahrgang keys and the legacy link", () => {
    expect(
      mapOfferingSourceOptions({
        offerings: [
          {
            id: 12,
            name: "Frühbetreuung",
            phase_id: 3,
            phase_name: "Schuljahr 2026/27",
            phase_service_start: "2026-08-13",
            total_count: 18,
            // "0" = Kinder ohne ableitbaren Jahrgang.
            grade_counts: { "0": 2, "1": 9, "2": 7 },
            sourced_templates: [
              { id: 7, name: "Frühbetreuung Jg. 1", grade_levels: [1] },
              { id: 8, name: "Frühbetreuung alle" },
            ],
            legacy_linked_template_id: 7,
          },
        ],
      }),
    ).toEqual([
      {
        id: "12",
        name: "Frühbetreuung",
        phaseId: "3",
        phaseName: "Schuljahr 2026/27",
        phaseServiceStart: "2026-08-13",
        totalCount: 18,
        gradeCounts: { 0: 2, 1: 9, 2: 7 },
        sourcedTemplates: [
          { id: "7", name: "Frühbetreuung Jg. 1", gradeLevels: [1] },
          { id: "8", name: "Frühbetreuung alle", gradeLevels: [] },
        ],
        legacyLinkedTemplateId: "7",
      },
    ]);
  });

  it("skips unparsable Jahrgang keys and defaults the optional fields", () => {
    expect(
      mapOfferingSourceOptions({
        offerings: [
          {
            id: 13,
            name: "Spätbetreuung",
            phase_id: 3,
            phase_name: "Schuljahr 2026/27",
            total_count: 0,
            grade_counts: { unbekannt: 4, "3": 1 } as never,
            sourced_templates: undefined as never,
            legacy_linked_template_id: null as never,
          },
        ],
      }),
    ).toEqual([
      {
        id: "13",
        name: "Spätbetreuung",
        phaseId: "3",
        phaseName: "Schuljahr 2026/27",
        totalCount: 0,
        gradeCounts: { 3: 1 },
        sourcedTemplates: [],
        legacyLinkedTemplateId: undefined,
      },
    ]);
  });

  it("maps a payload without offerings and without counts to an empty list", () => {
    expect(mapOfferingSourceOptions({ offerings: undefined as never })).toEqual(
      [],
    );
    expect(
      mapOfferingSourceOptions({
        offerings: [
          {
            id: 14,
            name: "Ferienbetreuung",
            phase_id: 4,
            phase_name: "Ferien",
            total_count: 0,
            grade_counts: undefined as never,
            sourced_templates: [],
          },
        ],
      })[0]?.gradeCounts,
    ).toEqual({});
  });
});

describe("mapCombinedOfferingCounts (Mehrfach-Quelle)", () => {
  it("maps string grade keys to numbers and keeps the deduplicated total", () => {
    expect(
      mapCombinedOfferingCounts({
        total_count: 18,
        grade_counts: { "0": 2, "1": 9, "2": 7 },
      }),
    ).toEqual({
      totalCount: 18,
      gradeCounts: { 0: 2, 1: 9, 2: 7 },
    });
  });

  it("tolerates missing counts", () => {
    expect(
      mapCombinedOfferingCounts({
        total_count: undefined as never,
        grade_counts: undefined as never,
      }),
    ).toEqual({ totalCount: 0, gradeCounts: {} });
  });
});
