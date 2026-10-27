import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Übersicht eines freigegebenen Laufs für die Kämmerei als PDF, Excel oder CSV (#4672).
export const GET = proxyFile(
  (params) => `/traeger/billing-runs/${requirePathSegmentParam(params)}/export`,
);
