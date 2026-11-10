import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Die gespeicherten Jahresbescheide eines Laufs in einem PDF zum Drucken; die
// Abfrage `only=without_email` geht mit durch (#6347).
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/care-notice-runs/${requirePathSegmentParam(params)}/letters/print`,
  ),
);
