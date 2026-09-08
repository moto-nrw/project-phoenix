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
  /**
   * True for a permanently released room ("offener Raum", #3065). Such a room
   * is shared: reachable by every caregiver, empty or not, and seeing it grants
   * no supervision and no booking right. The flag exists so the UI can keep
   * shared rooms visibly apart from the caller's own supervisions.
   */
  isOpenRoom?: boolean;
}

/**
 * One released room as the rooms endpoint reports it
 * (GET /api/rooms?is_open_room=true).
 */
export interface OpenRoomPayload {
  id: number;
  name: string;
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
  /** Released rooms, or null when the request did not succeed. */
  openRooms: OpenRoomPayload[] | null;
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

export function sortNavigationGroups(
  groups: readonly NavigationEducationalGroup[],
): NavigationEducationalGroup[] {
  return [...groups].sort((a, b) => a.name.localeCompare(b.name, "de"));
}

/**
 * Released rooms as navigation entries. Deliberately independent of who
 * supervises what: a released room is reachable for everyone, including when
 * nothing runs in it. The room id is the identity — no synthetic tab id and no
 * name matching — so sidebar, mobile navigation and target page all address
 * the same thing.
 */
function openRoomEntries(rooms: OpenRoomPayload[] | null): SupervisedRoom[] {
  if (!rooms) return [];
  return rooms
    .map((room) => ({
      id: room.id.toString(),
      name: room.name,
      groupId: room.id.toString(),
      isOpenRoom: true,
    }))
    .sort((a, b) => a.name.localeCompare(b.name, "de"));
}

export function deriveSupervision(
  supervised: SupervisedGroupPayload[] | null,
  openRooms: OpenRoomPayload[] | null,
  overviewOk: boolean,
): DerivedSupervision {
  const openEntries = openRoomEntries(openRooms);
  const openRoomIDs = new Set(openEntries.map((room) => room.id));
  const first = supervised?.[0];

  if (!supervised || !first) {
    // No own supervision (or that request failed). Released rooms are still
    // reachable — that is the point of releasing them — but reaching one is
    // not supervising it, so isSupervising stays false and no room is
    // preselected as "mine".
    return {
      isSupervising: false,
      supervisedRooms: openEntries,
      overviewEnabled: supervised !== null && overviewOk,
    };
  }

  // Released rooms are contributed once, from the room list. A supervision
  // that happens to run in one of them must not add a second entry for the
  // same place — that is the duplicate-tab problem this replaces.
  const eligible = supervised.filter(
    (g) => g.room_id && g.room && !openRoomIDs.has(g.room_id.toString()),
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
    // Own supervisions first, shared rooms after them: the order is the
    // separation the criterion asks for, and isOpenRoom carries it for any
    // stronger treatment the UI wants.
    supervisedRooms: [...rooms, ...openEntries],
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
