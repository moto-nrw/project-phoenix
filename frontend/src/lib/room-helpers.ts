// Room API helper functions

// Backend room model
export interface BackendRoom {
  id: number;
  room_name: string;
  building?: string;
  floor: number;
  capacity: number;
  category: string;
  color: string;
  device_id?: string;
  created_at?: string;
  modified_at?: string;
  occupancy?: {
    is_occupied: boolean;
    activity_name?: string;
    group_name?: string;
    supervisor_name?: string;
    student_count?: number;
  };
}

// Frontend room model
export interface Room {
  id: string;
  name: string; 
  building?: string;
  floor: number;
  capacity: number;
  category: string;
  color: string;
  deviceId?: string;
  isOccupied: boolean;
  activityName?: string;
  groupName?: string;
  supervisorName?: string;
  studentCount?: number;
  createdAt?: string;
  updatedAt?: string;
}

// Map a single backend room to our frontend model
export function mapSingleRoomResponse(room: BackendRoom): Room {
  return {
    id: room.id.toString(),
    name: room.room_name,
    building: room.building,
    floor: room.floor,
    capacity: room.capacity,
    category: room.category,
    color: room.color,
    deviceId: room.device_id,
    isOccupied: room.occupancy?.is_occupied ?? false,
    activityName: room.occupancy?.activity_name,
    groupName: room.occupancy?.group_name,
    supervisorName: room.occupancy?.supervisor_name,
    studentCount: room.occupancy?.student_count,
    createdAt: room.created_at,
    updatedAt: room.modified_at,
  };
}

// Map multiple backend rooms to our frontend model
export function mapRoomResponse(rooms: BackendRoom[]): Room[] {
  return rooms.map(mapSingleRoomResponse);
}

// Prepare frontend room for backend
export function prepareRoomForBackend(room: Partial<Room>): Partial<BackendRoom> {
  const backendRoom: Partial<BackendRoom> = {
    room_name: room.name,
    building: room.building,
    floor: room.floor,
    capacity: room.capacity,
    category: room.category,
    color: room.color,
    device_id: room.deviceId,
  };

  // Remove undefined fields
  return Object.fromEntries(
    Object.entries(backendRoom).filter(([_, v]) => v !== undefined)
  );
}