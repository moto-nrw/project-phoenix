import type { BackendRoom, Room } from "~/lib/room-helpers";

let roomIdCounter = 0;

function nextId() {
  return ++roomIdCounter;
}

/**
 * Builds a BackendRoom with sensible defaults.
 * Uses the extended BackendRoom type from room-helpers.ts
 */
export function buildBackendRoom(
  overrides?: Partial<BackendRoom>,
): BackendRoom {
  const id = overrides?.id ?? nextId();
  return {
    id,
    name: "Room 101",
    building: "Building A",
    floor: 2,
    capacity: 30,
    category: "classroom",
    color: "#FF0000",
    is_occupied: false,
    created_at: "2024-01-15T10:00:00Z",
    updated_at: "2024-01-15T12:00:00Z",
    ...overrides,
  };
}

/**
 * Builds a frontend Room with sensible defaults.
 */
export function buildRoom(overrides?: Partial<Room>): Room {
  const id = overrides?.id ?? String(nextId());
  return {
    id,
    name: "Room 101",
    building: "Building A",
    floor: 2,
    capacity: 30,
    category: "classroom",
    color: "#FF0000",
    isOccupied: false,
    ...overrides,
  };
}

/** Reset the auto-increment counter. */
export function resetRoomIds() {
  roomIdCounter = 0;
}
