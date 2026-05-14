import { createProxyGetDataHandler } from "~/lib/route-wrapper";

// Proxies GET /api/time-tracking/vacation/quota?year=YYYY → backend
export const GET = createProxyGetDataHandler(
  "/api/time-tracking/vacation/quota",
);
