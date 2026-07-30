import {
  proxyPost,
  createParentDeleteHandler,
  parentApiDelete,
} from "~/lib/parent/route-wrapper.server";
import {
  buildQueryString,
  requirePathSegmentParam,
} from "~/lib/route-wrapper-utils.server";
import type { BackendFeedbackPost } from "../../route";

interface VoteBody {
  school_id: string;
  direction: "up" | "down";
}

const votePath = (params: Record<string, unknown>) =>
  `/parent/me/feedback/${requirePathSegmentParam(params, "postId")}/vote`;

/** Proxy POST /api/parent/me/feedback/{postId}/vote → backend. */
export const POST = proxyPost<BackendFeedbackPost, VoteBody>(votePath);

/** Proxy DELETE /api/parent/me/feedback/{postId}/vote?school_id= → backend. */
export const DELETE = createParentDeleteHandler<null>(
  async (request, token, params) => {
    await parentApiDelete(
      `${votePath(params)}${buildQueryString(request)}`,
      token,
    );
    return null;
  },
);
