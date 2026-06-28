// POST /api/timetable/materialize — manually triggers materialization for
// a date window. With an empty body, the backend defaults to next Mon-Sun.
// The planner UI passes the currently visible week's from/to so admins can
// re-run materialization for that exact window.
import { proxyPost } from "~/lib/route-proxy.server";

interface MaterializeBody {
  from_date?: string;
  to_date?: string;
}

// The Go backend wraps responses in { status, data, message } — proxyPost
// strips the envelope so route-wrapper does not double-wrap the payload.
export const POST = proxyPost<unknown, MaterializeBody>(
  "/api/timetable/materialize",
);
