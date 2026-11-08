import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete } from "~/lib/traeger/route-wrapper.server";

// Die Kinderzahl der Familie eines Vertrags entfernen (#5918).
export const DELETE = proxyDelete(
  (params) => `/traeger/family-children/${requirePathSegmentParam(params)}`,
);
