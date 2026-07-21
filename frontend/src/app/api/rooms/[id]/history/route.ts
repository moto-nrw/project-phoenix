// app/api/rooms/[id]/history/route.ts
import type { NextRequest } from "next/server";
import { apiGet, ApiResponseError } from "~/lib/api-helpers.server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createLogger } from "~/lib/logger";
import { ROOM_HISTORY_STATUS_FEATURE_DISABLED } from "~/lib/room-helpers";

const logger = createLogger({ component: "RoomHistoryRoute" });
const dateOnlyPattern = /^\d{4}-\d{2}-\d{2}$/;

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

// True iff the backend rejected the request because the tenant has
// gdpr.attendance_log_enabled = false (issue #1425). The backend signals
// that with HTTP 403 + a JSON body of `{ "error": "feature_disabled" }`.
// A generic 403 (e.g. RBAC failure) intentionally does NOT match here so
// it surfaces as a real error to the user — only the GDPR feature-gate
// path is translated to the "section hidden" signal.
function isFeatureDisabledError(error: unknown): boolean {
  if (!(error instanceof ApiResponseError)) return false;
  if (error.status !== 403) return false;
  const body = error.body<{ error?: string }>();
  return body?.error === ROOM_HISTORY_STATUS_FEATURE_DISABLED;
}

function normalizeRangeParam(value: string, boundary: "start" | "end"): string {
  if (!dateOnlyPattern.test(value)) return value;
  return boundary === "start" ? `${value}T00:00:00Z` : `${value}T23:59:59Z`;
}

/**
 * Custom handler for GET /api/rooms/[id]/history
 * Returns history of a specific room's usage
 */
async function GETHandler(request: NextRequest): Promise<NextResponse> {
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

  if (start) queryParams.append("start", normalizeRangeParam(start, "start"));
  if (end) queryParams.append("end", normalizeRangeParam(end, "end"));

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
    if (apiError instanceof ApiResponseError && apiError.status === 404) {
      return NextResponse.json({ status: "success", data: [] });
    }

    // gdpr.attendance_log_enabled is off for this tenant — translate the
    // backend's 403 into an explicit, non-error signal so the drawer can
    // hide the section deliberately. Returning 200 here is correct: from
    // the client's perspective the request succeeded and the answer is
    // "the feature is off", not "the request failed".
    if (isFeatureDisabledError(apiError)) {
      return NextResponse.json({
        status: ROOM_HISTORY_STATUS_FEATURE_DISABLED,
        data: [],
      });
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

export const GET = withTenantAuth(GETHandler);
