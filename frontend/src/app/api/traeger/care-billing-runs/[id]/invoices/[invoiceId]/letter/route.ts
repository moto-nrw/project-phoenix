import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Das Schreiben einer Monatsrechnung oder, mit `kind=cancellation`, ihres
// Stornos als PDF (#4684).
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/invoices/${requirePathSegmentParam(params, "invoiceId")}/letter`,
  ),
);
