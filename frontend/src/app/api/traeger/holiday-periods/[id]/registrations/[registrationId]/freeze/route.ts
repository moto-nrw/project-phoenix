import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ferienanmeldung einfrieren (#4663).
export const POST = proxyPost(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/freeze`,
);
