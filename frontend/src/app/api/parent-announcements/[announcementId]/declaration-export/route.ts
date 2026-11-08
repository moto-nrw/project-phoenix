import { bffError } from "~/lib/api-response-error.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { tenant } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Erklärungen (#3430): den Nachweisbericht als PDF oder den Verlauf als CSV
// herunterladen. Beide Dateien erzeugt das Backend.
const FORMATS = new Set(["csv", "pdf"]);

export const GET = withPrivateDownloadNoStore(
  tenant.route(async (request, _token, params) => {
    const format = request.nextUrl.searchParams.get("format") ?? "";
    if (!FORMATS.has(format)) {
      throw bffError(400, "format must be csv or pdf");
    }
    return tenant.downloadFile(
      request,
      `/api/parent-announcements/${requirePathSegmentParam(params, "announcementId")}/declaration-export?format=${format}`,
      { backendCacheControl: true },
    );
  }),
);
