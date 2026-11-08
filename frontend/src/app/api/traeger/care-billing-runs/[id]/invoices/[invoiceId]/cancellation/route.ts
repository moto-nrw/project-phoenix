import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Eine Monatsrechnung mit Begründung stornieren (#4684).
export const POST = proxyPost(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/invoices/${requirePathSegmentParam(params, "invoiceId")}/cancellation`,
);
