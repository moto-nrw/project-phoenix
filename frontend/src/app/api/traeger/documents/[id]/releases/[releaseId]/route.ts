import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete } from "~/lib/traeger/route-wrapper.server";

// Eine Freigabe zurücknehmen (#4692).
export const DELETE = proxyDelete(
  (params) =>
    `/traeger/documents/${requirePathSegmentParam(params)}/releases/${requirePathSegmentParam(params, "releaseId")}`,
);
