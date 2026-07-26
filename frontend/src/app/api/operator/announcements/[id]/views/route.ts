import type { BackendAnnouncementViewDetail } from "~/lib/operator/announcements-helpers";
import { proxyGet } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

export const GET = proxyGet<BackendAnnouncementViewDetail[]>(
  (params) =>
    `/operator/announcements/${requirePathSegmentParam(params)}/views`,
);
