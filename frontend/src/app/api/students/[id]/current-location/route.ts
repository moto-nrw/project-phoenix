// app/api/students/[id]/current-location/route.ts
import type { NextRequest } from "next/server";
import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-client";

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
    // Use apiGet directly with the token instead of studentService which doesn't have auth in server context
    const studentResponse = await apiGet(`/api/students/${studentId}`, token);
    
    console.log('Student response data:', JSON.stringify(studentResponse.data, null, 2));
    
    // Extract the actual student data from the response
    const student = studentResponse.data?.data || studentResponse.data;
    
    console.log('Extracted student data:', JSON.stringify(student, null, 2));
    
    if (!student || !student.group_id) {
      return {
        status: "not_present",
        location: "Zuhause",
        room: null,
        group: null,
        checkInTime: null
      };
    }
    
    // Get the room status for the student's group
    try {
      const roomStatusResponse = await apiGet(`/api/groups/${student.group_id}/students/room-status`, token);
      
      // Extract the actual data (handle potential double-wrapping)
      const roomStatusData = roomStatusResponse.data?.data || roomStatusResponse.data;
      
      console.log('Room status API response:', JSON.stringify(roomStatusData, null, 2));
      console.log('Looking for student ID:', studentId);
      console.log('Has student_room_status?', !!roomStatusData?.student_room_status);
      console.log('Response data type:', typeof roomStatusData);
      
      if (roomStatusData?.student_room_status) {
        const studentStatus = roomStatusData.student_room_status[studentId];
        
        console.log('Student status:', JSON.stringify(studentStatus, null, 2));
        
        // Check if student has any current room (not just their group's room)
        if (studentStatus && studentStatus.current_room_id) {
          // Get room details
          try {
            const roomResponse = await apiGet(`/api/rooms/${studentStatus.current_room_id}`, token);
            if (roomResponse.data) {
              return {
                status: "present",
                location: "Anwesend",
                room: {
                  id: roomResponse.data.id.toString(),
                  name: roomResponse.data.name,
                  roomNumber: roomResponse.data.number,
                  building: roomResponse.data.building,
                  floor: roomResponse.data.floor
                },
                group: {
                  id: student.group_id.toString(),
                  name: student.group_name || "Unknown Group"
                },
                checkInTime: null
              };
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
                roomNumber: null,
                building: null,
                floor: null
              },
              group: {
                id: student.group_id.toString(),
                name: student.group_name || "Unknown Group"
              },
              checkInTime: null
            };
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
    };
    
  } catch (error) {
    console.error("Error fetching student current location:", error);
    // Return unknown status if there's an error
    return {
      status: "unknown",
      location: "Unbekannt",
      room: null,
      group: null,
      checkInTime: null
    };
  }
});