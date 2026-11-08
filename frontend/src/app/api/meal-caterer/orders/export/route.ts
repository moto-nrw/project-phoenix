import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";

// Die Lieferliste der Schule (#3873): dieselbe Vorlage wie die des Caterers,
// ohne Namen. meals:read erzwingt das Backend.
export const GET = withPrivateDownloadNoStore(
  proxyFile("/api/meal-caterer/orders/export", {
    forwardQuery: true,
    fallbackType: "application/octet-stream",
    fallbackDisposition: "attachment",
  }),
);
