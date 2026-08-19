// lib/room-helpers.ts
// Type definitions and helper functions for rooms

import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RoomHelpers" });

// Hex accent per room category — used by the rooms grid card and the
// room-detail history timeline so a category reads the same way in both
// places. Fallback for unknown / missing categories is the neutral
// gray below. Single source of truth: previously this map was duplicated
// in rooms/page.tsx and components/rooms/room-detail-content.tsx.
const ROOM_CATEGORY_COLORS: Record<string, string> = {
  "Normaler Raum": "#4F46E5",
  Gruppenraum: "#10B981",
  Themenraum: "#8B5CF6",
  Sport: "#EC4899",
};

const ROOM_CATEGORY_FALLBACK_COLOR = "#6B7280";

// Status sentinel the /api/rooms/[id]/history proxy emits when the backend
// has gated the endpoint off via gdpr.attendance_log_enabled (issue #1425).
// The drawer reads this to hide the "Belegungshistorie" section deliberately
// rather than letting it collapse incidentally on an empty history array.
// Shared so the proxy route and the drawer can't drift; both sides import
// from here.
export const ROOM_HISTORY_STATUS_FEATURE_DISABLED = "feature_disabled";

export function getRoomCategoryColor(
  category: string | null | undefined,
): string {
  if (!category) return ROOM_CATEGORY_FALLBACK_COLOR;
  return ROOM_CATEGORY_COLORS[category] ?? ROOM_CATEGORY_FALLBACK_COLOR;
}

// Backend types (from Go structs)
export interface BackendRoom {
  id: number;
  name: string; // Changed to match backend API which uses "name"
  building?: string;
  floor?: number | null; // Optional (nullable in DB)
  capacity?: number | null; // Optional (nullable in DB)
  category?: string | null; // Optional (nullable in DB)
  color?: string | null; // Optional (nullable in DB)
  device_id?: string;
  is_occupied: boolean;
  activity_name?: string;
  group_name?: string;
  supervisor_name?: string; // Legacy singular field
  supervisor_names?: string; // New: comma-separated list of supervisors
  student_count?: number;
  created_at: string;
  updated_at: string;
}

// Frontend types
export interface Room {
  id: string;
  name: string;
  building?: string;
  floor?: number; // Optional (nullable in DB)
  capacity?: number; // Optional (nullable in DB)
  category?: string; // Optional (nullable in DB)
  color?: string; // Optional (nullable in DB)
  deviceId?: string;
  isOccupied: boolean;
  activityName?: string;
  groupName?: string;
  supervisorName?: string;
  studentCount?: number;
  createdAt?: string;
  updatedAt?: string;
}

// System room names that cannot be deleted or renamed.
// Must stay in sync with backend constants/activities.go
// (SchulhofRoomName, WCRoomName, WCRoomAliasName) and PyrePortal
// src/services/api.ts (WC_ROOM_ALIASES). Matching is exact-case to mirror
// the backend's `name == SchulhofRoomName || IsWCRoomName(name)` check.
export const SCHULHOF_ROOM_NAME = "Schulhof";
const SYSTEM_ROOM_NAMES = [SCHULHOF_ROOM_NAME, "WC", "Toilette"] as const;

/** Returns true if the room is a system room (Schulhof, WC, Toilette). */
export function isSystemRoom(room: Room | null | undefined): boolean {
  if (!room) return false;
  return SYSTEM_ROOM_NAMES.includes(
    room.name as (typeof SYSTEM_ROOM_NAMES)[number],
  );
}

// Toilet-room aliases — the subset of system rooms whose color is NOT
// admin-configurable. The Schulhof left this set with #2405: schools
// color-code rooms and tablets and need the yard to take part, so it keeps
// only the rename/delete protection. The WC has no badge of its own, so a
// color picker there would configure nothing.
const COLOR_LOCKED_ROOM_NAMES = ["WC", "Toilette"] as const;

/**
 * Returns true if the room's color is fixed by the system and must not be
 * offered for editing. Mirrors the backend guard in
 * services/facilities.UpdateRoom, which rejects color changes for exactly
 * these rooms.
 */
export function isColorLockedRoom(room: Room | null | undefined): boolean {
  if (!room) return false;
  return COLOR_LOCKED_ROOM_NAMES.includes(
    room.name as (typeof COLOR_LOCKED_ROOM_NAMES)[number],
  );
}

/** Returns true if the room is the canonical Schulhof (schoolyard). */
export function isSchulhofRoom(room: Room | null | undefined): boolean {
  return room?.name === SCHULHOF_ROOM_NAME;
}

// Mapping functions
export function mapRoomResponse(backendRoom: BackendRoom): Room {
  return {
    id: String(backendRoom.id),
    name: backendRoom.name, // Changed from room_name to name to match backend API
    building: backendRoom.building,
    // Convert null to undefined for optional fields
    floor: backendRoom.floor ?? undefined,
    capacity:
      backendRoom.capacity !== null &&
      backendRoom.capacity !== undefined &&
      backendRoom.capacity > 0
        ? backendRoom.capacity
        : undefined,
    category: backendRoom.category ?? undefined,
    color: backendRoom.color ?? undefined,
    deviceId: backendRoom.device_id,
    isOccupied: backendRoom.is_occupied,
    activityName: backendRoom.activity_name,
    groupName: backendRoom.group_name,
    supervisorName: backendRoom.supervisor_names ?? backendRoom.supervisor_name,
    studentCount: backendRoom.student_count,
    createdAt: backendRoom.created_at,
    updatedAt: backendRoom.updated_at,
  };
}

export function mapRoomsResponse(
  backendRooms: BackendRoom[] | null | { data: BackendRoom[] },
): Room[] {
  // Handle nested API response structure (from server API)
  if (
    backendRooms &&
    typeof backendRooms === "object" &&
    "data" in backendRooms &&
    Array.isArray(backendRooms.data)
  ) {
    logger.debug("handling nested API response for rooms");
    return backendRooms.data.map(mapRoomResponse);
  }

  // Handle null, undefined or non-array responses
  if (!backendRooms || !Array.isArray(backendRooms)) {
    logger.warn("received invalid response format for rooms", {
      received: typeof backendRooms,
    });
    return [];
  }

  // Standard array response
  return backendRooms.map(mapRoomResponse);
}

export function mapSingleRoomResponse(response: { data: BackendRoom }): Room {
  return mapRoomResponse(response.data);
}

// Prepare frontend room for backend
export function prepareRoomForBackend(
  room: Partial<Room>,
): Partial<BackendRoom> {
  // Make sure we don't send an empty name
  if (room.name === "") return {};

  const backendRoom: Partial<BackendRoom> = {
    id: room.id ? Number.parseInt(room.id, 10) : undefined,
    name: room.name, // Changed from room_name to name to match backend API
    is_occupied: room.isOccupied ?? false,
  };

  // Only include optional fields if they have non-empty values
  if (room.building !== undefined && room.building !== "") {
    backendRoom.building = room.building;
  }
  if (room.floor !== undefined) {
    backendRoom.floor = room.floor;
  }
  const capacity = (room as { capacity?: unknown }).capacity;
  if (typeof capacity === "number") {
    backendRoom.capacity = capacity;
  } else if (capacity === null || capacity === "") {
    backendRoom.capacity = null;
  }
  if (room.category !== undefined && room.category !== "") {
    backendRoom.category = room.category;
  }
  if (room.color !== undefined && room.color !== "") {
    backendRoom.color = room.color;
  }
  if (room.deviceId !== undefined && room.deviceId !== "") {
    backendRoom.device_id = room.deviceId;
  }

  return backendRoom;
}

// Request/Response types

// Helper functions
export function formatFloor(floor: number | undefined): string {
  if (floor === undefined) return "Etage nicht angegeben";
  if (floor === 0) return "Erdgeschoss";
  return `Etage ${floor}`;
}

export function formatRoomName(room: Room): string {
  let name = room.name;

  if (room.building) {
    name = `${room.building} - ${name}`;
  }

  return name;
}

export function formatRoomLocation(room: Room): string {
  return formatFloor(room.floor);
}

export function formatRoomCategory(room: Room): string {
  if (!room.category) return "Keine Kategorie";

  const categories: Record<string, string> = {
    standard: "Standard",
    classroom: "Classroom",
    lab: "Laboratory",
    gym: "Gymnasium",
    cafeteria: "Cafeteria",
    office: "Office",
    meeting: "Meeting Room",
    bathroom: "Bathroom",
    storage: "Storage",
    other: "Other",
  };

  return categories[room.category.toLowerCase()] ?? room.category;
}

export function formatRoomCapacity(room: Room): string {
  if (room.capacity === undefined) return "Kapazität nicht angegeben";
  return `${room.studentCount ?? 0}/${room.capacity} students`;
}

export function getRoomUtilization(room: Room): number {
  if (room.capacity === undefined || room.capacity === 0) return 0;
  return ((room.studentCount ?? 0) / room.capacity) * 100;
}

export function getRoomStatusColor(room: Room): string {
  if (!room.isOccupied) return "green";

  const utilization = getRoomUtilization(room);
  if (utilization < 50) return "green";
  if (utilization < 80) return "yellow";
  return "red";
}
