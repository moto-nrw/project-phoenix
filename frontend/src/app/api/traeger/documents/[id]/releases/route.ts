import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Ein Dokument für Eltern oder Schulen freigeben (#4692).
export const POST = proxyPost(
  (params) => `/traeger/documents/${requirePathSegmentParam(params)}/releases`,
);
