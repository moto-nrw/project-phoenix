import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Eine Rechnung in der Prüfphase ändern oder eine Änderung zurücknehmen (#6197).
export const POST = proxyPost(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/amendments`,
);
