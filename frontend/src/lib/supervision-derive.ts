import type { NavigationEducationalGroup } from "~/lib/usercontext-helpers";

/**
 * Pure derivation of the supervision navigation state from the three backend
 * payloads (group navigation, supervised groups, Schulhof status). Shared by
 * SupervisionProvider (browser fetches through the Next API routes) and the
 * server-side shell bootstrap (#2973), so both sides produce the same rooms.
 */

export interface SupervisedRoom {
  id: string;
  name: string;
  groupId: string;
  groupName?: string;
  /** Special flag for the permanent Schulhof tab. */
  isSchulhof?: boolean;
}

export interface SchulhofStatus {
  exists: boolean;
  room_id?: number;
  room_name: string;
  active_group_id?: number;
  is_user_supervising: boolean;
}

/** One entry of /api/active/supervisors/all or /api/me/groups/supervised. */
export interface SupervisedGroupPayload {
  id: number;
  room_id?: number;
  group_id: number;
  room?: { id: number; name: string };
  actual_group?: { id: number; name: string };
}

/**
 * Raw payloads as fetched, before derivation. `null` means the request did
 * not succeed (non-OK response, network error, or not attempted).
 */
export interface SupervisionSnapshot {
  groups: NavigationEducationalGroup[] | null;
  supervised: SupervisedGroupPayload[] | null;
  schulhof: SchulhofStatus | null;
  /** True when `supervised` came from the school-wide overview endpoint. */
  overviewOk: boolean;
}

export interface DerivedSupervision {
  isSupervising: boolean;
  supervisedRoomId?: string;
  supervisedRoomName?: string;
  supervisedRooms: SupervisedRoom[];
  overviewEnabled: boolean;
}

const SCHULHOF_ROOM_NAME = "Schulhof";
const SCHULHOF_TAB_ID = "schulhof";

export function sortNavigationGroups(
  groups: readonly NavigationEducationalGroup[],
): NavigationEducationalGroup[] {
  return [...groups].sort((a, b) => a.name.localeCompare(b.name, "de"));
}

function schulhofRoom(status: SchulhofStatus | null): SupervisedRoom | null {
  // Intentionally check `exists` only, NOT `is_user_supervising`. The
  // Schulhof tab must be visible to ALL staff so anyone can opt in to
  // supervise; `is_user_supervising` is for UI hints, never tab visibility.
  if (!status?.exists) return null;
  return {
    id: SCHULHOF_TAB_ID,
    name: SCHULHOF_ROOM_NAME,
    groupId: status.active_group_id?.toString() ?? SCHULHOF_TAB_ID,
    isSchulhof: true,
  };
}

export function deriveSupervision(
  supervised: SupervisedGroupPayload[] | null,
  schulhof: SchulhofStatus | null,
  overviewOk: boolean,
): DerivedSupervision {
  const schulhofEntry = schulhofRoom(schulhof);
  const first = supervised?.[0];

  if (!supervised || !first) {
    // No regular supervision (or the request failed): Schulhof alone still
    // counts, so anyone can join it.
    return {
      isSupervising: schulhofEntry !== null,
      supervisedRoomId: schulhofEntry ? SCHULHOF_TAB_ID : undefined,
      supervisedRoomName: schulhofEntry ? SCHULHOF_ROOM_NAME : undefined,
      supervisedRooms: schulhofEntry ? [schulhofEntry] : [],
      overviewEnabled: supervised !== null && overviewOk,
    };
  }

  // Schulhof is handled separately, so keep it out of the regular rooms.
  const eligible = supervised.filter(
    (g) => g.room_id && g.room && g.room.name !== SCHULHOF_ROOM_NAME,
  );
  // Parallel sessions can share one room (#2265): a room-name-only label
  // would render indistinguishable entries, so suffix the activity name
  // whenever a room appears more than once.
  const roomUseCount = new Map<number, number>();
  for (const g of eligible) {
    roomUseCount.set(g.room_id!, (roomUseCount.get(g.room_id!) ?? 0) + 1);
  }
  const rooms: SupervisedRoom[] = eligible
    .map((g) => {
      const roomName = g.room?.name ?? `Room ${g.room_id}`;
      const shared = (roomUseCount.get(g.room_id!) ?? 0) > 1;
      return {
        id: g.room_id!.toString(),
        name:
          shared && g.actual_group?.name
            ? `${g.actual_group.name} · ${roomName}`
            : roomName,
        groupId: g.id.toString(),
        groupName: g.actual_group?.name,
      };
    })
    .sort((a, b) => a.name.localeCompare(b.name, "de"));

  return {
    isSupervising: true,
    supervisedRoomId: first.room_id?.toString(),
    supervisedRoomName:
      first.room?.name ?? (first.room_id ? `Room ${first.room_id}` : undefined),
    supervisedRooms: schulhofEntry ? [...rooms, schulhofEntry] : rooms,
    overviewEnabled: overviewOk,
  };
}

export function sameSupervision(
  prev: DerivedSupervision,
  next: DerivedSupervision,
): boolean {
  // Active groups can change while the physical room stays the same, so the
  // group id is part of the identity.
  const keys = (rooms: SupervisedRoom[]) =>
    rooms.map((r) => `${r.id}:${r.groupId}`).join(",");
  return (
    prev.isSupervising === next.isSupervising &&
    prev.supervisedRoomId === next.supervisedRoomId &&
    prev.supervisedRoomName === next.supervisedRoomName &&
    prev.overviewEnabled === next.overviewEnabled &&
    keys(prev.supervisedRooms) === keys(next.supervisedRooms)
  );
}

export function sameGroups(
  prev: readonly NavigationEducationalGroup[],
  next: readonly NavigationEducationalGroup[],
): boolean {
  return (
    prev.length === next.length &&
    prev.every((group, index) => {
      const other = next[index];
      return (
        group.id === other?.id &&
        group.name === other.name &&
        group.room_id === other.room_id &&
        group.via_substitution === other.via_substitution &&
        group.is_personal === other.is_personal
      );
    })
  );
}
