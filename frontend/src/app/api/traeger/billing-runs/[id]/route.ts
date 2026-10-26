import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Ein Abrechnungslauf (#4668): Vorschau oder ausgestellte Rechnungen.
export const GET = proxyGet(
  (params) => `/traeger/billing-runs/${requirePathSegmentParam(params)}`,
);
