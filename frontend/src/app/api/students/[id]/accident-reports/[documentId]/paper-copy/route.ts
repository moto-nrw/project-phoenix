import { proxyFileUpload, SCAN_UPLOAD } from "~/lib/file-upload-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Unterschriebene Papierfassung einer Unfallanzeige (#4771). Das Backend prüft
// den Inhalt, student_accidents:manage und dass die Anzeige noch offen ist.
export const POST = proxyFileUpload(
  (p) =>
    `/api/students/${requirePathSegmentParam(p)}/accident-reports/${requirePathSegmentParam(p, "documentId")}/paper-copy`,
  { ...SCAN_UPLOAD, file: "file" },
);
