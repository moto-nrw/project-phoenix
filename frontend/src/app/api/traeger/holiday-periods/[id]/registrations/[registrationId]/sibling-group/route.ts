import { proxyDelete } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Eine Ferienanmeldung von ihren Geschwistern trennen (#5628).
export const DELETE = proxyDelete(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/sibling-groups/registrations/${requirePathSegmentParam(params, "registrationId")}`,
);
