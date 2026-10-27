import { proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ferienanmeldung bestätigen und ihren Beitrag festhalten (#4663, #4664). Die
// Elternabrechnung bestätigt, darum geht es an ihre Route.
export const POST = proxyPost(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/confirm`,
);
