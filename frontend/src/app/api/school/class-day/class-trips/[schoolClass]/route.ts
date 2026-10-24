import { proxyPost } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// POST /api/school/class-day/class-trips/[schoolClass]: Ausflug oder
// Klassenfahrt einer zugewiesenen Klasse melden (#4799).
export const POST = proxyPost(
  (p) =>
    `/school/class-day/class-trips/${requirePathSegmentParam(p, "schoolClass")}`,
);
