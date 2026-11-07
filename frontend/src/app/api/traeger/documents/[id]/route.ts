import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete } from "~/lib/traeger/route-wrapper.server";

// Ein Dokument löschen; alle Freigaben enden mit ihm (#5993).
export const DELETE = proxyDelete(
  (params) => `/traeger/documents/${requirePathSegmentParam(params)}`,
);
