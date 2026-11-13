// lib/move-targets.ts
//
// The rooms and activities children can be put into from the web app. One
// list for every place that offers a target: moving children out of a room
// (#2969, #3066) and checking them in straight into a room or an activity
// (#6953). The backend checks every choice again; this list only avoids
// offering a choice that is sure to be refused.

import type { Session } from "next-auth";

import type { ActiveGroup } from "~/lib/active-helpers";
import { activeService, type StudentMoveResult } from "~/lib/active-service";
import type { Room } from "~/lib/room-helpers";

/**
 * One target. A `session` target puts children into a running session; an
 * `openRoom` target records an independent stay in a released room (#3066),
 * whatever runs there.
 */
export type TargetRoomOption =
  | {
      readonly kind: "session";
      readonly value: string;
      readonly activeGroupId: string;
      readonly roomId: string;
      readonly roomName: string;
      /** The activity running in the session, when it has a name. */
      readonly activityName?: string;
      /** Text in the target select; `roomName` names the room in messages. */
      readonly label: string;
    }
  | {
      readonly kind: "openRoom";
      readonly value: string;
      readonly roomId: string;
      readonly roomName: string;
      readonly label: string;
    };

/**
 * Which rooms may be offered as targets (#2969):
 * - `all`: admins see every room with exactly one running session.
 * - `supervised`: every room whose single running session has at least one
 *   supervisor (colleagues count), every session of the caller, and every
 *   released room. A supervisor of the source room may push there, and a
 *   child who just arrived has no source room, so the same list applies to
 *   a check-in (#6953).
 * - `own`: everyone else may only pull into rooms they supervise themselves.
 */
export type TargetScope = "all" | "supervised" | "own";

/**
 * Checks the children in and places them in the target in one request
 * (#6953). `keepPresent` leaves children who are already present where they
 * are; without it they move under the ordinary move rules.
 */
export function checkInToTarget(
  studentIds: string[],
  target: TargetRoomOption,
  keepPresent: boolean,
): Promise<StudentMoveResult> {
  return target.kind === "openRoom"
    ? activeService.checkInStudentsToOpenRoom(
        studentIds,
        target.roomId,
        keepPresent,
      )
    : activeService.checkInStudentsToActiveGroup(
        studentIds,
        target.activeGroupId,
        keepPresent,
      );
}

/** Admins may target any running module; everyone else follows the scope. */
export function canUseAllMoveTargets(session: Session | null): boolean {
  const permissions = session?.user?.permissions ?? [];
  return (
    session?.user?.isAdmin === true ||
    session?.user?.roles?.includes("admin") === true ||
    permissions.includes("admin:*") ||
    permissions.includes("*:*")
  );
}

/**
 * The targets for the given scope. `currentRoomId` skips the sessions of the
 * room the children are in; pass `""` when they have no room yet.
 */
export function buildTargetRoomOptions(
  activeGroups: readonly ActiveGroup[],
  rooms: readonly Room[],
  currentRoomId: string,
  scope: TargetScope,
  ownActiveGroupIds: ReadonlySet<string>,
): TargetRoomOption[] {
  const roomsById = new Map(rooms.map((room) => [room.id, room]));
  const groupsByRoomId = new Map<string, ActiveGroup[]>();

  // Group BEFORE filtering by eligibility: a room with one supervised and
  // one unsupervised session is still ambiguous and must not be offered.
  // Session targets skip the current room; the independent stay of this
  // released room is offered separately below.
  activeGroups.forEach((group) => {
    if (!group.isActive || group.roomId === currentRoomId) return;
    const groupsInRoom = groupsByRoomId.get(group.roomId) ?? [];
    groupsInRoom.push(group);
    groupsByRoomId.set(group.roomId, groupsInRoom);
  });

  // A released room is a shared destination (#3066): choosing it records an
  // independent stay, so it is offered as the room itself. That includes the
  // current room, so children in an activity there can stay without the
  // offering. Moving there needs rights over the children's current place,
  // which pull-only staff ("own") do not have.
  const openRoomOptions: TargetRoomOption[] =
    scope === "own"
      ? []
      : rooms
          .filter((room) => room.isOpenRoom)
          .map((room) => ({
            kind: "openRoom",
            value: `room:${room.id}`,
            roomId: room.id,
            roomName: room.name,
            label: `${room.name} (offener Raum)`,
          }));

  const sessionOptions = [...groupsByRoomId.entries()].flatMap(
    ([targetRoomId, groups]) => {
      const room = roomsById.get(targetRoomId);
      // Independent stays in a released room are the open-room target, not a
      // session. Supervised activities in that room stay pull/push targets.
      const sessionGroups =
        room?.isOpenRoom === true
          ? groups.filter((group) => (group.supervisorCount ?? 0) > 0)
          : groups;
      const ownGroups = sessionGroups.filter((group) =>
        ownActiveGroupIds.has(group.id),
      );
      const unambiguousGroup =
        sessionGroups.length === 1 ? sessionGroups[0] : undefined;

      const eligibleGroups =
        scope === "all"
          ? unambiguousGroup
            ? [unambiguousGroup]
            : []
          : scope === "own"
            ? ownGroups
            : [
                ...ownGroups,
                ...(unambiguousGroup &&
                !ownActiveGroupIds.has(unambiguousGroup.id) &&
                (unambiguousGroup.supervisorCount ?? 0) > 0
                  ? [unambiguousGroup]
                  : []),
              ];

      const roomName = room?.name ?? `Raum ${targetRoomId}`;
      return eligibleGroups.map((activeGroup): TargetRoomOption => {
        const activityName = activeGroup.actualGroup?.name?.trim();
        return {
          kind: "session",
          value: activeGroup.id,
          activeGroupId: activeGroup.id,
          roomId: targetRoomId,
          roomName,
          ...(activityName ? { activityName } : {}),
          label: roomName,
        };
      });
    },
  );

  return [...sessionOptions, ...openRoomOptions].sort((a, b) =>
    a.label.localeCompare(b.label, "de"),
  );
}
