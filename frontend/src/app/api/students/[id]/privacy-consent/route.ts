import { proxyGet, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Without a stored consent the backend answers with the school's defaults.
export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/privacy-consent`,
);
export const PUT = proxyPut(
  (p) => `/api/students/${requirePathSegmentParam(p)}/privacy-consent`,
);
