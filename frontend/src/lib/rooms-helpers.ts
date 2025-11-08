// Room type definitions and helper functions

export interface Room {
  id: string;
  name: string;
  building?: string;
  floor: number;
  capacity: number;
  category: string;
  color: string;
  created_at?: string;
  updated_at?: string;
}

export interface BackendRoom {
  id: number;
  name: string;
  building?: string;
  floor: number;
  capacity: number;
  category: string;
  color: string;
  created_at?: string;
  updated_at?: string;
}

export function mapRoomResponse(data: BackendRoom): Room {
  return {
    id: data.id.toString(),
    name: data.name,
    building: data.building,
    floor: data.floor,
    capacity: data.capacity,
    category: data.category,
    color: data.color,
    created_at: data.created_at,
    updated_at: data.updated_at,
  };
}

export function mapRoomsResponse(data: BackendRoom[]): Room[] {
  return data.map(mapRoomResponse);
}

// Create a map for quick room name lookup by ID
export function createRoomIdToNameMap(rooms: Room[]): Record<string, string> {
  return rooms.reduce(
    (acc, room) => {
      acc[room.id] = room.name;
      return acc;
    },
    {} as Record<string, string>,
  );
}
