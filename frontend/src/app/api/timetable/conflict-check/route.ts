// app/api/timetable/conflict-check/route.ts
//
// GET /api/timetable/conflict-check?date=...&start_time=...&end_time=...
//     [&room_id=N][&staff_ids=1,2][&student_ids=3,4][&exclude_instance_id=N]
//   Advisory pre-save conflict check. Forwards the query string verbatim
//   to the backend /api/timetable/conflicts endpoint.
//
// The Go backend wraps responses in { status, data, message }. Strip that
// envelope here so route-wrapper does not double-wrap the payload.
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const search = request.nextUrl.searchParams.toString();
    const path = `/api/timetable/conflicts${search ? `?${search}` : ""}`;
    const response = await apiGet<{ data: unknown }>(path, token);
    return response.data;
  },
);
