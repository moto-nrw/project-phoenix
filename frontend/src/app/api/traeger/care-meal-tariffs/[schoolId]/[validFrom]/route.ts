import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete } from "~/lib/traeger/route-wrapper.server";

// Mensa-Preis einer Schule ab einem Monat löschen, solange keine Rechnung
// den Monat abrechnet (#4686).
export const DELETE = proxyDelete(
  (params) =>
    `/traeger/care-meal-tariffs/${requirePathSegmentParam(params, "schoolId")}/${requirePathSegmentParam(params, "validFrom")}`,
);
