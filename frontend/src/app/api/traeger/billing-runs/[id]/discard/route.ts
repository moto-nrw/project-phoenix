import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Einen leeren offenen Ferienlauf verwerfen: er stellt nichts aus, der
// Ferienzeitraum ist danach frei für einen neuen Lauf (#6628).
export const POST = proxyPost(
  (params) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/discard`,
);
