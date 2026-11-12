import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Die gespeicherten Originale der Jahresbescheide eines Laufs mit dem Stand
// jeder Mail lesen und einmal anlegen (#6347).
export const GET = proxyGet(
  (params) =>
    `/traeger/care-notice-runs/${requirePathSegmentParam(params)}/letters`,
);

export const POST = proxyPost(
  (params) =>
    `/traeger/care-notice-runs/${requirePathSegmentParam(params)}/letters`,
);
