// lib/room-helpers.ts
// Type definitions and helper functions for rooms

// Backend types (from Go structs)
export interface BackendRoom {
    id: number;
    name: string;     // Changed to match backend API which uses "name"
    building?: string;
    floor: number;
    capacity: number;
    category: string;
    color: string;
    device_id?: string;
    is_occupied: boolean;
    activity_name?: string;
    group_name?: string;
    supervisor_name?: string;
    student_count?: number;
    created_at: string;
    updated_at: string;
}

// Frontend types
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

// Mapping functions
export function mapRoomResponse(backendRoom: BackendRoom): Room {
    return {
        id: String(backendRoom.id),
        name: backendRoom.name, // Changed from room_name to name to match backend API
        building: backendRoom.building,
        floor: backendRoom.floor,
        capacity: backendRoom.capacity,
        category: backendRoom.category,
        color: backendRoom.color,
        deviceId: backendRoom.device_id,
        isOccupied: backendRoom.is_occupied,
        activityName: backendRoom.activity_name,
        groupName: backendRoom.group_name,
        supervisorName: backendRoom.supervisor_name,
        studentCount: backendRoom.student_count,
        createdAt: backendRoom.created_at,
        updatedAt: backendRoom.updated_at,
    };
}

export function mapRoomsResponse(backendRooms: BackendRoom[] | null | any): Room[] {
    // Handle nested API response structure (from server API)
    if (backendRooms && typeof backendRooms === 'object' && 'data' in backendRooms && Array.isArray(backendRooms.data)) {
        console.log("Handling nested API response for rooms");
        return backendRooms.data.map(mapRoomResponse);
    }
    
    // Handle null, undefined or non-array responses
    if (!backendRooms || !Array.isArray(backendRooms)) {
        console.warn("Received invalid response format for rooms:", backendRooms);
        return [];
    }
    
    // Standard array response
    return backendRooms.map(mapRoomResponse);
}

export function mapSingleRoomResponse(response: { data: BackendRoom }): Room {
    return mapRoomResponse(response.data);
}

// Prepare frontend room for backend
export function prepareRoomForBackend(room: Partial<Room>): Partial<BackendRoom> {
    // Make sure we don't send an empty name
    if (room.name === "") return {};
    
    return {
        id: room.id ? parseInt(room.id, 10) : undefined,
        name: room.name, // Changed from room_name to name to match backend API
        building: room.building,
        floor: room.floor ?? 0,
        capacity: room.capacity ?? 0,
        category: room.category ?? 'standard',
        color: room.color ?? '#FFFFFF',
        device_id: room.deviceId,
        is_occupied: room.isOccupied ?? false,
    };
}

// Request/Response types
export interface CreateRoomRequest {
    name: string;
    building?: string;
    floor: number;
    capacity: number;
    category: string;
    color: string;
    device_id?: string;
}

export interface UpdateRoomRequest {
    name: string;
    building?: string;
    floor: number;
    capacity: number;
    category: string;
    color: string;
    device_id?: string;
}

// Helper functions
export function formatRoomName(room: Room): string {
    let name = room.name;
    
    if (room.building) {
        name = `${room.building} - ${name}`;
    }
    
    return name;
}

export function formatRoomLocation(room: Room): string {
    return `Floor ${room.floor}`;
}

export function formatRoomCategory(room: Room): string {
    const categories: Record<string, string> = {
        'standard': 'Standard',
        'classroom': 'Classroom',
        'lab': 'Laboratory',
        'gym': 'Gymnasium',
        'cafeteria': 'Cafeteria',
        'office': 'Office',
        'meeting': 'Meeting Room',
        'bathroom': 'Bathroom',
        'storage': 'Storage',
        'other': 'Other',
    };
    
    return categories[room.category.toLowerCase()] ?? room.category;
}

export function formatRoomCapacity(room: Room): string {
    return `${room.studentCount ?? 0}/${room.capacity} students`;
}

export function getRoomUtilization(room: Room): number {
    if (!room.capacity || room.capacity === 0) return 0;
    return ((room.studentCount ?? 0) / room.capacity) * 100;
}

export function getRoomStatusColor(room: Room): string {
    if (!room.isOccupied) return 'green';
    
    const utilization = getRoomUtilization(room);
    if (utilization < 50) return 'green';
    if (utilization < 80) return 'yellow';
    return 'red';
}