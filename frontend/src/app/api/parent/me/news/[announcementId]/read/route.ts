import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface ReadResult {
  read: boolean;
}

interface StampBody {
  published_at?: string;
}

/**
 * Proxy POST /api/parent/me/news/{announcementId}/read → backend. Records that
 * the guardian opened the announcement. Audience + visibility are enforced
 * server-side; published_at is the version the backend verifies to reject a
 * stale read after a correction/republish.
 */
export const POST = proxyPost<ReadResult, StampBody>(
  (params) =>
    `/parent/me/news/${requirePathSegmentParam(params, "announcementId")}/read`,
);
