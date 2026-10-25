import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Kind einer Ferienanmeldung umbuchen (#4663).
export const PUT = proxyPut(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/children/${requirePathSegmentParam(params, "childId")}/bookings`,
);
