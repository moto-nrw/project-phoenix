import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Das gespeicherte Original eines Jahresbescheids als PDF (#6347).
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/traeger/care-notice-runs/${requirePathSegmentParam(params)}/notices/${requirePathSegmentParam(params, "contractId")}/pdf`,
  ),
);
