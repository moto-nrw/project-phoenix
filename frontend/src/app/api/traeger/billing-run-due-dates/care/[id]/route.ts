import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Fälligkeiten eines berechneten Monatslaufs je Zahlungsweg (#4689).
export const PUT = proxyPut(
  (params) =>
    `/traeger/billing-run-due-dates/care/${requirePathSegmentParam(params)}`,
);
