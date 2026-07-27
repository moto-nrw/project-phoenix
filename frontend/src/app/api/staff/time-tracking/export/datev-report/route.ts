import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet(
  () => "/api/staff/time-tracking/export/datev-report",
);
