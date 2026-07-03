import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

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
export const POST = createParentPostHandler<ReadResult, StampBody>(
  async (_request, body, token, params) => {
    const announcementId = String(params.announcementId);
    return parentApiPost<ReadResult>(
      `/parent/me/news/${encodeURIComponent(announcementId)}/read`,
      token,
      { published_at: body?.published_at },
    );
  },
);
