import { operator } from "~/lib/operator/route-wrapper.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";

/** The billing key-date CSV (#2791) of one month. */
export const GET = withPrivateDownloadNoStore(
  operator.proxyFile("/operator/billing/key-date-counts/export", {
    forwardQuery: ["month"],
  }),
);
