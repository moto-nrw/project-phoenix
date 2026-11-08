import { bffError } from "~/lib/api-response-error.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { allowedQueryString } from "~/lib/route-wrapper-utils.server";
import { school } from "~/lib/school/route-wrapper.server";

// Fehlzeitenliste einer Klasse in moto schule (#4793) als PDF oder Excel
// herunterladen, mit der Sitzung von moto schule. Welche Klasse die Person
// sehen darf, prüft das Backend; es protokolliert jeden Export.
const FORMATS = new Set(["pdf", "xlsx"]);

export const GET = withPrivateDownloadNoStore(
  school.route(async (request) => {
    const format = request.nextUrl.searchParams.get("format") ?? "";
    if (!FORMATS.has(format)) {
      throw bffError(400, "format must be pdf or xlsx");
    }
    return school.downloadFile(
      request,
      `/school/class-day/absence-list/export${allowedQueryString(request, ["class", "from", "to", "format"])}`,
      { backendCacheControl: true },
    );
  }),
);
