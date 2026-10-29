import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// CSV-Export des Zugriffsprotokolls (#4655).
export const GET = withPrivateDownloadNoStore(
  proxyFile("/traeger/access-log/export"),
);
