import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Das Bestätigungsschreiben einer Familie per Mail senden, zuerst oder
// erneut (#4670).
export const POST = proxyPost(
  (params) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/documents/${requirePathSegmentParam(params, "documentId")}/mailings`,
);
