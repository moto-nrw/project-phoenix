import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Liste eines Monatslaufs für einen Kostenträger als PDF, Excel oder CSV
// (#4687). Die Abfrage `format` geht mit.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/cost-shares/${requirePathSegmentParam(params, "costBearerId")}/export`,
  ),
);
