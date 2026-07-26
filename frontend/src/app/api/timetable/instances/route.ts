// app/api/timetable/instances/route.ts
//
// GET  /api/timetable/instances?from=YYYY-MM-DD&to=YYYY-MM-DD
//   Proxies the weekly instance list (WP-F2 backend prerequisite
//   instances_list.go).
// POST /api/timetable/instances
//   Creates a planned (spontaneous or template-bound) instance.
//   Backend handler: instances_create.go. Body shape mirrors
//   CreateInstanceBody in lib/timetable-types.ts.
//
// The Go backend returns { status, data, message }; the proxies strip that
// envelope so route-wrapper does not double-wrap the payload.
import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/instances");

export const POST = proxyPost("/api/timetable/instances");
