import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/timetable/planning-tracks");
export const POST = proxyPost("/api/timetable/planning-tracks");
