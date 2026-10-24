import { proxyPost } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// POST /api/school/class-day/homework/[schoolClass]: Hausaufgaben für eine
// zugewiesene Klasse und einen Tag eintragen (#4796).
export const POST = proxyPost(
  (p) =>
    `/school/class-day/homework/${requirePathSegmentParam(p, "schoolClass")}`,
);
