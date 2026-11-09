import { proxyPost } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Prüfung eines Eintrags der Klärungsliste erfassen (#4720).
export const POST = proxyPost(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/beleg-retention/clarification-reviews`,
);
