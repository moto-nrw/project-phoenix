import { describe, expect, it } from "vitest";

import {
  activeSupervisionRosterKey,
  buildGroupNameToIdMap,
  mapSupervisedGroupsToRooms,
  mapVisitsToSupervisionStudents,
  hasOwnBlock,
  openRoomSections,
  schulhofHeadActionsApply,
  resolveSupervisionSelection,
  sessionsOutsideOpenRooms,
  supervisionTabLabel,
  additionalSupervisionTarget,
  occupiedRoomIdsForSpontaneousStart,
  withActiveSupervisionPresence,
  type OpenRoomSessionView,
} from "./view-model";

// A running block in the released room. `own` makes the caller supervise it.
const blockSession = (
  id: string,
  options: {
    readonly own?: boolean;
    readonly planned?: boolean;
    readonly canOperate?: boolean;
    readonly studentCount?: number;
  } = {},
): OpenRoomSessionView => ({
  activeGroupId: id,
  title: `Block ${id}`,
  independent: false,
  isUserSupervising: options.own === true,
  canAssign: options.own === true,
  studentCount: options.studentCount ?? 0,
  block: {
    instanceId: `instance-${id}`,
    startTime: "13:00",
    endTime: "14:00",
    isUserAssigned: options.planned === true,
    canOperate:
      options.canOperate ?? (options.own === true || options.planned === true),
  },
});

const roomSession = (
  id: string,
  options: {
    readonly independent?: boolean;
    readonly own?: boolean;
    readonly studentCount?: number;
  } = {},
): OpenRoomSessionView => ({
  activeGroupId: id,
  title: options.independent ? "" : "Schulhof Freispiel",
  independent: options.independent === true,
  isUserSupervising: options.own === true,
  canAssign: options.own === true,
  studentCount: options.studentCount ?? 0,
  block: null,
});

const sectionKeys = (sessions: readonly OpenRoomSessionView[]) =>
  openRoomSections({ sessions })?.map((section) => [
    section.key,
    section.isOwn,
  ]) ?? null;

describe("open room sections (#3281)", () => {
  // Former openRoomRosterActiveGroupId case: one foreign offering in the
  // room. Its roster stays reachable as a section, not as the room's view.
  it("keeps the roster of the room's only block reachable without an own session", () => {
    const sections = openRoomSections({ sessions: [blockSession("fußball")] });

    expect(sections).toEqual([
      {
        kind: "block",
        key: "block:fußball",
        session: blockSession("fußball"),
        block: blockSession("fußball").block,
        isOwn: false,
        assignableSessionId: null,
      },
    ]);
  });

  // Former case: several foreign offerings used to fall back to the merged
  // view without actions. Every block now keeps its section.
  it("lists every foreign block, none of them as the caller's own", () => {
    expect(
      sectionKeys([blockSession("fußball"), blockSession("tanzen")]),
    ).toEqual([
      ["block:fußball", false],
      ["block:tanzen", false],
    ]);
  });

  it("puts the caller's own block first while other offerings share the room", () => {
    expect(
      sectionKeys([
        blockSession("tanzen"),
        blockSession("fußball", { own: true }),
        blockSession("basteln"),
      ]),
    ).toEqual([
      ["block:fußball", true],
      ["block:tanzen", false],
      ["block:basteln", false],
    ]);
  });

  // Former cases: two own sessions needed an explicit ?session= selection,
  // otherwise the merged view without actions won. Both are now open.
  it("keeps every own block operable without a session choice", () => {
    const sections = openRoomSections({
      sessions: [
        blockSession("fußball", { own: true }),
        blockSession("tanzen", { own: true }),
      ],
    });

    expect(sections?.map((section) => section.isOwn)).toEqual([true, true]);
    expect(
      sections?.map((section) =>
        section.kind === "block" ? section.block.canOperate : null,
      ),
    ).toEqual([true, true]);
  });

  it("counts a plan entry as the caller's own block before it is started", () => {
    expect(
      sectionKeys([
        blockSession("tanzen"),
        blockSession("fußball", { planned: true }),
      ]),
    ).toEqual([
      ["block:fußball", true],
      ["block:tanzen", false],
    ]);
  });

  // Former case: a selection pointing at a session in another room was
  // ignored. Sections come from the room's own sessions only, so there is
  // nothing to ignore.
  it("builds a room's sections from that room's sessions only", () => {
    expect(
      openRoomSections({ sessions: [blockSession("fußball", { own: true })] })
        ?.length,
    ).toBe(1);
  });

  it("offers adding supervisors only where the caller may assign", () => {
    const sections = openRoomSections({
      sessions: [
        blockSession("fußball", { own: true }),
        blockSession("tanzen"),
      ],
    });

    expect(sections?.map((section) => section.assignableSessionId)).toEqual([
      "fußball",
      null,
    ]);
  });

  it("keeps a kiosk-only room as today's room view", () => {
    expect(
      openRoomSections({
        sessions: [
          roomSession("kiosk", { studentCount: 4 }),
          roomSession("stay", { independent: true, studentCount: 1 }),
        ],
      }),
    ).toBeNull();
    expect(openRoomSections({ sessions: [] })).toBeNull();
    expect(openRoomSections({})).toBeNull();
  });

  // #3634: a session outside a block keeps its activity's limit, so the
  // section can show "Anzahl / Grenze" and flag an overbooked session.
  it("keeps the activity's limit on a session section outside a block", () => {
    const sections = openRoomSections({
      sessions: [
        blockSession("fußball", { own: true }),
        { ...roomSession("kiosk", { studentCount: 66 }), participantLimit: 45 },
      ],
    });

    const kiosk = sections?.find((section) => section.key === "session:kiosk");
    expect(kiosk).toMatchObject({
      kind: "occupancy",
      studentCount: 66,
      participantLimit: 45,
    });
  });

  it("collects independent stays under one section after the blocks", () => {
    const sections = openRoomSections({
      sessions: [
        roomSession("stay-a", { independent: true, studentCount: 1 }),
        blockSession("fußball", { own: true, studentCount: 3 }),
        roomSession("stay-b", {
          independent: true,
          own: true,
          studentCount: 2,
        }),
      ],
    });

    expect(sections?.at(-1)).toEqual({
      kind: "occupancy",
      key: "independent",
      title: "",
      independent: true,
      activeGroupIds: ["stay-a", "stay-b"],
      studentCount: 3,
      isOwn: true,
      assignableSessionId: "stay-b",
    });
  });

  it("leaves out an empty section for independent stays", () => {
    expect(
      sectionKeys([
        blockSession("fußball"),
        roomSession("stay", { independent: true, own: true }),
      ]),
    ).toEqual([["block:fußball", false]]);
  });

  it("tells whether one of the room's blocks is the caller's own", () => {
    const own = openRoomSections({
      sessions: [
        blockSession("tanzen"),
        blockSession("fußball", { planned: true }),
      ],
    });
    const foreignOnly = openRoomSections({
      sessions: [
        blockSession("tanzen"),
        roomSession("freispiel", { own: true, studentCount: 1 }),
      ],
    });

    expect(hasOwnBlock(own)).toBe(true);
    // Supervising the yard's own session is not running a block.
    expect(hasOwnBlock(foreignOnly)).toBe(false);
    expect(hasOwnBlock(null)).toBe(false);
  });

  it("keeps the Schulhof head actions off the room's blocks", () => {
    const sections = openRoomSections({
      sessions: [
        blockSession("fußball", { own: true }),
        roomSession("freispiel", { independent: true, studentCount: 1 }),
      ],
    });

    // The yard's own session: „Beaufsichtigen“ / „Aufsicht abgeben“ act on it.
    expect(schulhofHeadActionsApply(sections, "freispiel")).toBe(true);
    // A block has its own section; the head must not claim or release it.
    expect(schulhofHeadActionsApply(sections, "fußball")).toBe(false);
    // Without blocks the head acts as before, also before anything started.
    expect(schulhofHeadActionsApply(null, "freispiel")).toBe(true);
    expect(schulhofHeadActionsApply(null, null)).toBe(true);
  });

  it("gives a kiosk session beside blocks its own read-only section", () => {
    const sections = openRoomSections({
      sessions: [
        roomSession("kiosk", { own: true }),
        blockSession("fußball"),
        roomSession("stay", { independent: true, studentCount: 1 }),
      ],
    });

    expect(sections?.map((section) => section.key)).toEqual([
      "block:fußball",
      "session:kiosk",
      "independent",
    ]);
    expect(sections?.[1]).toEqual({
      kind: "occupancy",
      key: "session:kiosk",
      title: "Schulhof Freispiel",
      independent: false,
      activeGroupIds: ["kiosk"],
      studentCount: 0,
      participantLimit: null,
      isOwn: true,
      assignableSessionId: "kiosk",
    });
  });
});

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

  it("drops every own session that a released room already stands for", () => {
    // All sessions of a released room feed its one shared entry, so listing
    // them again would put the same place on screen once per session (#3065).
    const rooms = [
      { id: "yard-1", name: "Aufsicht 1", room_name: "Schulhof", room_id: "7" },
      { id: "yard-2", name: "Aufsicht 2", room_name: "Schulhof", room_id: "7" },
      { id: "other", name: "Kreativ", room_name: "Atelier", room_id: "9" },
    ];

    expect(
      sessionsOutsideOpenRooms(rooms, new Set(["7"])).map((room) => room.id),
    ).toEqual(["other"]);
  });

  it("keeps every own session when nothing is released", () => {
    const rooms = [
      { id: "yard-1", name: "Aufsicht 1", room_id: "7" },
      { id: "other", name: "Kreativ", room_id: "9" },
    ];

    expect(
      sessionsOutsideOpenRooms(rooms, new Set()).map((room) => room.id),
    ).toEqual(["yard-1", "other"]);
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

  it("carries the activity's participant limit onto the session (#3634)", () => {
    const [limited, unlimited] = mapSupervisedGroupsToRooms([
      {
        id: "a",
        name: "Fußball",
        room: { id: "r1", name: "Atelier" },
        participantLimit: 45,
      },
      { id: "b", name: "Lesen", room: { id: "r2", name: "Bücherei" } },
    ]);

    expect(limited?.participant_limit).toBe(45);
    expect(unlimited?.participant_limit).toBeNull();
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
          activityName: "Fußball",
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
      activity_name: "Fußball",
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
    currentOpenRoomId: null,
    openRoomIds: new Set<string>(),
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

  it("answers a released room with its shared view, by real room id", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        roomParam: "7",
        openRoomIds: new Set(["7"]),
      }),
    ).toEqual({ kind: "open-room", roomId: "7" });
  });

  it("prefers the shared view over the sessions running in that room", () => {
    // Room 54 carries the caller's own parallel sessions. Once it is
    // released, the room is the thing being addressed — picking one of its
    // sessions would be the old, ambiguous answer.
    expect(
      resolveSupervisionSelection({
        ...base,
        roomParam: "54",
        openRoomIds: new Set(["54"]),
      }),
    ).toEqual({ kind: "open-room", roomId: "54" });
  });

  it("normalizes a session URL in a released room to its shared view", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        sessionParam: "group-b",
        openRoomIds: new Set(["54"]),
      }),
    ).toEqual({ kind: "open-room", roomId: "54" });
  });

  it("keeps the current selection when the shared room is already open", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        roomParam: "7",
        currentOpenRoomId: "7",
        openRoomIds: new Set(["7"]),
      }),
    ).toEqual({ kind: "none" });
  });

  it("restores a saved released room after a reload", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        savedRoomId: "7",
        openRoomIds: new Set(["7"]),
      }),
    ).toEqual({ kind: "open-room", roomId: "7" });
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

  it("restores a saved session in a released room as its shared view", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        savedSessionId: "group-b",
        openRoomIds: new Set(["54"]),
      }),
    ).toEqual({ kind: "open-room", roomId: "54" });
  });

  it("prefers a non-released session for an unaddressed dashboard", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        openRoomIds: new Set(["54"]),
      }),
    ).toEqual({ kind: "session", sessionId: "group-c" });
  });

  it("opens the shared view when every available session is in a released room", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        rooms: rooms.slice(0, 2),
        openRoomIds: new Set(["54"]),
      }),
    ).toEqual({ kind: "open-room", roomId: "54" });
  });

  it("opens the caller's supervised released room, not the first released room by name", () => {
    // Home "Zur Aufsicht" lands on /active-supervisions with no query.
    // Schulhof is first among released rooms, but the caller only
    // supervises Sporthalle — that is the matching sidebar row.
    expect(
      resolveSupervisionSelection({
        ...base,
        rooms: [
          {
            id: "fußball",
            name: "Fußball",
            room_name: "Sporthalle",
            room_id: "2",
          },
        ],
        openRoomIds: new Set(["1", "2"]),
      }),
    ).toEqual({ kind: "open-room", roomId: "2" });
  });

  it("opens the first released room when the caller has no own session", () => {
    expect(
      resolveSupervisionSelection({
        ...base,
        rooms: [],
        openRoomIds: new Set(["1", "2"]),
      }),
    ).toEqual({ kind: "open-room", roomId: "1" });
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

  it("offers additional supervision on the selected session", () => {
    expect(
      additionalSupervisionTarget({
        currentRoom: {
          id: "active-1",
          name: "Freispiel",
          canAssign: true,
          isCurrentUserSupervising: true,
        },
        currentOpenRoom: null,
      }),
    ).toBe("active-1");
    expect(
      additionalSupervisionTarget({
        currentRoom: { id: "active-2", name: "Malen", canAssign: true },
        currentOpenRoom: null,
      }),
    ).toBe("active-2");
    expect(
      additionalSupervisionTarget({
        currentRoom: { id: "active-3", name: "Basteln", canAssign: false },
        currentOpenRoom: null,
      }),
    ).toBeNull();
  });

  it("offers additional supervision in a shared room only for one assignable session", () => {
    const room = (sessions: readonly OpenRoomSessionView[]) => ({
      roomId: "7",
      name: "Schulhof",
      isUserSupervising: sessions.some((session) => session.isUserSupervising),
      activeGroupIds: sessions.map((session) => session.activeGroupId),
      studentCount: 0,
      students: [],
      sessions,
    });

    // The kiosk room view keeps the head action for its one supervision.
    expect(
      additionalSupervisionTarget({
        currentRoom: null,
        currentOpenRoom: room([roomSession("active-yard", { own: true })]),
      }),
    ).toBe("active-yard");

    // Two assignable sessions: none of them is "the" supervision of the
    // room, so the head offers nothing rather than picking one.
    expect(
      additionalSupervisionTarget({
        currentRoom: null,
        currentOpenRoom: room([
          roomSession("active-yard", { own: true }),
          roomSession("active-stay", { independent: true, own: true }),
        ]),
      }),
    ).toBeNull();

    // Seeing a shared room is not supervising it.
    expect(
      additionalSupervisionTarget({
        currentRoom: null,
        currentOpenRoom: room([roomSession("active-yard")]),
      }),
    ).toBeNull();

    // With blocks, every section carries its own action (#3281), so the
    // head carries none, even for a single own block.
    expect(
      additionalSupervisionTarget({
        currentRoom: null,
        currentOpenRoom: room([blockSession("active-ball", { own: true })]),
      }),
    ).toBeNull();
  });

  it("keeps a released room selectable when it only holds independent stays", () => {
    expect(
      occupiedRoomIdsForSpontaneousStart({
        ownSupervisionRoomIds: ["werkraum"],
        openRooms: [
          { roomId: "sporthalle", hasOccupyingSession: false },
          { roomId: "schulhof", hasOccupyingSession: true },
        ],
      }),
    ).toEqual(["werkraum", "schulhof"]);
  });

  it("still occupies a released room that runs an empty activity session", () => {
    expect(
      occupiedRoomIdsForSpontaneousStart({
        ownSupervisionRoomIds: [],
        openRooms: [{ roomId: "sporthalle", hasOccupyingSession: true }],
      }),
    ).toEqual(["sporthalle"]);
  });
});
