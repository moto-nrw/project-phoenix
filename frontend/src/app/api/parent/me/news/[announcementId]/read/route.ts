import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

interface ReadResult {
  read: boolean;
}

/**
 * Proxy POST /api/parent/me/news/{announcementId}/read → backend. Records that
 * the guardian opened the announcement. Audience + visibility are enforced
 * server-side.
 */
export const POST = createParentPostHandler<ReadResult>(
  async (_request, _body, token, params) => {
    const announcementId = String(params.announcementId);
    return parentApiPost<ReadResult>(
      `/parent/me/news/${encodeURIComponent(announcementId)}/read`,
      token,
      {},
    );
  },
);
