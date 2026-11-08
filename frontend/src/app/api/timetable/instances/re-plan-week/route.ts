import { proxyPost } from "~/lib/route-wrapper.server";

export const POST = proxyPost("/api/timetable/instances/re-plan-week");
