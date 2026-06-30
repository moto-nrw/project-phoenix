import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

interface AcknowledgeResult {
  acknowledged: boolean;
}

/**
 * Proxy POST /api/parent/me/news/{announcementId}/acknowledge → backend.
 * Records an explicit "gelesen und bestätigt" for an announcement that requires
 * acknowledgement.
 */
export const POST = createParentPostHandler<AcknowledgeResult>(
  async (_request, _body, token, params) => {
    const announcementId = String(params.announcementId);
    return parentApiPost<AcknowledgeResult>(
      `/parent/me/news/${encodeURIComponent(announcementId)}/acknowledge`,
      token,
      {},
    );
  },
);
