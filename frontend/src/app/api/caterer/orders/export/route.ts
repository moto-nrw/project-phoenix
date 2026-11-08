import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { caterer } from "~/lib/caterer/route-wrapper.server";

// The caterer's delivery list of every school it supplies (ADR 0077), as
// PDF, Excel or CSV.
export const GET = withPrivateDownloadNoStore(
  caterer.proxyFile("/caterer/orders/export", {
    forwardQuery: true,
    fallbackType: "application/octet-stream",
    fallbackDisposition: "attachment",
  }),
);
