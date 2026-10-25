import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ferienzeitraum veröffentlichen (#4659).
export const POST = proxyPost(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/publish`,
);
