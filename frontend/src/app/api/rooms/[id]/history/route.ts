// app/api/rooms/[id]/history/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RoomHistoryRoute" });

// Backend contract for room history entries — one aggregated session per row.
// Mirrors services/facilities/interface.go::RoomSessionEntry. No per-student
// IDs or names by design (issue #1425). Per-child detail lives behind
// /students/{id}/attendance-history.
export interface BackendRoomHistoryEntry {
  session_id: number;
  started_at: string; // RFC3339
  ended_at?: string | null; // RFC3339, null while session is open
  duration_minutes?: number | null; // null while session is open
  activity_name: string;
  supervisor_name: string;
  student_count: number;
}

// Shape the Go server emits via common.Respond — { status, data, message }.
// apiGet returns the raw body, so we pass it through verbatim instead of
// wrapping it a second time (which would nest `data` inside another `data`
// and silently empty out the drawer).
interface BackendResponseEnvelope {
  status: string;
  data: BackendRoomHistoryEntry[] | null;
  message?: string;
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

  // Backend expects RFC3339 start/end (not start_date/end_date — that was a
  // long-standing mismatch fixed alongside issue #1425). Accept both query
  // param names from the client for a release of overlap; forward only the
  // backend-canonical names.
  const queryParams = new URLSearchParams();
  const start =
    request.nextUrl.searchParams.get("start") ??
    request.nextUrl.searchParams.get("start_date");
  const end =
    request.nextUrl.searchParams.get("end") ??
    request.nextUrl.searchParams.get("end_date");

  if (start) queryParams.append("start", start);
  if (end) queryParams.append("end", end);

  const queryString = queryParams.toString();
  const querySuffix = queryString ? "?" + queryString : "";
  const endpoint = `/api/rooms/${roomId}/history${querySuffix}`;

  try {
    const backendResponse = await apiGet<BackendResponseEnvelope>(
      endpoint,
      session.user.token,
    );
    return NextResponse.json({
      status: "success",
      data: backendResponse?.data ?? [],
    });
  } catch (apiError) {
    // 404 means no history exists - return empty array
    if (apiError instanceof Error && apiError.message.includes("404")) {
      return NextResponse.json({ status: "success", data: [] });
    }

    logger.error("room history fetch failed", {
      room_id: roomId,
      error: apiError instanceof Error ? apiError.message : String(apiError),
    });
    const errorMessage =
      apiError instanceof Error ? apiError.message : String(apiError);
    return NextResponse.json(
      { error: `Backend API error: ${errorMessage}` },
      { status: 500 },
    );
  }
}
