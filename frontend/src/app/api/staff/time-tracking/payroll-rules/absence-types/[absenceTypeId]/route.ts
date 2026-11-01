import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface AbsenceTypeWageTypeBody {
  wage_type_id: string | null;
}

// Eine Abwesenheitsart der Schule einer Lohnart zuordnen (#4807); null
// bucht sie wieder in ihrer bisherigen Kategorie.
export const PUT = proxyPut<unknown, AbsenceTypeWageTypeBody>(
  (p) =>
    `/api/staff/time-tracking/payroll-rules/absence-types/${requirePathSegmentParam(p, "absenceTypeId")}`,
);
