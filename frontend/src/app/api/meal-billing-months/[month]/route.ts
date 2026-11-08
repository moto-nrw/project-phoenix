import { proxyGet } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Mensa-Monat mit seinen Kindern, Essen und Fehlzeiten (#4686).
export const GET = proxyGet(
  (p) => `/api/meal-billing-months/${requirePathSegmentParam(p, "month")}`,
);
