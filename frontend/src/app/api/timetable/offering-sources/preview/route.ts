// app/api/timetable/offering-sources/preview/route.ts
//
// GET /api/timetable/offering-sources/preview?period_id=…&offering_ids=…&roster=1:5,6
//   Proxies the per-weekday preview of an unsaved Regeltermin (#5104): who
//   joins and who leaves when it takes its children from the offerings.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/offering-sources/preview");
