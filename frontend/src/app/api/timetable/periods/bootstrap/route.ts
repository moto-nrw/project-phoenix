// app/api/timetable/periods/bootstrap/route.ts
//
// POST /api/timetable/periods/bootstrap
//   Idempotent default-period setup. The backend creates a default
//   school-year period when the tenant has none and returns
//   { periods: [...], created: boolean }.
//
// The Go backend wraps responses in { status, data, message }. proxyPost
// strips that envelope so route-wrapper does not double-wrap the payload.
import { proxyPost } from "~/lib/route-proxy.server";

export const POST = proxyPost("/api/timetable/periods/bootstrap");
