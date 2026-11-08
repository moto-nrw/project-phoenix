import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";

export const GET = withPrivateDownloadNoStore(
  proxyFile("/api/import/class-list-entries/template", {
    forwardQuery: ["format"],
    fallbackType: "text/csv; charset=utf-8",
    fallbackDisposition:
      'attachment; filename="klassenliste-import-vorlage.csv"',
  }),
);
