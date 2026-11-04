import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Das gespeicherte Bestätigungsschreiben einer Rechnung als PDF (#4669).
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/billing-runs/${requirePathSegmentParam(params)}/documents/${requirePathSegmentParam(params, "documentId")}/letter`,
  ),
);
