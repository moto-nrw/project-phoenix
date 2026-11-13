import { proxyGet } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Anmeldungen, deren Eingangsbestätigung oder Annahme-Mail die Eltern nicht
// erreicht hat (#6737): nur Nummern und Mailstand, keine Personendaten.
export const GET = proxyGet(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/mail-problems`,
);
