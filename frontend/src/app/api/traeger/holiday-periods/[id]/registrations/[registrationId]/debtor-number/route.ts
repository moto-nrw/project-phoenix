import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Debitorennummer einer bestätigten Ferienanmeldung ändern (#5621). Die
// Elternabrechnung prüft, ob schon eine Rechnung ausgestellt ist, darum geht
// es an ihre Route.
export const PUT = proxyPut(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/debtor-number`,
);
