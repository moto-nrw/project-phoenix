import { proxyUpload } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const POST = proxyUpload(
  (params) =>
    `/traeger/holiday-periods/${requirePathSegmentParam(params)}/form/documents`,
  10 * 1024 * 1024,
  { method: "POST", tooLargeCode: "enrollment.legal_document_invalid" },
);
