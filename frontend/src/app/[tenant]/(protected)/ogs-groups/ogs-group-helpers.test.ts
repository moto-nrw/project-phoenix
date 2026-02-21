import { describe, it, expect } from "vitest";
import {
  getPickupUrgency,
  isStudentInGroupRoom,
  matchesSearchFilter,
  matchesAttendanceFilter,
  matchesForeignRoomFilter,
} from "./ogs-group-helpers";
import type { OGSGroup } from "./ogs-group-helpers";
import type { Student } from "~/lib/api";

// Helper to create a minimal Student for testing
function makeStudent(overrides: Partial<Student> = {}): Student {
  return {
    id: "1",
    first_name: "Max",
    second_name: "Mustermann",
    name: "Max Mustermann",
    school_class: "1a",
    current_location: undefined,
    ...overrides,
  } as Student;
}

// Helper to create a minimal OGSGroup
function makeGroup(overrides: Partial<OGSGroup> = {}): OGSGroup {
  return {
    id: "10",
    name: "Gruppe A",
    room_name: "Raum 1",
    room_id: "5",
    ...overrides,
  };
}

describe("getPickupUrgency", () => {
  it("returns 'none' when pickupTimeStr is undefined", () => {
    expect(getPickupUrgency(undefined, new Date())).toBe("none");
  });

  it("returns 'overdue' when pickup time is in the past", () => {
    const now = new Date("2025-01-15T15:00:00");
    expect(getPickupUrgency("14:30", now)).toBe("overdue");
  });

  it("returns 'soon' when pickup time is within 30 minutes", () => {
    const now = new Date("2025-01-15T14:45:00");
    expect(getPickupUrgency("15:00", now)).toBe("soon");
  });

  it("returns 'soon' when pickup time is exactly now (0 minutes diff)", () => {
    const now = new Date("2025-01-15T15:00:00");
    expect(getPickupUrgency("15:00", now)).toBe("soon");
  });

  it("returns 'normal' when pickup time is more than 30 minutes away", () => {
    const now = new Date("2025-01-15T13:00:00");
    expect(getPickupUrgency("15:00", now)).toBe("normal");
  });

  it("returns 'soon' at exactly 30 minutes before pickup", () => {
    const now = new Date("2025-01-15T14:30:00");
    expect(getPickupUrgency("15:00", now)).toBe("soon");
  });

  it("returns 'normal' at 31 minutes before pickup", () => {
    const now = new Date("2025-01-15T14:29:00");
    expect(getPickupUrgency("15:00", now)).toBe("normal");
  });
});

describe("isStudentInGroupRoom", () => {
  it("returns false when student has no current_location", () => {
    const student = makeStudent({ current_location: undefined });
    const group = makeGroup();
    expect(isStudentInGroupRoom(student, group)).toBe(false);
  });

  it("returns false when group has no room_name", () => {
    const student = makeStudent({ current_location: "Anwesend - Raum 1" });
    const group = makeGroup({ room_name: undefined });
    expect(isStudentInGroupRoom(student, group)).toBe(false);
  });

  it("returns false when group is null", () => {
    const student = makeStudent({ current_location: "Anwesend - Raum 1" });
    expect(isStudentInGroupRoom(student, null)).toBe(false);
  });

  it("returns false when group is undefined", () => {
    const student = makeStudent({ current_location: "Anwesend - Raum 1" });
    expect(isStudentInGroupRoom(student, undefined)).toBe(false);
  });

  it("returns true when student location matches group room name", () => {
    const student = makeStudent({ current_location: "Anwesend - Raum 1" });
    const group = makeGroup({ room_name: "Raum 1" });
    expect(isStudentInGroupRoom(student, group)).toBe(true);
  });

  it("matches room names case-insensitively", () => {
    const student = makeStudent({ current_location: "Anwesend - raum 1" });
    const group = makeGroup({ room_name: "Raum 1" });
    expect(isStudentInGroupRoom(student, group)).toBe(true);
  });

  it("returns false when student is in a different room", () => {
    const student = makeStudent({ current_location: "Anwesend - Raum 2" });
    const group = makeGroup({ room_name: "Raum 1" });
    expect(isStudentInGroupRoom(student, group)).toBe(false);
  });

  it("falls back to room_id matching when room name does not match", () => {
    const student = makeStudent({ current_location: "some location with 5" });
    const group = makeGroup({ room_name: "Raum X", room_id: "5" });
    expect(isStudentInGroupRoom(student, group)).toBe(true);
  });

  it("returns false when neither room name nor room_id matches", () => {
    const student = makeStudent({ current_location: "Anwesend - Raum 2" });
    const group = makeGroup({ room_name: "Raum 1", room_id: "99" });
    expect(isStudentInGroupRoom(student, group)).toBe(false);
  });

  it("returns false when room name does not match and no room_id set", () => {
    const student = makeStudent({ current_location: "Anwesend - Raum 2" });
    const group = makeGroup({ room_name: "Raum 1", room_id: undefined });
    expect(isStudentInGroupRoom(student, group)).toBe(false);
  });
});

describe("matchesSearchFilter", () => {
  it("returns true when search term is empty", () => {
    const student = makeStudent();
    expect(matchesSearchFilter(student, "")).toBe(true);
  });

  it("matches by first_name", () => {
    const student = makeStudent({ first_name: "Mia" });
    expect(matchesSearchFilter(student, "mia")).toBe(true);
  });

  it("matches by second_name", () => {
    const student = makeStudent({ second_name: "Fischer" });
    expect(matchesSearchFilter(student, "fisch")).toBe(true);
  });

  it("matches by full name", () => {
    const student = makeStudent({ name: "Mia Fischer" });
    expect(matchesSearchFilter(student, "mia")).toBe(true);
  });

  it("matches by school_class", () => {
    const student = makeStudent({ school_class: "2b" });
    expect(matchesSearchFilter(student, "2b")).toBe(true);
  });

  it("is case-insensitive", () => {
    const student = makeStudent({ first_name: "Max" });
    expect(matchesSearchFilter(student, "MAX")).toBe(true);
  });

  it("returns false when nothing matches", () => {
    const student = makeStudent({
      name: "Max Mustermann",
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "1a",
    });
    expect(matchesSearchFilter(student, "zzz")).toBe(false);
  });

  it("handles undefined student fields gracefully", () => {
    const student = makeStudent({
      name: undefined,
      first_name: undefined,
      second_name: undefined,
      school_class: undefined,
    });
    expect(matchesSearchFilter(student, "test")).toBe(false);
  });
});

describe("matchesAttendanceFilter", () => {
  const roomStatus: Record<
    string,
    { in_group_room?: boolean; current_room_id?: number }
  > = {
    "1": { in_group_room: true, current_room_id: 5 },
    "2": { in_group_room: false, current_room_id: 10 },
    "3": { in_group_room: false },
  };

  it("returns true when filter is 'all'", () => {
    const student = makeStudent();
    expect(matchesAttendanceFilter(student, "all", roomStatus)).toBe(true);
  });

  it("filters by in_room using room status", () => {
    const inRoomStudent = makeStudent({ id: "1" });
    const notInRoomStudent = makeStudent({ id: "2" });
    expect(matchesAttendanceFilter(inRoomStudent, "in_room", roomStatus)).toBe(
      true,
    );
    expect(
      matchesAttendanceFilter(notInRoomStudent, "in_room", roomStatus),
    ).toBe(false);
  });

  it("filters by foreign_room", () => {
    const foreignRoomStudent = makeStudent({ id: "2" });
    const inGroupRoomStudent = makeStudent({ id: "1" });
    expect(
      matchesAttendanceFilter(foreignRoomStudent, "foreign_room", roomStatus),
    ).toBe(true);
    expect(
      matchesAttendanceFilter(inGroupRoomStudent, "foreign_room", roomStatus),
    ).toBe(false);
  });

  it("filters by transit location", () => {
    const transitStudent = makeStudent({ current_location: "Unterwegs" });
    const homeStudent = makeStudent({ current_location: "Zuhause" });
    expect(matchesAttendanceFilter(transitStudent, "transit", {})).toBe(true);
    expect(matchesAttendanceFilter(homeStudent, "transit", {})).toBe(false);
  });

  it("filters by schoolyard location", () => {
    const schoolyardStudent = makeStudent({ current_location: "Schulhof" });
    const homeStudent = makeStudent({ current_location: "Zuhause" });
    expect(matchesAttendanceFilter(schoolyardStudent, "schoolyard", {})).toBe(
      true,
    );
    expect(matchesAttendanceFilter(homeStudent, "schoolyard", {})).toBe(false);
  });

  it("filters by at_home location", () => {
    const homeStudent = makeStudent({ current_location: "Zuhause" });
    const presentStudent = makeStudent({
      current_location: "Anwesend - Raum 1",
    });
    expect(matchesAttendanceFilter(homeStudent, "at_home", {})).toBe(true);
    expect(matchesAttendanceFilter(presentStudent, "at_home", {})).toBe(false);
  });

  it("returns true for unknown filter values (default case)", () => {
    const student = makeStudent();
    expect(matchesAttendanceFilter(student, "unknown_filter", {})).toBe(true);
  });

  it("returns false for in_room when student has no room status", () => {
    const student = makeStudent({ id: "999" }); // Not in roomStatus map
    expect(matchesAttendanceFilter(student, "in_room", roomStatus)).toBe(false);
  });
});

describe("matchesForeignRoomFilter", () => {
  it("returns true when student is in a foreign room", () => {
    expect(
      matchesForeignRoomFilter({ in_group_room: false, current_room_id: 10 }),
    ).toBe(true);
  });

  it("returns false when student is in group room", () => {
    expect(
      matchesForeignRoomFilter({ in_group_room: true, current_room_id: 5 }),
    ).toBe(false);
  });

  it("returns false when student has no room status", () => {
    expect(matchesForeignRoomFilter(undefined)).toBe(false);
  });

  it("returns false when student has no current_room_id", () => {
    expect(matchesForeignRoomFilter({ in_group_room: false })).toBe(false);
  });

  it("returns false when in_group_room is undefined but has room_id", () => {
    expect(matchesForeignRoomFilter({ current_room_id: 10 })).toBe(false);
  });
});
