// app/api/timetable/offering-sources/combined-counts/route.ts
//
// GET /api/timetable/offering-sources/combined-counts?ids=1,2,3&calendar_period_id=...
//   Proxies the deduplicated child counts across a selection of offerings
//   (Mehrfach-Quelle, follow-up to #2137): a child enrolled in two selected
//   Angebote counts once, so the Regeltermin editor previews the exact
//   roster size the union would seed.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/offering-sources/combined-counts");
