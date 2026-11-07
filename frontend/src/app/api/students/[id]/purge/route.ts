import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Hard-deletes a child a grade transition graduated. Its own route rather than
// a flag on the students DELETE: the backend gate that hides alumni from every
// ordinary student route is what this one bypasses, so it stays greppable and
// cannot be reached by a stray query parameter.
export const DELETE = proxyDelete(
  (p) => `/api/students/${requirePathSegmentParam(p)}/purge`,
);
