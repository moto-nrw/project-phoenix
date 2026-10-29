import { proxyFile, proxyDelete } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

const path = (params: Record<string, unknown>) =>
  `/traeger/holiday-periods/${requirePathSegmentParam(params)}/form/documents/${requirePathSegmentParam(params, "documentId")}`;
export const GET = proxyFile(path);
export const DELETE = proxyDelete(path);
