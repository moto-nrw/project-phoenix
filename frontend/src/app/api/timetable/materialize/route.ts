// POST /api/timetable/materialize — manually triggers materialization for
// a date window. With an empty body, the backend defaults to next Mon-Sun.
// The planner UI passes the currently visible week's from/to so admins can
// re-run materialization for that exact window.
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface MaterializeBody {
  from_date?: string;
  to_date?: string;
}

// The Go backend wraps responses in { status, data, message } — strip
// the envelope so route-wrapper does not double-wrap the payload.
export const POST = createPostHandler<unknown, MaterializeBody>(
  async (_request: NextRequest, body: MaterializeBody, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/timetable/materialize",
      token,
      body ?? {},
    );
    return response.data;
  },
);
