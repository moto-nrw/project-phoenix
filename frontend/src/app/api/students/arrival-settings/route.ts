import { proxyGet } from "@/lib/route-proxy.server";

// GET /api/students/arrival-settings - source of the tenant's care days
export const GET = proxyGet(() => "/api/students/arrival-settings");
