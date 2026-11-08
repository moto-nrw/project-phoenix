import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Die Schreiben eines freigegebenen Monatslaufs mit dem Stand jeder Mail
// lesen und einmal erstellen (#4684).
export const GET = proxyGet(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/letters`,
);

export const POST = proxyPost(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/letters`,
);
