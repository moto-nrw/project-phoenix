import { proxyGet } from "~/lib/route-wrapper.server";

// Proxies GET /api/time-tracking/vacation/quota?year=YYYY → backend
export const GET = proxyGet("/api/time-tracking/vacation/quota");
