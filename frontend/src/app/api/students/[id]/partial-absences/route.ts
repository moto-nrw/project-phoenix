import { proxyGet, proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet(
  (p) => `/api/students/${requirePathSegmentParam(p)}/partial-absences`,
);

export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/partial-absences`,
);
