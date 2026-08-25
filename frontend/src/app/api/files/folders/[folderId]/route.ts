// Dateiablage (#2596): rename / re-share / delete one folder.
import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const PUT = proxyPut(
  (p) => `/api/files/folders/${requirePathSegmentParam(p, "folderId")}`,
);
export const DELETE = proxyDelete(
  (p) => `/api/files/folders/${requirePathSegmentParam(p, "folderId")}`,
);
