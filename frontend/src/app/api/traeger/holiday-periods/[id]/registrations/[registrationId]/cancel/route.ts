import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ferienanmeldung stornieren (#4663). Die Elternabrechnung storniert, damit
// sie die Geschwistergruppe gleich neu berechnet (#5662), darum geht es an
// ihre Route.
export const POST = proxyPost(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/cancel`,
);
