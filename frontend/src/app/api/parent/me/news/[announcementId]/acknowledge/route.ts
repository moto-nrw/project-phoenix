import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface AcknowledgeResult {
  acknowledged: boolean;
}

interface StampBody {
  published_at?: string;
}

/**
 * Proxy POST /api/parent/me/news/{announcementId}/acknowledge → backend.
 * Records an explicit "gelesen und bestätigt" for an announcement that requires
 * acknowledgement. published_at is the version the backend verifies to reject a
 * confirmation against since-corrected wording.
 */
export const POST = proxyPost<AcknowledgeResult, StampBody>(
  (params) =>
    `/parent/me/news/${requirePathSegmentParam(params, "announcementId")}/acknowledge`,
);
