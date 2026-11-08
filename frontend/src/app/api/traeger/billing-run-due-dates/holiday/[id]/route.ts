import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Fälligkeiten eines berechneten Ferienlaufs je Zahlungsweg (#4689).
export const PUT = proxyPut(
  (params) =>
    `/traeger/billing-run-due-dates/holiday/${requirePathSegmentParam(params)}`,
);
