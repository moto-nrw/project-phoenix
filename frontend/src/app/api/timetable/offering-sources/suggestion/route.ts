// app/api/timetable/offering-sources/suggestion/route.ts
//
// GET /api/timetable/offering-sources/suggestion?period_id=…&roster=1:5,6&…
//   Proxies the offering suggestion of the Regeltermin editor (#5104): the
//   Betreuungsangebot whose bookings match the children an unsaved
//   Regeltermin plans per weekday.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/offering-sources/suggestion");
