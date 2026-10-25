import { proxyPost } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Erfundene Beispielkinder für die Test-Schulen eines Test-Trägers (#4658).
export const POST = proxyPost(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/sample-children`,
);
