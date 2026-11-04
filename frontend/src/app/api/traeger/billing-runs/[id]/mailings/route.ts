import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Der Versand der Bestätigungsschreiben eines Laufs per Mail (#4670): der
// Stand je Familie und das Senden an alle Familien.
export const GET = proxyGet(
  (params) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/mailings`,
);
export const POST = proxyPost(
  (params) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/mailings`,
);
