import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requireNumericParam } from "~/lib/route-wrapper-utils.server";

/** One staff member's time report as CSV or XLSX. */
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/api/staff/${requireNumericParam(params)}/time-tracking/export`,
    { forwardQuery: true },
  ),
);
