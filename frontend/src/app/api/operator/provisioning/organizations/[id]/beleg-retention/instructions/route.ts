import { proxyPost } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Weisung des Trägers für eine Belegart erfassen (#4720).
export const POST = proxyPost(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/beleg-retention/instructions`,
);
