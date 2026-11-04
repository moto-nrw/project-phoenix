import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Liste eines freigegebenen Laufs für die Erstattung durch einen Kostenträger als PDF, Excel oder CSV (#5627).
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/billing-runs/${requirePathSegmentParam(params)}/cost-takeovers/${requirePathSegmentParam(params, "costBearerId")}/export`,
  ),
);
