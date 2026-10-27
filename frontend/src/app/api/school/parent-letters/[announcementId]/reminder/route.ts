import { proxyPut } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// PUT /api/school/parent-letters/[announcementId]/reminder (#4800): die
// geplante Erinnerung eines eigenen Elternbriefs ändern.
export const PUT = proxyPut(
  (p) =>
    `/school/parent-letters/${requirePathSegmentParam(p, "announcementId")}/reminder`,
);
