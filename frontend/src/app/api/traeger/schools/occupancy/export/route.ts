import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// CSV-Export der Belegung (#4676).
export const GET = withPrivateDownloadNoStore(
  proxyFile("/traeger/schools/occupancy/export"),
);
