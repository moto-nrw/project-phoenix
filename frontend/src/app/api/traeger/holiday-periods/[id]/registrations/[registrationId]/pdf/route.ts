import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Eine Ferienanmeldung als PDF (#6691). Das Backend protokolliert den Abruf.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/holiday-periods/${requirePathSegmentParam(params)}/registrations/${requirePathSegmentParam(params, "registrationId")}/pdf`,
  ),
);
