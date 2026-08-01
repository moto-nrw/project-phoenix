import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * One reply in a feedback thread. Guardians are pseudonymous here too:
 * author_name carries "Elternteil"/"Verfasser", author_type tells the app
 * whether the moto product team answered, and is_own marks the caller's own
 * comments so they can delete them.
 */
export interface BackendFeedbackComment {
  id: string;
  content: string;
  author_name: string;
  author_type: "operator" | "user" | "parent";
  is_own: boolean;
  created_at: string;
}

interface CreateCommentBody {
  school_id: string;
  content: string;
}

const commentsPath = (params: Record<string, unknown>) =>
  `/parent/me/feedback/${requirePathSegmentParam(params, "postId")}/comments`;

/** Proxy GET /api/parent/me/feedback/{postId}/comments?school_id= → backend. */
export const GET = proxyGet<BackendFeedbackComment[]>(commentsPath);

/** Proxy POST /api/parent/me/feedback/{postId}/comments → backend. */
export const POST = proxyPost<{ created: boolean }, CreateCommentBody>(
  commentsPath,
);
