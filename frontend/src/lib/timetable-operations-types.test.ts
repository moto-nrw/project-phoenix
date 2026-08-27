import { describe, expect, it } from "vitest";
import {
  mapPlannedInstance,
  mapRoster,
  mapStartOperation,
  type BackendStartOperationResult,
  type BackendTimetableRoster,
} from "./timetable-operations-types";

describe("timetable operation mappers", () => {
  it("keeps loaded pickup times distinct from a failed pickup lookup", () => {
    const loaded = mapRoster({
      instance: {
        id: 119,
        title: "Randstunde",
        status: "planned",
        room_id: 219,
      },
      pickup_times_loaded: true,
      rows: [
        {
          student_id: 419,
          student_name: "Mara Montag",
          school_class: "1a",
          group_name: "OGS Grün",
          planned: true,
          is_unplanned: false,
          currently_present: false,
          status: "expected",
          pickup_time: "13:30",
        },
      ],
    });
    const failed = mapRoster({
      instance: {
        id: 118,
        title: "Ganztag",
        status: "active",
        room_id: 218,
      },
      pickup_times_loaded: false,
      rows: [
        {
          student_id: 418,
          student_name: "Nora Nichtgeladen",
          school_class: "2b",
          group_name: "OGS Blau",
          planned: true,
          is_unplanned: false,
          currently_present: false,
          status: "expected",
          pickup_time: null,
        },
      ],
    });

    expect(loaded.pickupTimesLoaded).toBe(true);
    expect(loaded.rows[0]!.pickupTime).toBe("13:30");
    expect(failed.pickupTimesLoaded).toBe(false);
    expect(failed.rows[0]!.pickupTime).toBeNull();
  });

  it("maps planned instances from backend snake_case into frontend camelCase", () => {
    const result = mapPlannedInstance({
      id: 120,
      title: "Lernzeit",
      date: "2026-05-10",
      start_time: "14:00",
      end_time: "15:00",
      room_id: 220,
      room_name: "Lernraum",
      status: "planned",
      is_overdue: true,
      minutes_until_start: -5,
      expected_students_count: 18,
      present_students_count: 3,
      assigned_staff_ids: [320, 321],
      is_assigned: true,
      is_primary: true,
      is_substitute: false,
      is_absent: false,
      roster_preview: [
        {
          student_id: 420,
          student_name: "Mila Muster",
          school_class: "3a",
          group_name: "OGS Blau",
          planned: true,
          is_unplanned: false,
          currently_present: false,
          status: "expected",
        },
      ],
    });

    expect(result).toEqual({
      id: "120",
      title: "Lernzeit",
      date: "2026-05-10",
      startTime: "14:00",
      endTime: "15:00",
      roomId: "220",
      roomName: "Lernraum",
      status: "planned",
      isOverdue: true,
      minutesUntilStart: -5,
      expectedStudentsCount: 18,
      notScheduledStudentsCount: 0,
      presentStudentsCount: 3,
      assignedStaffIds: ["320", "321"],
      isAssigned: true,
      isPrimary: true,
      isSubstitute: false,
      isAbsent: false,
      canStart: false,
      startAvailableAt: "",
      startExpiresAt: "",
      rosterPreview: [
        {
          studentId: "420",
          studentName: "Mila Muster",
          schoolClass: "3a",
          groupName: "OGS Blau",
          planned: true,
          isUnplanned: false,
          currentlyPresent: false,
          visitId: null,
          status: "expected",
          substatus: null,
          note: null,
          checkedInAt: null,
          checkedOutAt: null,
          parallelPresentIn: null,
          visitEntryTime: null,
          warnings: [],
          careDayStatus: "unknown",
        },
      ],
    });
  });

  it("maps roster ids and optional fields without leaking undefined", () => {
    const raw: BackendTimetableRoster = {
      instance: {
        id: 121,
        title: "AG Kunst",
        status: "active",
        is_spontaneous: true,
        active_group_id: 221,
        room_id: 222,
      },
      rows: [
        {
          student_id: 420,
          student_name: "Mila Muster",
          school_class: "3a",
          group_name: "OGS Blau",
          planned: true,
          is_unplanned: false,
          currently_present: true,
          visit_id: 520,
          status: "present",
          substatus: "late",
          note: "kam nach",
          checked_in_at: "2026-05-10T12:00:00Z",
          checked_out_at: "2026-05-10T13:00:00Z",
          visit_entry_time: "2026-05-10T12:01:00Z",
        },
        {
          student_id: 421,
          student_name: "Ben Beispiel",
          school_class: "4b",
          group_name: "",
          planned: false,
          is_unplanned: true,
          currently_present: false,
          status: "expected",
        },
      ],
    };

    expect(mapRoster(raw)).toEqual({
      instance: {
        id: "121",
        title: "AG Kunst",
        status: "active",
        isSpontaneous: true,
        activeGroupId: "221",
        roomId: "222",
        roomName: null,
        date: "",
        startTime: "",
        endTime: "",
        canComplete: false,
        completeAvailableAt: "",
      },
      rows: [
        {
          studentId: "420",
          studentName: "Mila Muster",
          schoolClass: "3a",
          groupName: "OGS Blau",
          planned: true,
          isUnplanned: false,
          currentlyPresent: true,
          visitId: "520",
          status: "present",
          substatus: "late",
          note: "kam nach",
          checkedInAt: "2026-05-10T12:00:00Z",
          checkedOutAt: "2026-05-10T13:00:00Z",
          visitEntryTime: "2026-05-10T12:01:00Z",
          warnings: [],
          careDayStatus: "unknown",
          parallelPresentIn: null,
        },
        {
          studentId: "421",
          studentName: "Ben Beispiel",
          schoolClass: "4b",
          groupName: "",
          planned: false,
          isUnplanned: true,
          currentlyPresent: false,
          visitId: null,
          status: "expected",
          substatus: null,
          note: null,
          checkedInAt: null,
          checkedOutAt: null,
          parallelPresentIn: null,
          visitEntryTime: null,
          warnings: [],
          careDayStatus: "unknown",
        },
      ],
      movedFrom: null,
    });
  });

  it("maps planned instance defaults when backend omits nullable flags and preview", () => {
    const result = mapPlannedInstance({
      id: 124,
      title: "Freispiel",
      date: "2026-05-11",
      start_time: "15:00",
      end_time: "16:00",
      room_id: 225,
      status: "planned",
      is_overdue: false,
      minutes_until_start: 30,
      expected_students_count: 0,
      present_students_count: 0,
      assigned_staff_ids: [],
    });

    expect(result).toEqual({
      id: "124",
      title: "Freispiel",
      date: "2026-05-11",
      startTime: "15:00",
      endTime: "16:00",
      roomId: "225",
      roomName: null,
      status: "planned",
      isOverdue: false,
      minutesUntilStart: 30,
      expectedStudentsCount: 0,
      notScheduledStudentsCount: 0,
      presentStudentsCount: 0,
      assignedStaffIds: [],
      isAssigned: false,
      isPrimary: false,
      isSubstitute: false,
      isAbsent: false,
      canStart: false,
      startAvailableAt: "",
      startExpiresAt: "",
      rosterPreview: [],
    });
  });

  it("maps roster warnings and normalizes nullable warning ids", () => {
    const raw: BackendTimetableRoster = {
      instance: {
        id: 125,
        title: "Lernzeit",
        status: "active",
        active_group_id: 226,
        room_id: 227,
        room_name: null,
      },
      rows: [
        {
          student_id: 423,
          student_name: "Warn Kind",
          school_class: "3d",
          group_name: "OGS Rot",
          planned: true,
          is_unplanned: false,
          currently_present: false,
          status: "expected",
          warnings: [
            {
              kind: "arrival_after_slot_start",
              message: "zu spät",
              expected_arrival: "14:30",
              slot_start: "14:00",
              expected_group_id: 700,
              expected_group_name: "Klasse 3d",
              current_education_group_id: 701,
            },
            {
              kind: "missing_arrival_schedule",
              message: "kein Plan",
              expected_arrival: null,
              slot_start: null,
              expected_group_id: null,
              expected_group_name: null,
              current_education_group_id: null,
            },
          ],
        },
      ],
    };

    expect(mapRoster(raw).rows[0]!.warnings).toEqual([
      {
        kind: "arrival_after_slot_start",
        message: "zu spät",
        expectedArrival: "14:30",
        slotStart: "14:00",
        expectedGroupId: "700",
        expectedGroupName: "Klasse 3d",
        currentEducationGroupId: "701",
      },
      {
        kind: "missing_arrival_schedule",
        message: "kein Plan",
        expectedArrival: null,
        slotStart: null,
        expectedGroupId: null,
        expectedGroupName: null,
        currentEducationGroupId: null,
      },
    ]);
  });

  it("maps nullable roster instance fields", () => {
    const raw: BackendTimetableRoster = {
      instance: {
        id: 123,
        title: "Spätbetreuung",
        status: "planned",
        active_group_id: null,
        room_id: 224,
        room_name: "Raum B",
      },
      rows: [
        {
          student_id: 422,
          student_name: "Nora Null",
          school_class: "2c",
          group_name: "OGS Grün",
          planned: true,
          is_unplanned: false,
          currently_present: false,
          visit_id: null,
          status: "absent",
          substatus: null,
          note: null,
          checked_in_at: null,
          visit_entry_time: null,
        },
      ],
    };

    expect(mapRoster(raw)).toEqual({
      instance: {
        id: "123",
        title: "Spätbetreuung",
        status: "planned",
        activeGroupId: null,
        isSpontaneous: false,
        roomId: "224",
        roomName: "Raum B",
        date: "",
        startTime: "",
        endTime: "",
        canComplete: false,
        completeAvailableAt: "",
      },
      rows: [
        {
          studentId: "422",
          studentName: "Nora Null",
          schoolClass: "2c",
          groupName: "OGS Grün",
          planned: true,
          isUnplanned: false,
          currentlyPresent: false,
          visitId: null,
          status: "absent",
          substatus: null,
          note: null,
          checkedInAt: null,
          checkedOutAt: null,
          parallelPresentIn: null,
          visitEntryTime: null,
          warnings: [],
          careDayStatus: "unknown",
        },
      ],
      movedFrom: null,
    });
  });

  it("maps start operation ids to strings", () => {
    const raw: BackendStartOperationResult = {
      instance_id: 122,
      active_group_id: 223,
      status: "active",
    };

    expect(mapStartOperation(raw)).toEqual({
      instanceId: "122",
      activeGroupId: "223",
      status: "active",
    });
  });
});
