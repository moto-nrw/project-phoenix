import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Ein Monatslauf (#4684): Vorschau oder ausgestellte Rechnungen.
export const GET = proxyGet(
  (params) => `/traeger/care-billing-runs/${requirePathSegmentParam(params)}`,
);
