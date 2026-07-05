import { proxyPost } from "~/lib/route-proxy.server";

export const POST = proxyPost("/api/timetable/instances/re-plan-week");
