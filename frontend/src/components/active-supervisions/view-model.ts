import type { Student } from "~/lib/student-helpers";

export const SCHULHOF_ROOM_NAME = "Schulhof";
export const SCHULHOF_TAB_ID = "schulhof";

export interface MinimalActiveGroup {
  id: string;
  room?: { name?: string };
}

export interface ActiveSupervisionStudent extends Student {
  activeGroupId: string;
  checkInTime: Date;
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

export type SupervisionSelectionTarget =
  | { kind: "schulhof" }
  | { kind: "session"; sessionId: string }
  | { kind: "persist-first" }
  | { kind: "none" };

/**
 * Resolves which supervision session (active group) the page should select.
 *
 * Sessions are keyed by active-group ID (`?session=`), never by physical room:
 * several parallel sessions can share one room (#2265), so a room-keyed URL is
 * ambiguous. The legacy `?room=` entry point (sidebar, old links) still
 * resolves — but never switches away from a session already selected in that
 * room, which was the silent-switch bug behind contradictory attendance views.
 */
export function resolveSupervisionSelection(options: {
  readonly sessionParam: string | null;
  readonly roomParam: string | null;
  readonly savedSessionId: string | null;
  readonly savedRoomId: string | null;
  readonly rooms: readonly ActiveSupervisionRoom[];
  readonly currentSessionId: string | null;
  readonly schulhofAvailable: boolean;
}): SupervisionSelectionTarget {
  const { rooms, currentSessionId } = options;

  const sessionTarget = (sessionId: string): SupervisionSelectionTarget => {
    if (sessionId === currentSessionId) return { kind: "none" };
    return { kind: "session", sessionId };
  };
  const roomTarget = (roomId: string): SupervisionSelectionTarget | null => {
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

  if (options.sessionParam === SCHULHOF_TAB_ID && options.schulhofAvailable) {
    return { kind: "schulhof" };
  }
  if (options.sessionParam) {
    const found = rooms.find((room) => room.id === options.sessionParam);
    if (found) return sessionTarget(found.id);
    // A stale session must not block the saved-room fallback below. This is
    // common when returning from a detail page after that session ended.
  }
  if (options.roomParam === SCHULHOF_TAB_ID && options.schulhofAvailable) {
    return { kind: "schulhof" };
  }
  if (options.roomParam) {
    return roomTarget(options.roomParam) ?? { kind: "none" };
  }
  if (options.savedSessionId === SCHULHOF_TAB_ID && options.schulhofAvailable) {
    return { kind: "schulhof" };
  }
  if (options.savedSessionId) {
    const found = rooms.find((room) => room.id === options.savedSessionId);
    if (found) return sessionTarget(found.id);
  }
  if (options.savedRoomId === SCHULHOF_TAB_ID && options.schulhofAvailable) {
    return { kind: "schulhof" };
  }
  if (options.savedRoomId) {
    const target = roomTarget(options.savedRoomId);
    if (target) return target;
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

export function roomsOutsideSchulhofStatus(
  rooms: readonly ActiveSupervisionRoom[],
  options: {
    readonly schulhofTabEnabled: boolean;
    readonly statusActiveGroupId: string | null | undefined;
  },
): ActiveSupervisionRoom[] {
  if (!options.schulhofTabEnabled || !options.statusActiveGroupId) {
    return [...rooms];
  }

  return rooms.filter((room) => room.id !== options.statusActiveGroupId);
}

interface VisitDisplayLike {
  studentId: string;
  studentName?: string;
  schoolClass?: string;
  groupName?: string;
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

export function additionalSupervisionTarget(options: {
  readonly currentRoom: ActiveSupervisionRoom | null;
  readonly isSchulhofTabSelected: boolean;
  readonly schulhofStatus: Pick<
    SchulhofStatusResponse,
    "activeGroupId" | "isUserSupervising"
  > | null;
}): string | null {
  if (options.isSchulhofTabSelected) {
    return options.schulhofStatus?.isUserSupervising
      ? (options.schulhofStatus.activeGroupId ?? null)
      : null;
  }
  return options.currentRoom?.id ?? null;
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
