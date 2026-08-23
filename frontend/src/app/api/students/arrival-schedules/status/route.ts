import { proxyPost } from "@/lib/route-proxy.server";

// POST /api/students/arrival-schedules/status - Batch lookup for own arrival schedules
export const POST = proxyPost("/api/students/arrival-schedules/status");
