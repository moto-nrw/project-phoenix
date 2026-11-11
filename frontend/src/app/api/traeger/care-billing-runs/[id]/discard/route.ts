import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Einen leeren offenen Monatslauf verwerfen: er stellt nichts aus, der Monat
// ist danach frei für einen neuen Lauf (#6626).
export const POST = proxyPost(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/discard`,
);
