import { proxyGet } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Eine Schule des Trägers mit Kontaktdaten und Zahlen (#5686).
export const GET = proxyGet(
  (params) => `/traeger/schools/${requirePathSegmentParam(params)}`,
);
