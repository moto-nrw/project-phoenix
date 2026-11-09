import { bffError } from "~/lib/api-response-error.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { school } from "~/lib/school/route-wrapper.server";

// Einverständnis eines eigenen Elternbriefs in moto schule (#4800): den
// Nachweisbericht als PDF oder den Verlauf als CSV herunterladen. Derselbe
// Download wie im OGS-Portal (#3430), mit der Sitzung von moto schule.
const FORMATS = new Set(["csv", "pdf"]);

export const GET = withPrivateDownloadNoStore(
  school.route(async (request, _token, params) => {
    const format = request.nextUrl.searchParams.get("format") ?? "";
    if (!FORMATS.has(format)) {
      throw bffError(400, "format must be csv or pdf");
    }
    return school.downloadFile(
      request,
      `/school/parent-letters/${requirePathSegmentParam(params, "announcementId")}/declaration-export?format=${format}`,
      { backendCacheControl: true },
    );
  }),
);
