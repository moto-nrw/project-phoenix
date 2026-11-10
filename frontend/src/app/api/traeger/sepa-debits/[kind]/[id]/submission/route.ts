import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Bestätigen, dass die Lastschriftdatei bei der Bank eingereicht ist (#4697).
export const POST = proxyPost(
  (params) =>
    `/traeger/sepa-debits/${requirePathSegmentParam(params, "kind")}/${requirePathSegmentParam(params)}/submission`,
);
