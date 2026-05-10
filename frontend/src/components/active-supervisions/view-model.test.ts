import { describe, expect, it } from "vitest";

import {
  buildGroupNameToIdMap,
  mapSupervisedGroupsToRooms,
  mapVisitsToSupervisionStudents,
} from "./view-model";

describe("active-supervisions view model", () => {
  it("maps educational group names to ids", () => {
    const result = buildGroupNameToIdMap([
      { id: "g2", name: "Gruppe Blau", room: { name: "Raum B" } },
      { id: "g1", name: "Gruppe Rot", room: { name: "Raum A" } },
    ]);

    expect(result.get("Gruppe Rot")).toBe("g1");
    expect(result.get("Gruppe Blau")).toBe("g2");
  });

  it("maps and sorts supervised groups by visible room name", () => {
    const result = mapSupervisedGroupsToRooms([
      {
        id: "active-z",
        name: "Gruppe Z",
        room_id: "room-z",
        room: { id: "room-z", name: "Zeichenraum", color: "#5080D8" },
      },
      {
        id: "active-a",
        name: "Gruppe A",
        room_id: "room-a",
        room: { id: "room-a", name: "Atelier", color: "#83CD2D" },
      },
    ]);

    expect(result.map((room) => room.room_name)).toEqual([
      "Atelier",
      "Zeichenraum",
    ]);
    expect(result[0]?.room_color).toBe("#83CD2D");
  });

  it("maps only active visits to student card rows", () => {
    const groupNameToId = new Map([["Gruppe Rot", "g1"]]);
    const result = mapVisitsToSupervisionStudents(
      [
        {
          studentId: "student-1",
          studentName: "Max Mustermann",
          schoolClass: "2a",
          groupName: "Gruppe Rot",
          activeGroupId: "active-1",
          checkInTime: "2026-01-15T10:00:00.000Z",
          isActive: true,
        },
        {
          studentId: "student-2",
          studentName: "Erika Beispiel",
          activeGroupId: "active-1",
          checkInTime: "2026-01-15T09:00:00.000Z",
          isActive: false,
        },
      ],
      {
        roomName: "Atelier",
        roomColor: "#83CD2D",
        groupNameToId,
      },
    );

    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({
      id: "student-1",
      first_name: "Max",
      second_name: "Mustermann",
      group_id: "g1",
      current_location: "Anwesend - Atelier",
      current_room_color: "#83CD2D",
    });
    expect(result[0]?.checkInTime).toBeInstanceOf(Date);
  });

  it("falls back cleanly when optional visit and room fields are missing", () => {
    const checkInTime = new Date("2026-01-15T10:00:00.000Z");

    const rooms = mapSupervisedGroupsToRooms([
      {
        id: "active-without-room",
        name: "Freie Aufsicht",
      },
    ]);
    const students = mapVisitsToSupervisionStudents(
      [
        {
          studentId: "student-3",
          activeGroupId: "active-without-room",
          checkInTime,
          isActive: true,
        },
      ],
      {},
    );

    expect(rooms[0]).toMatchObject({
      id: "active-without-room",
      room_name: undefined,
      room_color: undefined,
    });
    expect(students[0]).toMatchObject({
      id: "student-3",
      name: "",
      first_name: "",
      second_name: "",
      school_class: "",
      current_location: "Anwesend",
      current_room_color: null,
      group_id: undefined,
    });
    expect(students[0]?.checkInTime).toBe(checkInTime);
  });
});
