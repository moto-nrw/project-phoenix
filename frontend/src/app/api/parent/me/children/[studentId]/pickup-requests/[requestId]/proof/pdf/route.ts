import { parent } from "~/lib/parent/route-wrapper.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requireNumericParam } from "~/lib/route-wrapper-utils.server";

// Abholerlaubnis (#4780), Elternseite: der Nachweis einer unterschriebenen
// Anfrage als PDF der Schule. Nur die Person, die unterschrieben hat, bekommt
// ihn; sonst antwortet das Backend mit 404.
export const GET = withPrivateDownloadNoStore(
  parent.proxyFile(
    (params) =>
      `/parent/me/children/${requireNumericParam(params, "studentId")}/pickup-requests/${requireNumericParam(params, "requestId")}/proof?format=pdf`,
    { backendCacheControl: true },
  ),
);
