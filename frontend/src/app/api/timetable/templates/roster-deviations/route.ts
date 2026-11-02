// app/api/timetable/templates/roster-deviations/route.ts
//
// GET /api/timetable/templates/roster-deviations?period_id=...[&template_id=...]
//   Regeltermine whose hand-kept children list differs from the bookings of
//   a care offering (#5103), with the offering and filter that match best.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/templates/roster-deviations");
