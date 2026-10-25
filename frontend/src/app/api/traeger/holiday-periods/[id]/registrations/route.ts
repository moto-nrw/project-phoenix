import { proxyGet } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ferienanmeldungen eines Zeitraums (#4663): Liste mit Filtern, nur mit dem
// Recht „Anmeldung verwalten“.
export const GET = proxyGet(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations`,
);
