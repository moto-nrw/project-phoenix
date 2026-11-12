import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Die Jahresbescheide eines Laufs ausstellen: jeder bekommt seine Nummer (#6219).
export const POST = proxyPost(
  (params) =>
    `/traeger/care-notice-runs/${requirePathSegmentParam(params)}/release`,
);
