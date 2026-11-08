import { proxyPost } from "~/lib/route-wrapper.server";

export const POST = proxyPost("/api/timetable/operations/spontaneous/start");
