import { proxyGet } from "~/lib/route-proxy.server";

// Der neueste automatische Versand an das Lohnbüro (#4805) für /payroll.
export const GET = proxyGet(
  () => "/api/staff/time-tracking/payroll-dispatch",
);
