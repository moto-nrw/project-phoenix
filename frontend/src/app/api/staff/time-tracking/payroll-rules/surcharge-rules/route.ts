import { proxyPost } from "~/lib/route-proxy.server";

interface SurchargeRuleBody {
  kind: string;
  window_start_minute?: number | null;
  window_end_minute?: number | null;
  rate_type: string;
  rate_hundredths: number;
  wage_type_id: string;
}

// Eine Zuschlagsregel anlegen (#4807).
export const POST = proxyPost<unknown, SurchargeRuleBody>(
  "/api/staff/time-tracking/payroll-rules/surcharge-rules",
);
