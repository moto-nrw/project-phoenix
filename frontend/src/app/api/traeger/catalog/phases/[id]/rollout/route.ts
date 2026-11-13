import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Freigeben und übertragen (#4704) mit dem Fingerabdruck der Vorschau.
export const POST = proxyPost(
  (params) =>
    `/traeger/catalog/phases/${requirePathSegmentParam(params)}/rollout`,
);
