import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Eine ausgestellte Rechnung mit Begründung stornieren (#4690).
export const POST = proxyPost(
  (params) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/documents/${requirePathSegmentParam(params, "documentId")}/cancellation`,
);
