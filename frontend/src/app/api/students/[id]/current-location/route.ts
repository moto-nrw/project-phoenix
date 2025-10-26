// app/api/students/[id]/current-location/route.ts
import type { NextRequest } from "next/server";
import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-client";
import type { BackendStudent } from "~/lib/student-helpers";
import { LOCATION_STATUSES, isHomeLocation, isPresentLocation, normalizeLocation } from "~/lib/location-helper";
import axios from "axios";
import type { BackendRoom } from "~/lib/rooms-helpers";

// Define proper types for the API responses
interface StudentApiResponse {
  data: BackendStudent;
}

interface RoomStatusApiResponse {
  data: {
    student_room_status: Record<string, {
      current_room_id: number;
      check_in_time?: string;
    }>;
  };
}

interface RoomApiResponse {
  data: BackendRoom;
}

type RoomInfo = {
  id: string;
  name: string;
  roomNumber?: string;
  building?: string;
  floor?: number;
};

type GroupInfo = {
  id: string;
  name: string;
  roomId?: string; // Group's assigned room ID
};

type PresentLocation = {
  status: "present";
  location: string; // e.g. "Anwesend", "Unterwegs", or specific room label
  room: RoomInfo | null;
  group: GroupInfo | null;
  checkInTime: string | null;
  isGroupRoom: boolean;
};

type NotPresentLocation = {
  status: "not_present";
  location: "Zuhause";
  room: null;
  group: GroupInfo | null;
  checkInTime: null;
  isGroupRoom: false;
};

type UnknownLocation = {
  status: "unknown";
  location: "Unbekannt";
  room: null;
  group: null;
  checkInTime: null;
  isGroupRoom: false;
  errorCode?: "NETWORK" | "UNAUTHORIZED" | "FORBIDDEN" | "NOT_FOUND" | "SERVER";
};

type LocationResponse = PresentLocation | NotPresentLocation | UnknownLocation;

/**
 * isGroupRoom semantics
 * - true: The student is present and currently located in their group's assigned room (groupRoomId matches current_room_id).
 * - false: The student is present but not in their group's room (e.g., unterwegs or different room), or not present/unknown.
 * Usage guidance:
 * - UI may highlight a green badge when isGroupRoom is true and room != null.
 * - When present but in transit (Unterwegs) or when room details are not accessible due to permissions, expose status "present" with room = null and isGroupRoom = false.
 * - Do not infer presence solely from isGroupRoom; rely on discriminated union status.
 */

/**
 * Handler for GET /api/students/[id]/current-location
 * Returns the current location of a student including room details
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params: Record<string, unknown>) => {
  const studentId = params.id as string;
  
  if (!studentId) {
    throw new Error('Student ID is required');
  }
  
  try {
    // First, get the student details - this includes attendance status from the backend
    const studentResponse = await apiGet<StudentApiResponse>(`/api/students/${studentId}`, token);
    const student = studentResponse.data.data;
    const normalizedLocation = normalizeLocation(student.current_location);

    // Get the student's group room ID for comparison
    let groupRoomId: number | null = null;
    if (student.group_id) {
      try {
        const groupResponse = await apiGet<{ data: { room_id: number | null } }>(`/api/groups/${student.group_id}`, token);
        groupRoomId = groupResponse.data.data.room_id;
      } catch (e) {
        console.error('Error fetching group room ID:', e);
      }
    }

    // The backend already determines the correct location based on attendance data
    if (isHomeLocation(normalizedLocation)) {
      return {
        status: "not_present",
        location: LOCATION_STATUSES.HOME,
        room: null,
        group: student.group_id ? {
          id: student.group_id.toString(),
          name: student.group_name ?? "Unknown Group",
          roomId: groupRoomId?.toString()
        } : null,
        checkInTime: null,
        isGroupRoom: false
      } satisfies LocationResponse;
    }

    // If student is marked as present (Anwesend...), they are onsite
    if (isPresentLocation(normalizedLocation)) {
      // Student is checked in - try to get detailed room information if they have a group
      if (student?.group_id) {
        // Try to get room status for the student's group (may fail due to permissions)
        try {
          const roomStatusResponse = await apiGet<RoomStatusApiResponse>(`/api/groups/${student.group_id}/students/room-status`, token);
          const roomStatusData = roomStatusResponse.data.data;
          
          if (roomStatusData?.student_room_status) {
            const studentStatus = roomStatusData.student_room_status[studentId];
            
            // Check if student has any current room (not just their group's room)
            if (studentStatus?.current_room_id) {
              const isInGroupRoom = groupRoomId === studentStatus.current_room_id;

              // Get room details
              try {
                const roomResponse = await apiGet<RoomApiResponse>(`/api/rooms/${studentStatus.current_room_id}`, token);
                const roomData = roomResponse.data.data;

                if (roomData) {
                  return {
                    status: "present",
                    location: normalizedLocation,
                    room: {
                      id: roomData.id.toString(),
                      name: roomData.name ?? `Raum ${studentStatus.current_room_id}`,
                      roomNumber: undefined,
                      building: roomData.building,
                      floor: roomData.floor
                    },
                    group: {
                      id: student.group_id.toString(),
                      name: student.group_name ?? "Unknown Group",
                      roomId: groupRoomId?.toString()
                    },
                    checkInTime: studentStatus.check_in_time ?? null,
                    isGroupRoom: isInGroupRoom
                  } satisfies LocationResponse;
                }
              } catch (e) {
                console.error('Error fetching room details:', e);
                // Even if room details fail, we know they're in a room
                return {
                  status: "present",
                  location: normalizedLocation,
                  room: {
                    id: studentStatus.current_room_id.toString(),
                    name: `Raum ${studentStatus.current_room_id}`,
                    roomNumber: undefined,
                    building: undefined,
                    floor: undefined
                  },
                  group: {
                    id: student.group_id.toString(),
                    name: student.group_name ?? "Unknown Group",
                    roomId: groupRoomId?.toString()
                  },
                  checkInTime: studentStatus.check_in_time ?? null,
                  isGroupRoom: isInGroupRoom
                } satisfies LocationResponse;
              }
            }
          }
        } catch (error) {
          console.error('Error fetching room status (likely permissions):', error);
          // If we can't get room details due to permissions, that's OK
          // We still know they're checked in, so show them as "Unterwegs"
        }
      }
      
      // If we get here, student is checked in but we couldn't get room details
      // This means they are in transit (not assigned to a specific room yet)
      // Return "Unterwegs" to match the behavior in ogs_groups page
      return {
        status: "present",
        location: LOCATION_STATUSES.TRANSIT,
        room: null,
        group: student.group_id ? {
          id: student.group_id.toString(),
          name: student.group_name ?? "Unknown Group",
          roomId: groupRoomId?.toString()
        } : null,
        checkInTime: null,
        isGroupRoom: false
      } satisfies LocationResponse;
    }

    // Default fallback: student is considered not present
    return {
      status: "not_present",
      location: LOCATION_STATUSES.HOME,
      room: null,
      group: student.group_id ? {
        id: student.group_id.toString(),
        name: student.group_name ?? "Unknown Group",
        roomId: groupRoomId?.toString()
      } : null,
      checkInTime: null,
      isGroupRoom: false
    } satisfies LocationResponse;

  } catch (error) {
    console.error("Error fetching student current location:", error);
    // Return unknown status if there's an error with a structured error code
    let errorCode: UnknownLocation["errorCode"];
    if (axios.isAxiosError(error)) {
      if (error.response) {
        const status = error.response.status;
        if (status === 401) errorCode = "UNAUTHORIZED";
        else if (status === 403) errorCode = "FORBIDDEN";
        else if (status === 404) errorCode = "NOT_FOUND";
        else errorCode = "SERVER";
      } else {
        errorCode = "NETWORK";
      }
    }
    return {
      status: "unknown",
      location: LOCATION_STATUSES.UNKNOWN,
      room: null,
      group: null,
      checkInTime: null,
      isGroupRoom: false,
      errorCode,
    } satisfies LocationResponse;
  }
});
