import { proxyGet } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ferienbeitrag einer Anmeldung (#4664): festgehalten oder als Vorschau.
export const GET = proxyGet(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}`,
);
