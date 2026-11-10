import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Das Träger-Büro entscheidet den Ferienwunsch eines angemeldeten Kindes
// (#4703): ja oder nein. Die Entscheidung ordnet nur die Warteliste.
export const PUT = proxyPut(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/children/${requirePathSegmentParam(params, "childId")}/holiday-wish`,
);
