// app/api/timetable/templates/route.ts
//
// GET  /api/timetable/templates?period_id=...
//   Proxies the read-only template list for the planner "Vorlagen" view.
// POST /api/timetable/templates — proxies the smart template-creation
// endpoint (templates_create.go). The backend bundles timeframe +
// activities.groups + activities.schedules into one transaction and can
// optionally materialize the visible week.
//
// The Go backend returns { status, data, message }. The proxies unwrap the Go
// envelope here so route-wrapper's wrapInApiResponse doesn't double-wrap
// the payload — clients (timetable-api.ts unwrap) expect a single
// envelope and read envelope.data as the real result.
import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/templates");

export const POST = proxyPost("/api/timetable/templates");
