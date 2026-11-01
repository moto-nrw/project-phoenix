import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface SurchargeRuleBody {
  kind: string;
  window_start_minute?: number | null;
  window_end_minute?: number | null;
  rate_type: string;
  rate_hundredths: number;
  wage_type_id: string;
}

const path = (p: Record<string, unknown>) =>
  `/api/staff/time-tracking/payroll-rules/surcharge-rules/${requirePathSegmentParam(p, "ruleId")}`;

// Eine Zuschlagsregel ändern (#4807).
export const PUT = proxyPut<unknown, SurchargeRuleBody>(path);

// Eine Zuschlagsregel archivieren.
export const DELETE = proxyDelete(path);
