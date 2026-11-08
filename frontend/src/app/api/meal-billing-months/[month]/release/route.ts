import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Die OGS-Leitung gibt einen Mensa-Monat frei (#4686). Danach rechnet der
// Träger ihn ab; die Freigabe ändert sich nicht mehr.
export const POST = proxyPost(
  (p) =>
    `/api/meal-billing-months/${requirePathSegmentParam(p, "month")}/release`,
);
