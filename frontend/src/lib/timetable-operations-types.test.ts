import { describe, expect, it } from "vitest";
import {
  mapPlannedInstance,
  mapRoster,
  mapStartOperation,
  type BackendStartOperationResult,
  type BackendTimetableRoster,
} from "./timetable-operations-types";

describe("timetable operation mappers", () => {
  it("maps planned instances from backend snake_case into frontend camelCase", () => {
    const result = mapPlannedInstance({
      id: 120,
      title: "Lernzeit",
      date: "2026-05-10",
      start_time: "14:00",
      end_time: "15:00",
      room_id: 220,
      status: "planned",
      is_overdue: true,
      minutes_until_start: -5,
      expected_students_count: 18,
      present_students_count: 3,
      assigned_staff_ids: [320, 321],
    });

    expect(result).toEqual({
      id: "120",
      title: "Lernzeit",
      date: "2026-05-10",
      startTime: "14:00",
      endTime: "15:00",
      roomId: "220",
      status: "planned",
      isOverdue: true,
      minutesUntilStart: -5,
      expectedStudentsCount: 18,
      presentStudentsCount: 3,
      assignedStaffIds: ["320", "321"],
    });
  });

  it("maps roster ids and optional fields without leaking undefined", () => {
    const raw: BackendTimetableRoster = {
      instance: {
        id: 121,
        title: "AG Kunst",
        status: "active",
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
        activeGroupId: "221",
        roomId: "222",
        roomName: null,
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
          visitEntryTime: "2026-05-10T12:01:00Z",
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
          visitEntryTime: null,
        },
      ],
    });
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
        roomId: "224",
        roomName: "Raum B",
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
          visitEntryTime: null,
        },
      ],
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
