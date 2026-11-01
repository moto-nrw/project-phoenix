import { proxyPost } from "~/lib/route-proxy.server";

interface WageTypeBody {
  name: string;
  number: string;
  unit: string;
}

// Eine eigene Lohnart anlegen (#4807).
export const POST = proxyPost<unknown, WageTypeBody>(
  "/api/staff/time-tracking/payroll-rules/wage-types",
);
