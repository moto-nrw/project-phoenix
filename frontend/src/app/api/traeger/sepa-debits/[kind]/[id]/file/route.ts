import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Die SEPA-Lastschriftdatei (PAIN.008) eines ausgestellten Laufs (#4697).
// Sie trägt Bankdaten; niemand speichert sie zwischen.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/sepa-debits/${requirePathSegmentParam(params, "kind")}/${requirePathSegmentParam(params)}/file`,
  ),
);
