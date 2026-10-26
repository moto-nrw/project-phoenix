import { proxyGet } from "~/lib/route-proxy.server";

// Proxies GET /api/time-tracking/absence-allowances?year=YYYY → backend:
// the staff member's own Kontingente of the school's absence types (#4971).
export const GET = proxyGet("/api/time-tracking/absence-allowances");
