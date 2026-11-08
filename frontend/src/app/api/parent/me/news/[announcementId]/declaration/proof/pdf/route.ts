import { bffError } from "~/lib/api-response-error.server";
import { parent } from "~/lib/parent/route-wrapper.server";
import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Erklärungen (#3430), Elternseite: der Nachweis als PDF der Schule für dieses
// Konto und ein Kind. Ohne Zugriff oder ohne eigene Antwort antwortet das
// Backend mit 404.
export const GET = withPrivateDownloadNoStore(
  parent.route(async (request, _token, params) => {
    const studentId = request.nextUrl.searchParams.get("student_id") ?? "";
    if (!/^\d+$/.test(studentId)) {
      throw bffError(400, "student_id is required");
    }
    return parent.downloadFile(
      request,
      `/parent/me/news/${requirePathSegmentParam(params, "announcementId")}/declaration/proof?student_id=${studentId}&format=pdf`,
      { backendCacheControl: true },
    );
  }),
);
