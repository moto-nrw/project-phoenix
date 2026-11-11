import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Einen leeren offenen Jahreslauf verwerfen: er stellt keinen Bescheid aus,
// das Jahr ist danach frei für einen neuen Lauf (#6626).
export const POST = proxyPost(
  (params) =>
    `/traeger/care-notice-runs/${requirePathSegmentParam(params)}/discard`,
);
