import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Das Büro entscheidet über offene Plätze (#4700), einzeln oder mit
// „Vorschlag übernehmen“.
export const POST = proxyPost(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/allocation/decisions`,
);
