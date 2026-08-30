import { describe, expect, it } from "vitest";

import {
  activeSupervisionRosterKey,
  buildGroupNameToIdMap,
  mapSupervisedGroupsToRooms,
  mapVisitsToSupervisionStudents,
  resolveSupervisionSelection,
  roomsOutsideSchulhofStatus,
  supervisionTabLabel,
  additionalSupervisionTarget,
  withActiveSupervisionPresence,
} from "./view-model";

describe("active-supervisions view model", () => {
  it("suppresses active-group roster keys after a not-found roster miss", () => {
    const missing = new Set(["active-1"]);

    expect(
      activeSupervisionRosterKey({
        selectedTimetableInstanceId: null,
        currentRoomId: "active-1",
        missingRosterActiveGroupIds: missing,
      }),
    ).toBeNull();
    expect(
      activeSupervisionRosterKey({
        selectedTimetableInstanceId: null,
        currentRoomId: "active-2",
        missingRosterActiveGroupIds: missing,
      }),
    ).toBe("timetable-roster-active-group-active-2");
  });

  it("keeps explicit timetable instance roster keys even when the active group missed", () => {
    expect(
      activeSupervisionRosterKey({
        selectedTimetableInstanceId: "instance-1",
        currentRoomId: "active-1",
        missingRosterActiveGroupIds: new Set(["active-1"]),
      }),
    ).toBe("timetable-roster-instance-1");
  });

  it("resolves a planned roster by active group after a Schulhof reload", () => {
    expect(
      activeSupervisionRosterKey({
        selectedTimetableInstanceId: null,
        currentRoomId: "schulhof-planned-active-group",
        missingRosterActiveGroupIds: new Set(),
      }),
    ).toBe("timetable-roster-active-group-schulhof-planned-active-group");
  });

  it("keeps parallel Schulhof sessions outside the permanent status tab", () => {
    const rooms = [
      { id: "status-group", name: "Aufsicht 1", room_name: "Schulhof" },
      { id: "parallel-group", name: "Aufsicht 2", room_name: "Schulhof" },
      { id: "other-group", name: "Kreativ", room_name: "Atelier" },
    ];

    expect(
      roomsOutsideSchulhofStatus(rooms, {
        schulhofTabEnabled: true,
        statusActiveGroupId: "status-group",
      }).map((room) => room.id),
    ).toEqual(["parallel-group", "other-group"]);
  });

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
        isCurrentUserSupervising: false,
      },
      {
        id: "active-a",
        name: "Gruppe A",
        room_id: "room-a",
        room: { id: "room-a", name: "Atelier", color: "#83CD2D" },
        isCurrentUserSupervising: true,
      },
    ]);

    expect(result.map((room) => room.room_name)).toEqual([
      "Atelier",
      "Zeichenraum",
    ]);
    expect(result[0]?.room_color).toBe("#83CD2D");
    expect(result[0]?.isCurrentUserSupervising).toBe(true);
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

  it("makes active room presence win over stale absence flags", () => {
    const checkInTime = new Date("2026-01-15T10:00:00.000Z");
    const [student] = mapVisitsToSupervisionStudents(
      [
        {
          studentId: "student-4",
          studentName: "Kerstin Krank",
          activeGroupId: "active-1",
          checkInTime,
          isActive: true,
          sick: true,
          sickSince: "2026-01-15T07:00:00.000Z",
          excused: true,
          excusedSince: "2026-01-15T07:30:00.000Z",
        },
      ],
      { roomName: "Atelier" },
    );

    expect(student?.sick).toBe(true);

    const normalized = withActiveSupervisionPresence(student!);

    expect(normalized).toMatchObject({
      current_location: "Anwesend - Atelier",
      sick: false,
      sick_since: undefined,
      excused: false,
      excused_since: undefined,
      class_trip: false,
      class_trip_since: undefined,
      day_planning_status: undefined,
      day_planning_reason: undefined,
      day_planning_label: undefined,
    });
  });
});

describe("resolveSupervisionSelection (#2265)", () => {
  const rooms = [
    { id: "group-a", name: "GT 1", room_name: "Raum 54", room_id: "54" },
    { id: "group-b", name: "GT 2", room_name: "Raum 54", room_id: "54" },
    { id: "group-c", name: "Kreativ", room_name: "Atelier", room_id: "55" },
  ];
  const base = {
    sessionParam: null,
    roomParam: null,
    savedSessionId: null,
    savedRoomId: null,
    rooms,
    currentSessionId: null,
    schulhofAvailable: false,
  };

  it("selects the session named by ?session=", () => {
    expect(
      resolveSupervisionSelection({ ...base, sessionParam: "group-b" }),
    ).toEqual({ kind: "session", sessionId: "group-b" });
  });

  it("keeps the current selection when ?session= already matches", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        sessionParam: "group-b",
        currentSessionId: "group-b",
      }),
    ).toEqual({ kind: "none" });
  });

  it("falls back to the saved room when ?session= is stale", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        sessionParam: "gone",
        savedSessionId: "gone",
        savedRoomId: "54",
        currentSessionId: "group-c",
      }),
    ).toEqual({ kind: "session", sessionId: "group-a" });
  });

  it("resolves schulhof from either param when available", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        sessionParam: "schulhof",
        schulhofAvailable: true,
      }),
    ).toEqual({ kind: "schulhof" });
    expect(
      resolveSupervisionSelection({
        ...base,
        roomParam: "schulhof",
        schulhofAvailable: true,
      }),
    ).toEqual({ kind: "schulhof" });
  });

  it("does NOT switch sessions when a legacy ?room= names the room the current session already runs in", () => {
    // The #2265 bug: four parallel sessions in room 54, a room-keyed URL
    // re-resolved to the FIRST session in that room on every refresh.
    expect(
      resolveSupervisionSelection({
        ...base,
        roomParam: "54",
        currentSessionId: "group-b",
      }),
    ).toEqual({ kind: "none" });
  });

  it("enters via legacy ?room= when no session in that room is selected", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        roomParam: "54",
        currentSessionId: "group-c",
      }),
    ).toEqual({ kind: "session", sessionId: "group-a" });
  });

  it("restores the saved session when no URL param is present", () => {
    expect(
      resolveSupervisionSelection({ ...base, savedSessionId: "group-c" }),
    ).toEqual({ kind: "session", sessionId: "group-c" });
  });

  it("falls back to the saved legacy room without switching within the room", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        savedRoomId: "54",
        currentSessionId: "group-b",
      }),
    ).toEqual({ kind: "none" });
    expect(resolveSupervisionSelection({ ...base, savedRoomId: "54" })).toEqual(
      { kind: "session", sessionId: "group-a" },
    );
  });

  it("asks to persist the first session when nothing is saved", () => {
    expect(resolveSupervisionSelection({ ...base })).toEqual({
      kind: "persist-first",
    });
  });
});

describe("supervision tab identity (#2265)", () => {
  it("orders parallel sessions in the same room stably by session name", () => {
    const result = mapSupervisedGroupsToRooms([
      {
        id: "active-2",
        name: "GT 2",
        room_id: "54",
        room: { id: "54", name: "Mehrzweckraum" },
      },
      {
        id: "active-1",
        name: "GT 1",
        room_id: "54",
        room: { id: "54", name: "Mehrzweckraum" },
      },
    ]);

    expect(result.map((room) => room.id)).toEqual(["active-1", "active-2"]);
  });

  it("labels a tab with the instance title plus the plan time when known", () => {
    expect(
      supervisionTabLabel(
        { id: "active-1", name: "GT 1", room_name: "Mehrzweckraum" },
        { title: "GT 1", timeRange: "12:45–13:45" },
      ),
    ).toBe("GT 1 · 12:45–13:45");
  });

  it("falls back to the room name when the live payload carries no session name", () => {
    expect(
      supervisionTabLabel({
        id: "active-3",
        name: undefined as unknown as string,
        room_name: "OGS-Raum 1",
      }),
    ).toBe("OGS-Raum 1");
  });

  it("labels a tab with the session name plus the room", () => {
    expect(
      supervisionTabLabel({
        id: "active-1",
        name: "GT 1",
        room_name: "Mehrzweckraum",
      }),
    ).toBe("GT 1 · Mehrzweckraum");
    expect(supervisionTabLabel({ id: "active-2", name: "Schulhof" })).toBe(
      "Schulhof",
    );
  });

  it("marks sessions supervised by the current user", () => {
    expect(
      supervisionTabLabel({
        id: "active-1",
        name: "Freispiel",
        isCurrentUserSupervising: true,
      }),
    ).toBe("Freispiel · Eigene Aufsicht");
  });

  it("offers additional supervision only on the user's current session", () => {
    expect(
      additionalSupervisionTarget({
        currentRoom: {
          id: "active-1",
          name: "Freispiel",
          isCurrentUserSupervising: true,
        },
        isSchulhofTabSelected: false,
        schulhofStatus: null,
      }),
    ).toBe("active-1");
    expect(
      additionalSupervisionTarget({
        currentRoom: { id: "active-2", name: "Malen" },
        isSchulhofTabSelected: false,
        schulhofStatus: null,
      }),
    ).toBeNull();
    expect(
      additionalSupervisionTarget({
        currentRoom: null,
        isSchulhofTabSelected: true,
        schulhofStatus: {
          activeGroupId: "active-yard",
          isUserSupervising: true,
        },
      }),
    ).toBe("active-yard");
  });
});
