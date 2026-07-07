import { proxyPost } from "~/lib/route-proxy.server";

export const POST = proxyPost("/api/timetable/operations/spontaneous/start");
