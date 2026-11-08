import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Vorschau des ersten Schreibens eines Monatslaufs als PDF, ohne zu
// speichern (#4684).
export const POST = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/letters/preview`,
    { method: "POST" },
  ),
);
