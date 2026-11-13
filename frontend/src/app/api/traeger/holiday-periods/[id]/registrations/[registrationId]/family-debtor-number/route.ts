import { proxyGet } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Debitorennummer, die die Familie dieser Ferienanmeldung beim Träger schon
// hat (#6692). Das Bestätigen schlägt sie vor; die Elternabrechnung hält sie.
export const GET = proxyGet(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/family-debtor-number`,
);
