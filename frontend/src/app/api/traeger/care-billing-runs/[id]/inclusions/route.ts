import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Eine ausgeschlossene Rechnung wieder in den Lauf nehmen (#5916): nur die
// Rechnung aus Vertrag und Leistung im Body (#6346). Die Antwort ist der Lauf.
export const POST = proxyPost(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/inclusions`,
);
