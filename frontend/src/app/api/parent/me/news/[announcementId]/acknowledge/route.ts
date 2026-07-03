import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

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
export const POST = createParentPostHandler<AcknowledgeResult, StampBody>(
  async (_request, body, token, params) => {
    const announcementId = String(params.announcementId);
    return parentApiPost<AcknowledgeResult>(
      `/parent/me/news/${encodeURIComponent(announcementId)}/acknowledge`,
      token,
      { published_at: body?.published_at },
    );
  },
);
