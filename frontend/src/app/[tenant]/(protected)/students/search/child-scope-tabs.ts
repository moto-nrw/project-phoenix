import type { TenantPageTab } from "~/components/ui/tenant-page";

/**
 * Reiter von „Alle Kinder“, wenn die Schule Kinder nur noch hier finden lässt
 * (operations.child_search_only_navigation, #1321). Sie ersetzen die
 * Navigationseinträge „Meine Gruppe“ und „Aktuelle Aufsicht“ und sind
 * Voreinstellungen der vorhandenen Filter Gruppe und Raum, keine eigene
 * Filterlogik: was ein Reiter zeigt, steht danach auch im Filter.
 */

export const ALL_CHILDREN_SCOPE = "all";
export const MY_GROUPS_SCOPE = "groups";
const ROOM_SCOPE_PREFIX = "room:";

export interface ScopeRoom {
  readonly id: string;
  readonly name: string;
}

export function roomScope(roomId: string): string {
  return `${ROOM_SCOPE_PREFIX}${roomId}`;
}

export function roomIdFromScope(scope: string): string | null {
  return scope.startsWith(ROOM_SCOPE_PREFIX)
    ? scope.slice(ROOM_SCOPE_PREFIX.length)
    : null;
}

/**
 * Die Reiter, oder `null`, wenn es nichts zu wählen gibt: ohne eigene Gruppe
 * und ohne eigene Aufsicht bliebe nur „Alle“ übrig.
 *
 * Eine Aufsicht heißt „Meine Aufsicht“. Bei mehreren trägt jeder Reiter den
 * Raum im Namen, weil der Raumfilter genau einen Raum kennt.
 */
export function buildChildScopeTabs({
  ownGroupIds,
  rooms,
}: {
  readonly ownGroupIds: readonly string[];
  readonly rooms: readonly ScopeRoom[];
}): TenantPageTab[] | null {
  if (ownGroupIds.length === 0 && rooms.length === 0) return null;
  const tabs: TenantPageTab[] = [{ value: ALL_CHILDREN_SCOPE, label: "Alle" }];
  if (ownGroupIds.length > 0) {
    tabs.push({
      value: MY_GROUPS_SCOPE,
      label: ownGroupIds.length === 1 ? "Meine Gruppe" : "Meine Gruppen",
    });
  }
  for (const room of rooms) {
    tabs.push({
      value: roomScope(room.id),
      label: rooms.length === 1 ? "Meine Aufsicht" : `Aufsicht ${room.name}`,
    });
  }
  return tabs;
}

function sameIdSet(a: readonly string[], b: readonly string[]): boolean {
  if (a.length !== b.length) return false;
  const set = new Set(a);
  return b.every((id) => set.has(id));
}

/**
 * Welcher Reiter zur aktuellen Filterlage passt. Hat jemand Gruppe oder Raum
 * im Filter anders gewählt, passt keiner — dann ist auch keiner markiert,
 * statt einen Reiter zu zeigen, dessen Liste gerade nicht zu sehen ist.
 */
export function activeChildScope({
  selectedGroupIds,
  roomId,
  ownGroupIds,
}: {
  readonly selectedGroupIds: readonly string[];
  readonly roomId: string;
  readonly ownGroupIds: readonly string[];
}): string {
  if (roomId) {
    return selectedGroupIds.length === 0 ? roomScope(roomId) : "";
  }
  if (selectedGroupIds.length === 0) return ALL_CHILDREN_SCOPE;
  return ownGroupIds.length > 0 && sameIdSet(selectedGroupIds, ownGroupIds)
    ? MY_GROUPS_SCOPE
    : "";
}
