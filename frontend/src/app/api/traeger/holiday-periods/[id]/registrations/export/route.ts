import { proxyFile } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Export der Ferienanmeldungen als CSV oder Excel (#4663).
export const GET = proxyFile(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/export`,
);
