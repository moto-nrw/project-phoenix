// app/api/rooms/[id]/history/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
// No need for the ApiResponse type import anymore since we're using NextResponse directly

// Backend interface for room history entries
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

/**
 * Custom handler for GET /api/rooms/[id]/history
 * Returns history of a specific room's usage
 */
export async function GET(
  request: NextRequest,
  context: { params: { id: string } }
): Promise<NextResponse> {
  try {
    const session = await auth();

    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Unauthorized" },
        { status: 401 }
      );
    }

    // Extract roomId from context.params
    // This is the correct way to access dynamic route parameters in Next.js App Router
    const roomId = context.params.id;

    if (!roomId) {
      return NextResponse.json(
        { error: "Invalid id parameter" },
        { status: 400 }
      );
    }

    // Extract date range query parameters if provided
    const start_date = request.nextUrl.searchParams.get('start_date');
    const end_date = request.nextUrl.searchParams.get('end_date');
  
    // Build query parameters for the API call
    let endpoint = `/api/rooms/${roomId}/history`;
    const queryParams = new URLSearchParams();
    
    if (start_date) {
      queryParams.append('start_date', start_date);
    }
    
    if (end_date) {
      queryParams.append('end_date', end_date);
    }
    
    if (queryParams.toString()) {
      endpoint += `?${queryParams.toString()}`;
    }
    
    // Fetch data from the API
    try {
      // Get data from the real API
      const data = await apiGet<BackendRoomHistoryEntry[]>(endpoint, session.user.token);
      
      // Return the real data if successful
      return NextResponse.json({
        status: "success",
        data: data
      });
    } catch (apiError) {
      console.error(`Error fetching room history for room ${roomId}:`, apiError);
      
      // Check if it's a 404 error (resource not found)
      if (apiError instanceof Error && apiError.message.includes("404")) {
        return NextResponse.json({
          status: "success", 
          data: [] // Return empty array when no history data exists
        });
      }
      
      // For other API errors, return an error response
      return NextResponse.json(
        { error: `Backend API error: ${apiError instanceof Error ? apiError.message : String(apiError)}` },
        { status: 500 }
      );
    }
  } catch (error) {
    // Handle any unexpected errors in the overall request processing
    console.error("Error in room history endpoint:", error);
    return NextResponse.json(
      { error: `Failed to fetch room history: ${error instanceof Error ? error.message : String(error)}` },
      { status: 500 }
    );
  }
}