import { proxyGet } from "~/lib/route-wrapper.server";

export const GET = proxyGet("/api/timetable/operations/planned-now");
