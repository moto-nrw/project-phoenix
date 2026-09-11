import type { Student } from "~/lib/student-helpers";

export const SCHULHOF_ROOM_NAME = "Schulhof";

export interface MinimalActiveGroup {
  id: string;
  room?: { name?: string };
}

export interface ActiveSupervisionStudent extends Student {
  activeGroupId: string;
  checkInTime: Date;
  activity_name?: string;
}

export interface ActiveSupervisionRoom {
  id: string;
  name: string;
  room_name?: string;
  room_id?: string;
  room_color?: string;
  student_count?: number;
  supervisor_name?: string;
  isCurrentUserSupervising?: boolean;
  canAssign?: boolean;
  students?: ActiveSupervisionStudent[];
}

export interface SchulhofStatusResponse {
  exists: boolean;
  roomId: string | null;
  roomName: string;
  activityGroupId: string | null;
  activeGroupId: string | null;
  isUserSupervising: boolean;
  supervisionId: string | null;
  supervisorCount: number;
  studentCount: number;
  supervisors: Array<{
    id: string;
    staffId: string;
    name: string;
    isCurrentUser: boolean;
  }>;
}

/**
 * One permanently released room ("offener Raum", #3065) with everyone
 * currently recorded in it.
 *
 * A released room is shared: reachable by every caregiver, empty or not, and
 * opening it grants no supervision. `isUserSupervising` exists only so the
 * screen can keep it visibly apart from the caller's own supervisions.
 * `activeGroupIds` lists the sessions that feed the view — several of them is
 * the normal case, which is why the room, not a session, is the identity here.
 */
export interface OpenRoomView {
  readonly roomId: string;
  readonly name: string;
  readonly isUserSupervising: boolean;
  readonly activeGroupIds: readonly string[];
  readonly studentCount: number;
  readonly students: readonly VisitDisplayLike[];
}

export type SupervisionSelectionTarget =
  | { kind: "open-room"; roomId: string }
  | { kind: "session"; sessionId: string }
  | { kind: "persist-first" }
  | { kind: "none" };

/**
 * Resolves what the page should select: one of the caller's own supervision
 * sessions, or one released room's shared view.
 *
 * Own sessions are keyed by active-group ID (`?session=`), never by physical
 * room: several parallel sessions can share one room (#2265), so a room-keyed
 * URL is ambiguous for them. Released rooms are the opposite case — the room
 * IS the thing, every session in it feeds one view — so they are keyed by room
 * ID (`?room=`, #3065). The legacy `?room=` entry point for ordinary rooms
 * still resolves, and still never switches away from a session already
 * selected in that room, which was the silent-switch bug behind contradictory
 * attendance views.
 */
export function resolveSupervisionSelection(options: {
  readonly sessionParam: string | null;
  readonly roomParam: string | null;
  readonly savedSessionId: string | null;
  readonly savedRoomId: string | null;
  readonly rooms: readonly ActiveSupervisionRoom[];
  readonly currentSessionId: string | null;
  readonly currentOpenRoomId: string | null;
  /** Room ids of the released rooms, as the dashboard reported them. */
  readonly openRoomIds: ReadonlySet<string>;
}): SupervisionSelectionTarget {
  const { rooms, currentSessionId, currentOpenRoomId, openRoomIds } = options;

  const sessionTarget = (
    session: ActiveSupervisionRoom,
  ): SupervisionSelectionTarget => {
    if (session.room_id && openRoomIds.has(session.room_id)) {
      return session.room_id === currentOpenRoomId
        ? { kind: "none" }
        : { kind: "open-room", roomId: session.room_id };
    }
    if (session.id === currentSessionId) return { kind: "none" };
    return { kind: "session", sessionId: session.id };
  };
  const roomTarget = (roomId: string): SupervisionSelectionTarget | null => {
    // A released room is answered by its shared view, never by one of the
    // sessions running in it: which session a child happens to be recorded
    // under is not what the room is addressed by (#3065).
    if (openRoomIds.has(roomId)) {
      return roomId === currentOpenRoomId
        ? { kind: "none" }
        : { kind: "open-room", roomId };
    }
    const inRoom = rooms.filter((room) => room.room_id === roomId);
    if (inRoom.length === 0) return null;
    if (inRoom.some((room) => room.id === currentSessionId)) {
      // A session in this room is already selected — never silently switch
      // to a sibling session just because the URL names the shared room.
      return { kind: "none" };
    }
    const first = inRoom[0];
    return first ? { kind: "session", sessionId: first.id } : null;
  };

  if (options.sessionParam) {
    const found = rooms.find((room) => room.id === options.sessionParam);
    if (found) return sessionTarget(found);
    // A stale session must not block the saved-room fallback below. This is
    // common when returning from a detail page after that session ended.
  }
  if (options.roomParam) {
    return roomTarget(options.roomParam) ?? { kind: "none" };
  }
  if (options.savedSessionId) {
    const found = rooms.find((room) => room.id === options.savedSessionId);
    if (found) return sessionTarget(found);
  }
  if (options.savedRoomId) {
    const target = roomTarget(options.savedRoomId);
    if (target) return target;
  }

  // With no explicit target, prefer an own session that does not already sit
  // in a released room. The navigation has one entry for a released room, not
  // one per session in it, so selecting such a session by default would leave
  // the sidebar without a matching entry. If every running session is in a
  // released room, select that session's room — the matching sidebar row —
  // not the first released room by name. Home "Zur Aufsicht" lands here with
  // no query.
  if (openRoomIds.size > 0) {
    const ownSession = rooms.find(
      (room) => !room.room_id || !openRoomIds.has(room.room_id),
    );
    if (ownSession) return sessionTarget(ownSession);
    const firstOwn = rooms[0];
    if (firstOwn) return sessionTarget(firstOwn);
    const firstOpenRoomId = openRoomIds.values().next().value as
      string | undefined;
    if (firstOpenRoomId) {
      return (
        roomTarget(firstOpenRoomId) ?? {
          kind: "open-room",
          roomId: firstOpenRoomId,
        }
      );
    }
  }
  return { kind: "persist-first" };
}

export function activeSupervisionRosterKey(options: {
  readonly selectedTimetableInstanceId: string | null;
  readonly currentRoomId: string | null | undefined;
  readonly missingRosterActiveGroupIds: ReadonlySet<string>;
}): string | null {
  if (options.selectedTimetableInstanceId) {
    return `timetable-roster-${options.selectedTimetableInstanceId}`;
  }
  if (
    !options.currentRoomId ||
    options.missingRosterActiveGroupIds.has(options.currentRoomId)
  ) {
    return null;
  }
  return `timetable-roster-active-group-${options.currentRoomId}`;
}

/**
 * The caller's own supervisions that a shared room entry does not already
 * stand for.
 *
 * A session running in a released room is part of that room's shared view, so
 * listing it again would put the same place on screen twice — once per
 * parallel session, which is the duplicate-tab problem #3065 removes.
 */
export function sessionsOutsideOpenRooms(
  rooms: readonly ActiveSupervisionRoom[],
  openRoomIds: ReadonlySet<string>,
): ActiveSupervisionRoom[] {
  if (openRoomIds.size === 0) return [...rooms];
  return rooms.filter(
    (room) => !room.room_id || !openRoomIds.has(room.room_id),
  );
}

/**
 * Selects the caller's session to retain while opening a shared room.
 *
 * A previously selected session is only meaningful in the target room. This
 * keeps the roster after starting that exact session, while preventing its
 * controls from leaking into another shared room.
 */
export function openRoomSessionSelection(options: {
  readonly roomId: string;
  readonly preferredSessionId: string | undefined;
  readonly selectedSessionId: string | null;
  readonly rooms: readonly ActiveSupervisionRoom[];
}): { sessionId: string | null; keepsTimetableInstance: boolean } {
  const sessionId = options.preferredSessionId ?? options.selectedSessionId;
  const sessionRunsInRoom =
    sessionId !== null &&
    options.rooms.some(
      (room) => room.id === sessionId && room.room_id === options.roomId,
    );

  return {
    sessionId: sessionRunsInRoom ? sessionId : null,
    // A URL can name the session while an unrelated instance is still in
    // memory. Only retain an instance that was already selected in this room.
    keepsTimetableInstance:
      sessionRunsInRoom && options.selectedSessionId === sessionId,
  };
}

/**
 * The caller's single active session in a shared room, used to keep that
 * session's roster actionable without turning the room back into a
 * session-keyed navigation entry.
 *
 * Only the room's own sessions count. Intersecting with the caller's
 * `supervisedGroups` (or a leftover `?session=` / last-session id) still
 * looks like "one session" when someone else runs a second offering in the
 * same room — and that offering's children then disappear behind the
 * caller's roster (#3065). Several sessions in the room is the same
 * ambiguity `additionalSupervisionTarget` already treats as "none".
 */
export function openRoomRosterActiveGroupId(options: {
  readonly currentOpenRoom: OpenRoomView | null;
}): string | null {
  const { currentOpenRoom } = options;
  if (!currentOpenRoom?.isUserSupervising) return null;
  if (currentOpenRoom.activeGroupIds.length !== 1) return null;
  return currentOpenRoom.activeGroupIds[0] ?? null;
}

export interface VisitDisplayLike {
  studentId: string;
  studentName?: string;
  schoolClass?: string;
  groupName?: string;
  activityName?: string;
  activeGroupId: string;
  checkInTime: string | Date;
  actualArrivalTime?: string;
  actualPickupTime?: string;
  isActive: boolean;
  sick?: boolean;
  sickSince?: string;
  excused?: boolean;
  excusedSince?: string;
  photoUrl?: string;
}

interface SupervisedGroupLike {
  id: string;
  name: string;
  room_id?: string;
  room?: { id: string; name: string; color?: string | null };
  isCurrentUserSupervising?: boolean;
  canAssign?: boolean;
}

interface EducationalGroupLike {
  id: string;
  name: string;
  room?: { name: string };
}

export function buildGroupNameToIdMap(
  groups: readonly EducationalGroupLike[],
): Map<string, string> {
  const nameToIdMap = new Map<string, string>();
  groups.forEach((group) => {
    if (group.name) {
      nameToIdMap.set(group.name, group.id);
    }
  });
  return nameToIdMap;
}

export function mapSupervisedGroupsToRooms(
  groups: readonly SupervisedGroupLike[],
): ActiveSupervisionRoom[] {
  return groups
    .map((group) => ({
      id: group.id,
      name: group.name,
      room_name: group.room?.name,
      room_id: group.room_id,
      room_color: group.room?.color ?? undefined,
      student_count: undefined,
      supervisor_name: undefined,
      isCurrentUserSupervising: group.isCurrentUserSupervising === true,
      canAssign: group.canAssign === true,
    }))
    .sort(
      (a, b) =>
        // Same room → order by session name so parallel sessions (#2265)
        // keep a deterministic tab order; fully equal entries keep the
        // backend order (Array.prototype.sort is stable). The active-group
        // payload may carry no name at runtime, so compare defensively.
        (a.room_name ?? a.name ?? "").localeCompare(
          b.room_name ?? b.name ?? "",
          "de",
        ) || (a.name ?? "").localeCompare(b.name ?? "", "de"),
    );
}

/** Title + plan window of a running timetable session (#2265). */
export interface SupervisionSessionInfo {
  readonly title: string;
  readonly timeRange: string;
}

/**
 * Tab label for a supervision session: the instance title plus its plan time
 * when the running timetable session is known, else session/room names.
 * Room-name-only labels made parallel sessions in one room
 * indistinguishable (#2265). Defensive against missing names — the live
 * active-group payload does not always carry one.
 */
export function supervisionTabLabel(
  room: ActiveSupervisionRoom,
  liveSession?: SupervisionSessionInfo | null,
): string {
  let label: string;
  if (liveSession) {
    label = `${liveSession.title} · ${liveSession.timeRange}`;
  } else {
    const sessionName: string | undefined = room.name;
    const roomName = room.room_name;
    label =
      sessionName && roomName && sessionName !== roomName
        ? `${sessionName} · ${roomName}`
        : (sessionName ?? roomName ?? "Aufsicht");
  }
  return room.isCurrentUserSupervising ? `${label} · Eigene Aufsicht` : label;
}

/**
 * The session a further supervisor would be added to (#2806), or null when
 * there is no unambiguous one.
 *
 * In a shared room that is the caller's own single session there. With several
 * sessions running, none of them is "the" supervision of the room, so the
 * screen offers nothing rather than picking one.
 */
export function additionalSupervisionTarget(options: {
  readonly currentRoom: ActiveSupervisionRoom | null;
  readonly currentOpenRoom: OpenRoomView | null;
}): string | null {
  if (options.currentOpenRoom) {
    const { isUserSupervising, activeGroupIds } = options.currentOpenRoom;
    if (!isUserSupervising || activeGroupIds.length !== 1) return null;
    return activeGroupIds[0] ?? null;
  }
  return options.currentRoom?.canAssign ? options.currentRoom.id : null;
}

function mapVisitToSupervisionStudent(
  visit: VisitDisplayLike,
  options: {
    roomName?: string;
    roomColor?: string | null;
    groupNameToId?: Map<string, string>;
  },
): ActiveSupervisionStudent {
  const nameParts = visit.studentName?.split(" ") ?? ["", ""];
  const firstName = nameParts[0] ?? "";
  const lastName = nameParts.slice(1).join(" ") ?? "";
  const location = options.roomName
    ? `Anwesend - ${options.roomName}`
    : "Anwesend";
  const groupId = visit.groupName
    ? options.groupNameToId?.get(visit.groupName)
    : undefined;

  return {
    id: visit.studentId,
    name: visit.studentName ?? "",
    first_name: firstName,
    second_name: lastName,
    school_class: visit.schoolClass ?? "",
    current_location: location,
    current_room_color: options.roomColor ?? null,
    group_name: visit.groupName,
    group_id: groupId,
    activity_name: visit.activityName,
    sick: visit.sick,
    sick_since: visit.sickSince,
    excused: visit.excused,
    excused_since: visit.excusedSince,
    actual_arrival_time: visit.actualArrivalTime,
    actual_pickup_time: visit.actualPickupTime,
    activeGroupId: visit.activeGroupId,
    checkInTime:
      visit.checkInTime instanceof Date
        ? visit.checkInTime
        : new Date(visit.checkInTime),
    photo_url: visit.photoUrl,
  } as ActiveSupervisionStudent;
}

export function mapVisitsToSupervisionStudents(
  visits: readonly VisitDisplayLike[],
  options: {
    roomName?: string;
    roomColor?: string | null;
    groupNameToId?: Map<string, string>;
  },
): ActiveSupervisionStudent[] {
  return visits
    .filter((visit) => visit.isActive)
    .map((visit) => mapVisitToSupervisionStudent(visit, options));
}

export function withActiveSupervisionPresence(
  student: ActiveSupervisionStudent,
): ActiveSupervisionStudent {
  return {
    ...student,
    sick: false,
    sick_since: undefined,
    excused: false,
    excused_since: undefined,
    class_trip: false,
    class_trip_since: undefined,
    day_planning_status:
      student.day_planning_status === "not_coming_today"
        ? undefined
        : student.day_planning_status,
    day_planning_reason: undefined,
    day_planning_label: undefined,
  };
}
