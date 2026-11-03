import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Einen Teil einer ausgestellten Rechnung für ein Kind gutschreiben (#4690).
export const POST = proxyPost(
  (params) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/documents/${requirePathSegmentParam(params, "documentId")}/credits`,
);
