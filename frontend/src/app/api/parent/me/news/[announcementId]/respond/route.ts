import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface RespondResult {
  answered: boolean;
}

interface RespondBody {
  student_id?: string;
  option_ids?: string[];
  published_at?: string;
}

/**
 * Proxy POST /api/parent/me/news/{announcementId}/respond → backend. Records the
 * guardian's answer to an Umfrage for ONE child (#1371); the backend authorizes
 * the child, verifies the published_at version and enforces the answer deadline.
 */
export const POST = proxyPost<RespondResult, RespondBody>(
  (params) =>
    `/parent/me/news/${requirePathSegmentParam(params, "announcementId")}/respond`,
);
