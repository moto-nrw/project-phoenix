import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Alle Bestätigungsschreiben eines Laufs in einem PDF zum Drucken (#4670).
export const GET = proxyFile(
  (params) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/letters/print`,
);
