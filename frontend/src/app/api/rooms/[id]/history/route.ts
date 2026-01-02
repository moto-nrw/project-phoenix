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
export async function GET(request: NextRequest): Promise<NextResponse> {
  const session = await auth();

  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  // Extract roomId from URL path
  const pathParts = request.nextUrl.pathname.split("/");
  const roomsIndex = pathParts.indexOf("rooms");
  const roomId = roomsIndex >= 0 ? pathParts[roomsIndex + 1] : undefined;

  if (!roomId) {
    return NextResponse.json(
      { error: "Invalid id parameter" },
      { status: 400 },
    );
  }

  // Build query parameters for the API call
  const queryParams = new URLSearchParams();
  const start_date = request.nextUrl.searchParams.get("start_date");
  const end_date = request.nextUrl.searchParams.get("end_date");

  if (start_date) queryParams.append("start_date", start_date);
  if (end_date) queryParams.append("end_date", end_date);

  const queryString = queryParams.toString();
  const querySuffix = queryString ? "?" + queryString : "";
  const endpoint = `/api/rooms/${roomId}/history${querySuffix}`;

  try {
    const data = await apiGet<BackendRoomHistoryEntry[]>(
      endpoint,
      session.user.token,
    );
    return NextResponse.json({ status: "success", data });
  } catch (apiError) {
    // 404 means no history exists - return empty array
    if (apiError instanceof Error && apiError.message.includes("404")) {
      return NextResponse.json({ status: "success", data: [] });
    }

    console.error(`Error fetching room history for room ${roomId}:`, apiError);
    const errorMessage =
      apiError instanceof Error ? apiError.message : String(apiError);
    return NextResponse.json(
      { error: `Backend API error: ${errorMessage}` },
      { status: 500 },
    );
  }
}
