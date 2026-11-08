import { proxyPost } from "~/lib/route-wrapper.server";

// POST /api/students/arrival-schedules/bulk - Bulk upsert arrival schedules for a school class
export const POST = proxyPost("/api/students/arrival-schedules/bulk");
