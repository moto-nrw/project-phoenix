import {
  proxyGet,
  proxyPut,
  createParentDeleteHandler,
  parentApiDelete,
} from "~/lib/parent/route-wrapper.server";
import {
  buildQueryString,
  requirePathSegmentParam,
} from "~/lib/route-wrapper-utils.server";
import type { BackendFeedbackPost } from "../route";

interface UpdateFeedbackBody {
  school_id: string;
  title: string;
  description: string;
}

const postPath = (params: Record<string, unknown>) =>
  `/parent/me/feedback/${requirePathSegmentParam(params, "postId")}`;

/** Proxy GET /api/parent/me/feedback/{postId}?school_id= → backend. */
export const GET = proxyGet<BackendFeedbackPost>(postPath);

/** Proxy PUT /api/parent/me/feedback/{postId} → backend. */
export const PUT = proxyPut<BackendFeedbackPost, UpdateFeedbackBody>(postPath);

/**
 * Proxy DELETE /api/parent/me/feedback/{postId}?school_id= → backend.
 * Hand-rolled rather than proxyDelete because the addressed school travels in
 * the query string, which the pass-through delete factory does not forward.
 */
export const DELETE = createParentDeleteHandler<null>(
  async (request, token, params) => {
    await parentApiDelete(
      `${postPath(params)}${buildQueryString(request)}`,
      token,
    );
    return null;
  },
);
