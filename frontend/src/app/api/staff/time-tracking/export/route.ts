import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";

/** Cross-staff time-tracking export (#1417 2b) as CSV or XLSX. */
export const GET = withPrivateDownloadNoStore(
  proxyFile("/api/staff/time-tracking/export", { forwardQuery: true }),
);
