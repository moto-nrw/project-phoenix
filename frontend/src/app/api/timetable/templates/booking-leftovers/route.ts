// app/api/timetable/templates/booking-leftovers/route.ts
//
// GET /api/timetable/templates/booking-leftovers?period_id=...
//   Kinder, die nach einer Buchungsänderung noch von Hand in einem
//   Regeltermin stehen, ohne passende Buchung an diesen Tagen (#5102).
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/templates/booking-leftovers");
