"use client";

import { DatabaseListItem } from "~/components/database/database-list-item";
import { DatabaseListLayout } from "~/components/database/database-list-layout";
import {
  GroupedList,
  type GroupDefinition,
} from "~/components/database/grouped-list";
import { formatFloor, type Room } from "~/lib/room-helpers";

interface RoomsListProps {
  groupDefinitions: GroupDefinition<Room>[];
  /** Ziel der Raumseite für eine Zeile, samt Rückweg (`?from=`). */
  objectHref: (room: Room) => string;
}

function keyForRoom(room: Room): string {
  return room.id;
}

export function buildRoomSubtitle(room: Room): string {
  const parts: string[] = [];
  if (room.building && room.floor !== undefined) {
    parts.push(`${room.building} · ${formatFloor(room.floor)}`);
  } else if (room.building) {
    parts.push(room.building);
  } else if (room.floor !== undefined) {
    parts.push(formatFloor(room.floor));
  }
  if (room.category) parts.push(room.category);
  const base = parts.join(" · ") || "–";
  return room.isOccupied ? `Belegt · ${base}` : base;
}

/**
 * Die Sammlung der Räume in der Datenverwaltung (BAUARTEN-SPEC Bauart 1):
 * gruppierte Liste, jede Zeile ein Link auf die Raumseite `/rooms/[id]`, die
 * einzige Objektansicht eines Raums (#3115).
 */
export function RoomsList({ groupDefinitions, objectHref }: RoomsListProps) {
  return (
    <DatabaseListLayout>
      <GroupedList
        groups={groupDefinitions}
        renderItem={(room) => (
          <DatabaseListItem
            title={room.name}
            subtitle={
              <span
                className={
                  room.isOccupied ? "text-moto-orange font-medium" : undefined
                }
              >
                {buildRoomSubtitle(room)}
              </span>
            }
            isSelected={false}
            href={objectHref(room)}
          />
        )}
        keyFor={keyForRoom}
        emptyState={
          <div className="text-center text-sm text-gray-500">
            Keine Räume gefunden.
          </div>
        }
      />
    </DatabaseListLayout>
  );
}
