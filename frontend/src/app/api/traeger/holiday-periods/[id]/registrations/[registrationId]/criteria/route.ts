import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Das Büro prüft die Angaben einer Anmeldung zu den Kriterien (#4700).
export const PUT = proxyPut(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/criteria`,
);
