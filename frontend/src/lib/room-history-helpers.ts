// lib/room-history-helpers.ts
// Type definitions and helper functions for room history

// Backend types
export interface BackendRoomHistoryEntry {
  id: number;
  room_id: number;
  date: string; // ISO string date
  group_name: string;
  activity_name?: string;
  supervisor_name?: string;
  student_count: number;
  duration: number; // in minutes
}

// Frontend types
export interface RoomHistoryEntry {
  id: string;
  roomId: string;
  date: string;
  groupName: string;
  activityName?: string;
  supervisorName?: string;
  studentCount: number;
  duration: number; // in minutes
}

// Mapping functions
export function mapRoomHistoryEntryResponse(
  backendEntry: BackendRoomHistoryEntry,
): RoomHistoryEntry {
  return {
    id: String(backendEntry.id),
    roomId: String(backendEntry.room_id),
    date: backendEntry.date,
    groupName: backendEntry.group_name,
    activityName: backendEntry.activity_name,
    supervisorName: backendEntry.supervisor_name,
    studentCount: backendEntry.student_count,
    duration: backendEntry.duration,
  };
}

export function mapRoomHistoryEntriesResponse(
  backendEntries: BackendRoomHistoryEntry[],
): RoomHistoryEntry[] {
  return backendEntries.map(mapRoomHistoryEntryResponse);
}

// Utility functions
export function formatDate(dateString: string): string {
  const date = new Date(dateString);
  return date.toLocaleDateString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatDuration(minutes: number): string {
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;

  if (hours === 0) {
    return `${mins} Minuten`;
  } else if (mins === 0) {
    return hours === 1 ? `1 Stunde` : `${hours} Stunden`;
  } else {
    return `${hours} Std. ${mins} Min.`;
  }
}
