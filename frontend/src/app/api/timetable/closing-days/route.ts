// app/api/timetable/closing-days/route.ts
//
// GET  /api/timetable/closing-days  - list all closing day ranges for the tenant
// POST /api/timetable/closing-days  - create a new closing day range
//
// The Go backend (api/timetable/closing_days.go) wraps responses in
// { status, data, message }. The proxies strip that envelope so
// route-wrapper does not double-wrap the payload.
import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/closing-days");

export const POST = proxyPost("/api/timetable/closing-days");
