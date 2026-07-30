import {
  createParentDeleteHandler,
  parentApiDelete,
} from "~/lib/parent/route-wrapper.server";
import {
  buildQueryString,
  requirePathSegmentParam,
} from "~/lib/route-wrapper-utils.server";

/**
 * Proxy DELETE /api/parent/me/feedback/{postId}/comments/{commentId}?school_id=
 * → backend. Ownership of the comment is enforced server-side.
 */
export const DELETE = createParentDeleteHandler<null>(
  async (request, token, params) => {
    const postId = requirePathSegmentParam(params, "postId");
    const commentId = requirePathSegmentParam(params, "commentId");
    await parentApiDelete(
      `/parent/me/feedback/${postId}/comments/${commentId}${buildQueryString(request)}`,
      token,
    );
    return null;
  },
);
