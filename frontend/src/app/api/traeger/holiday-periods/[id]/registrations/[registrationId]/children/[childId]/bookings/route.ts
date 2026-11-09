import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Kind einer Ferienanmeldung umbuchen (#4663). Eine frei gewordene Woche
// geht an die Warteliste; die Elternabrechnung bucht um, damit sie den
// Beitrag der Nachrückenden gleich neu berechnet (#4699).
export const PUT = proxyPut(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/children/${requirePathSegmentParam(params, "childId")}/bookings`,
);
