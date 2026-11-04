import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/timetable/school-vacations?from=&to=
 * Schulferien des Bundeslands der Schule für „Schuljahr und Kalender“
 * (#5345). moto liest sie täglich aus der OpenHolidays API.
 */
export const GET = proxyGet("/api/timetable/school-vacations");
