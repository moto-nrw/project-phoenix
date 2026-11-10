import { proxyDelete } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Aussetzung aufheben (#4720); sie bleibt als Verlauf erhalten.
export const DELETE = proxyDelete(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/beleg-retention/suspensions/${requirePathSegmentParam(params, "suspensionId")}`,
);
