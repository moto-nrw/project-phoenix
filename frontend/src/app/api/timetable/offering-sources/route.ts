// app/api/timetable/offering-sources/route.ts
//
// GET /api/timetable/offering-sources?calendar_period_id=...
//   Proxies the offering-source editor support endpoint (#2137): the
//   Betreuungsangebote selectable as Regeltermin roster source, with
//   per-Jahrgang counts and the templates already sourcing each offering.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/offering-sources");
