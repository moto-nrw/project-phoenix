import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/students/${requirePathSegmentParam(p)}/attendance-history/export`,
    { forwardQuery: ["format", "from", "to"] },
  ),
);
