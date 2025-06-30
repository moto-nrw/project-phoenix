// app/api/students/[id]/current-location/route.ts
import type { NextRequest } from "next/server";
import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-client";
import type { BackendStudent } from "~/lib/student-helpers";
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

interface LocationResponse {
  status: "present" | "not_present" | "unknown";
  location: string;
  room: {
    id: string;
    name: string;
    roomNumber?: string;
    building?: string;
    floor?: number;
  } | null;
  group: {
    id: string;
    name: string;
  } | null;
  checkInTime: string | null;
}

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
    // First, get the student details to find their group
    const studentResponse = await apiGet<StudentApiResponse>(`/api/students/${studentId}`, token);
    const student = studentResponse.data.data;
    
    if (!student?.group_id) {
      return {
        status: "not_present",
        location: "Zuhause",
        room: null,
        group: null,
        checkInTime: null
      } satisfies LocationResponse;
    }
    
    // Get the room status for the student's group
    try {
      const roomStatusResponse = await apiGet<RoomStatusApiResponse>(`/api/groups/${student.group_id}/students/room-status`, token);
      const roomStatusData = roomStatusResponse.data.data;
      
      if (roomStatusData?.student_room_status) {
        const studentStatus = roomStatusData.student_room_status[studentId];
        
        // Check if student has any current room (not just their group's room)
        if (studentStatus?.current_room_id) {
          // Get room details
          try {
            const roomResponse = await apiGet<RoomApiResponse>(`/api/rooms/${studentStatus.current_room_id}`, token);
            const roomData = roomResponse.data.data;
            
            if (roomData) {
              return {
                status: "present",
                location: "Anwesend",
                room: {
                  id: roomData.id.toString(),
                  name: roomData.name ?? `Raum ${studentStatus.current_room_id}`,
                  roomNumber: undefined,
                  building: roomData.building,
                  floor: roomData.floor
                },
                group: {
                  id: student.group_id.toString(),
                  name: student.group_name ?? "Unknown Group"
                },
                checkInTime: studentStatus.check_in_time ?? null
              } satisfies LocationResponse;
            }
          } catch (e) {
            console.error('Error fetching room details:', e);
            // Even if room details fail, we know they're in a room
            return {
              status: "present",
              location: "Anwesend",
              room: {
                id: studentStatus.current_room_id.toString(),
                name: `Raum ${studentStatus.current_room_id}`,
                roomNumber: undefined,
                building: undefined,
                floor: undefined
              },
              group: {
                id: student.group_id.toString(),
                name: student.group_name ?? "Unknown Group"
              },
              checkInTime: studentStatus.check_in_time ?? null
            } satisfies LocationResponse;
          }
        }
      }
    } catch (error) {
      console.error('Error fetching room status:', error);
    }
    
    // If we get here, student is not in any room
    return {
      status: "not_present",
      location: "Zuhause",
      room: null,
      group: null,
      checkInTime: null
    } satisfies LocationResponse;
    
  } catch (error) {
    console.error("Error fetching student current location:", error);
    // Return unknown status if there's an error
    return {
      status: "unknown",
      location: "Unbekannt",
      room: null,
      group: null,
      checkInTime: null
    } satisfies LocationResponse;
  }
});