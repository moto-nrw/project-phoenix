import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface WageTypeBody {
  name: string;
  number: string;
  unit: string;
}

const path = (p: Record<string, unknown>) =>
  `/api/staff/time-tracking/payroll-rules/wage-types/${requirePathSegmentParam(p, "wageTypeId")}`;

// Eine eigene Lohnart ändern (#4807).
export const PUT = proxyPut<unknown, WageTypeBody>(path);

// Eine eigene Lohnart archivieren; die Zeile bleibt für abgeschlossene Monate.
export const DELETE = proxyDelete(path);
