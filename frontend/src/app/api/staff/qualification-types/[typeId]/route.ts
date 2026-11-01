import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface QualificationTypeBody {
  name: string;
  validity_months: number | null;
  reminder_lead_weeks: number;
  mandatory: boolean;
  archived: boolean;
}

/**
 * PUT /api/staff/qualification-types/{typeId}
 * Eine Nachweisart ändern oder archivieren (#4808). Es gibt kein DELETE:
 * eingetragene Nachweise behalten ihre Art.
 */
export const PUT = proxyPut<unknown, QualificationTypeBody>(
  (p) =>
    `/api/staff/qualification-types/${requirePathSegmentParam(p, "typeId")}`,
);
