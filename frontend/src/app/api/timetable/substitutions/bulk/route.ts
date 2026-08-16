// POST /api/timetable/substitutions/bulk — applies one person's day-wide
// absence (optionally covered by one substitute) to several selected days in
// a single atomic backend transaction (Sammel-Vertretung, #2284).
import { proxyPost } from "~/lib/route-proxy.server";

export const POST = proxyPost(() => "/api/timetable/substitutions/bulk");
