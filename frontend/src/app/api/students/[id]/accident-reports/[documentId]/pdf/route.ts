import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { proxyFile } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Unfallanzeige (#4771) im Aufbau des amtlichen Vordrucks. Das Backend prüft
// student_accidents:read und protokolliert den Zugriff vor der Auslieferung.
export const GET = withPrivateDownloadNoStore(
  proxyFile(
    (p) =>
      `/api/students/${requirePathSegmentParam(p)}/accident-reports/${requirePathSegmentParam(p, "documentId")}/pdf`,
  ),
);
