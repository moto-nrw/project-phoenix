// app/api/timetable/offering-gaps/route.ts
//
// GET /api/timetable/offering-gaps?period_id=...
//   Offerings whose booked children no Regeltermin takes over (#5140), with
//   the school's decision „Kein eigener Regeltermin nötig“.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/offering-gaps");
