import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/operations/planned-now");
