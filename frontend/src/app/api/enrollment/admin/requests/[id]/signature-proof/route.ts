import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requireNumericParam } from "~/lib/route-wrapper-utils.server";

// Authenticated proxy for the staff proof of a signed enrollment (#4786):
// the backend answers the request detail with ?format=pdf as a PDF with the
// parent's signature.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (params) =>
      `/api/enrollment/admin/requests/${requireNumericParam(params)}?format=pdf`,
    { backendCacheControl: true },
  ),
);
