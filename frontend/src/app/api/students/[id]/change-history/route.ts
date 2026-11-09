import { proxyGet } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Änderungsverlauf of one child (#1455); the backend requires full access.
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/change-history`,
);
