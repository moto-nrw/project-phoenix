import { proxyGet } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// GET /api/school/parent-letters/[announcementId]/declaration-status (#4800): nur für
// eigene Elternbriefe.
export const GET = proxyGet(
  (p) =>
    `/school/parent-letters/${requirePathSegmentParam(p, "announcementId")}/declaration-status`,
);
