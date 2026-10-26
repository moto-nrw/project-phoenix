import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Vorschau des ersten Bestätigungsschreibens als PDF, ohne zu speichern (#4669).
export const POST = proxyFile(
  (params) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/letters/preview`,
  { method: "POST" },
);
