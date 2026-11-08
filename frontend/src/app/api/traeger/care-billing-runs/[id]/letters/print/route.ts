import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Die Schreiben eines Monatslaufs in einem PDF zum Drucken; die Abfrage
// `only=without_email` geht mit durch (#4684).
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/letters/print`,
  ),
);
