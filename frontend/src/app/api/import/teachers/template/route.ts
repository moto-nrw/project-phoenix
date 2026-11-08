import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";

export const GET = withPrivateDownloadNoStore(
  proxyFile("/api/import/teachers/template", {
    forwardQuery: ["format"],
    fallbackType: "text/csv; charset=utf-8",
    fallbackDisposition:
      'attachment; filename="mitarbeiter-import-vorlage.csv"',
  }),
);
