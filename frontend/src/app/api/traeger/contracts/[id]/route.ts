import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyGet } from "~/lib/traeger/route-wrapper.server";

// Ein Betreuungsvertrag (#4678): lesen und einen Entwurf verwerfen.
export const GET = proxyGet(
  (params) => `/traeger/contracts/${requirePathSegmentParam(params)}`,
);

export const DELETE = proxyDelete(
  (params) => `/traeger/contracts/${requirePathSegmentParam(params)}`,
);
