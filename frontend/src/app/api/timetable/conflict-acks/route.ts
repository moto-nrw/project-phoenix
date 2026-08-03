// app/api/timetable/conflict-acks/route.ts
//
// GET /api/timetable/conflict-acks — fingerprints of the planning conflicts
// the current user has acknowledged (hidden) in this school (#2139).
// Forwards to the backend and strips the Go envelope.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/conflict-acks");
