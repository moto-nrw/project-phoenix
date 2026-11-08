import { proxyPost } from "~/lib/route-wrapper.server";

export const POST = proxyPost("/api/students/pickup-schedules/bulk");
