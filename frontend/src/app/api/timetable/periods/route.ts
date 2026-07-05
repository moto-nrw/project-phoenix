// app/api/timetable/periods/route.ts
//
// GET  /api/timetable/periods       - list all calendar periods for the tenant
// POST /api/timetable/periods       - create a new calendar period
//
// The Go backend (api/timetable/api.go listPeriods/createPeriod) wraps
// responses in { status, data, message }. The proxies strip that envelope so
// route-wrapper does not double-wrap the payload.
import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/periods");

export const POST = proxyPost("/api/timetable/periods");
