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
    // First, get the student details - this includes attendance status from the backend
    const studentResponse = await apiGet<StudentApiResponse>(`/api/students/${studentId}`, token);
    const student = studentResponse.data.data;
    
    // The backend already determines the correct location based on attendance data
    // If student.location is "Abwesend", they're not checked in today - regardless of group assignment
    if (student.location === "Abwesend") {
      return {
        status: "not_present",
        location: "Zuhause",
        room: null,
        group: student.group_id ? {
          id: student.group_id.toString(),
          name: student.group_name ?? "Unknown Group"
        } : null,
        checkInTime: null
      } satisfies LocationResponse;
    }
    
    // If student.location starts with "Anwesend" (checked in), they are present
    // This includes "Anwesend", "Anwesend - Aktivität", "Anwesend - Room Name", etc.
    if (student.location?.startsWith("Anwesend")) {
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
              // Get room details
              try {
                const roomResponse = await apiGet<RoomApiResponse>(`/api/rooms/${studentStatus.current_room_id}`, token);
                const roomData = roomResponse.data.data;
                
                if (roomData) {
                  return {
                    status: "present",
                    location: student.location || "Anwesend",
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
                  location: student.location || "Anwesend",
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
        location: "Unterwegs",
        room: null,
        group: student.group_id ? {
          id: student.group_id.toString(),
          name: student.group_name ?? "Unknown Group"
        } : null,
        checkInTime: null
      } satisfies LocationResponse;
    }
    
    // If we get here, student.location is neither "Abwesend" nor "Anwesend"
    // This shouldn't happen with proper backend logic, but handle as fallback
    return {
      status: "not_present",
      location: "Zuhause", // Default to home for unknown status
      room: null,
      group: student.group_id ? {
        id: student.group_id.toString(),
        name: student.group_name ?? "Unknown Group"
      } : null,
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