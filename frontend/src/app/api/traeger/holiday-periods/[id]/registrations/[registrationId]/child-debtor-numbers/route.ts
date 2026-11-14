import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Debitorennummern der Kinder einer bestätigten Ferienanmeldung ändern,
// wenn der Träger die Nummer je Kind führt (#7007). Die Elternabrechnung
// liest die Kinder aus der Anmeldung, darum geht es an ihre Route.
export const PUT = proxyPut(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/child-debtor-numbers`,
);
