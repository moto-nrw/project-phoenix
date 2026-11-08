import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Die Prüfliste eines Monatslaufs vor der Freigabe als Excel oder CSV, ohne
// Rechnungsnummern (#5916). Die Abfrage `format` geht mit.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/review-export`,
  ),
);
