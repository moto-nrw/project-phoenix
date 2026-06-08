import { describe, expect, it } from "vitest";

import {
  assignBlockLanes,
  chunkDateRange,
  formatDayHeader,
  formatMonthLabel,
  formatWeekLabel,
  formatYearLabel,
  getActivityColor,
  getActivityLightTint,
  getActivityTypeBadge,
  getCurrentTimeOffset,
  getEventBlockPosition,
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
  mapAttendance,
  mapCreateTemplateResult,
  mapExceptionConflicts,
  mapGaps,
  mapInstanceStatusResult,
  mapMaterializeResult,
  mapReplanWeekResult,
  mapStartInstanceResult,
  mapSubstitute,
  mapTemplates,
  mapWeeklyInstances,
  parseTimeToMinutes,
  toISODate,
} from "./timetable-helpers";
import type { EnrichedInstance } from "./timetable-types";

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

  it("formats German labels", () => {
    const monday = new Date("2026-05-04T00:00:00");
    const sunday = new Date("2026-05-10T00:00:00");
    const weekOne2027 = new Date("2027-01-04T00:00:00");
    const weekOneSunday2027 = new Date("2027-01-10T00:00:00");

    expect(formatWeekLabel(monday, sunday)).toContain("KW 19");
    expect(formatWeekLabel(weekOne2027, weekOneSunday2027)).toContain("KW 1");
    expect(formatWeekLabel(monday, sunday)).toContain("04.05");
    expect(formatWeekLabel(monday, sunday)).toContain("So 10.05.2026");
    expect(formatDayHeader(monday)).toBe("Mo 04.05.");
    expect(formatMonthLabel(monday)).toMatch(/Mai 2026/i);
    expect(formatYearLabel(monday)).toBe("2026");
    expect(getGermanWeekdayLong(monday)).toBe("Montag");
    expect(getGermanWeekdayShort(monday)).toBe("Mo");
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
          conflict_warnings: [
            {
              kind: "room",
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
      mapExceptionConflicts({
        from: "2026-05-04",
        to: "2026-05-08",
        conflicts: [
          {
            kind: "modified_instance_time_mismatch",
            date: "2026-05-04",
            activity_group_id: 7,
            instance_id: 42,
            activity_title: "Mensa",
            student_id: 21,
            expected_arrival: "12:00",
            arrival_source: "schedule",
            original_start_time: "12:00",
            modified_start_time: "12:30",
          },
        ],
      }).conflicts[0],
    ).toMatchObject({
      activityGroupId: "7",
      instanceId: "42",
      studentId: "21",
    });

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

    expect(
      mapExceptionConflicts({
        from: "2026-05-04",
        to: "2026-05-08",
        conflicts: undefined as never,
      }),
    ).toMatchObject({ conflicts: [] });
  });

  it("maps substitutes and templates", () => {
    expect(
      mapSubstitute({
        absent_staff_id: 11,
        substitute_staff_id: 12,
        date: "2026-05-04",
        affected_instances: [
          {
            instance_id: 42,
            title: "Mensa",
            start_time: "12:00",
            action: "substituted",
          },
        ],
        warnings: [
          {
            instance_id: 43,
            title: "Yoga",
            date: "2026-05-04",
            start_time: "14:00",
            end_time: "15:00",
          },
        ],
      }),
    ).toMatchObject({
      absentStaffId: "11",
      substituteStaffId: "12",
      affectedInstances: [{ instanceId: "42" }],
      warnings: [{ instanceId: "43" }],
    });

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
            enrollment_count: 8,
            supervisor_count: 1,
            student_ids: [21],
            staff_ids: [11],
            primary_staff_id: 11,
            schedules: [
              {
                id: 9,
                weekday: 1,
                start_time: "14:00",
                end_time: "15:00",
                week_pattern: 2,
                calendar_period_id: 5,
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
      schedules: [{ id: "9", calendarPeriodId: "5" }],
    });
  });

  it("maps optional substitute/template fields without fallback ids", () => {
    expect(
      mapSubstitute({
        absent_staff_id: 11,
        substitute_staff_id: 12,
        date: "2026-05-04",
        affected_instances: undefined as never,
        warnings: undefined as never,
      }),
    ).toMatchObject({ affectedInstances: [], warnings: [] });

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
            enrollment_count: undefined as never,
            supervisor_count: 0,
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
      enrollmentCount: undefined,
      supervisorCount: 0,
      studentIds: [],
      staffIds: [],
      primaryStaffId: undefined,
      schedules: [],
    });
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
