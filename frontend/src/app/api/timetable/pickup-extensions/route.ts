// app/api/timetable/pickup-extensions/route.ts
//
// GET /api/timetable/pickup-extensions[?student_id=] — open decisions for
// children who stay longer than before and may be on no block for the extra
// time (#3261). Forwards to the backend and strips the Go envelope.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/pickup-extensions");
