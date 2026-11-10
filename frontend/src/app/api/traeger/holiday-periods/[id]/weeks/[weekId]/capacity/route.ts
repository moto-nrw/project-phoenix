import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Plätze einer Ferienwoche festlegen (#4699). Frei gewordene Plätze gehen an
// die Warteliste; die Elternabrechnung berechnet den Beitrag der
// Nachrückenden gleich neu, darum geht es an ihre Route.
export const PUT = proxyPut(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/weeks/${requirePathSegmentParam(params, "weekId")}/capacity`,
);
