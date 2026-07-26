import { proxyPost } from "@/lib/route-proxy.server";

// POST /api/active/tracking-indicators - Bulk check if students visited configured rooms/activities today
export const POST = proxyPost("/api/active/tracking-indicators");
