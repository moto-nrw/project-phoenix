import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";

/**
 * One entry of the parents portal's feedback board (#1678). The board is
 * addressed to the moto product team, not to the school, and guardians appear
 * under a pseudonym — hence author_name without any author id, plus is_own so
 * the app can still offer edit/delete on the guardian's own entries.
 */
export interface BackendFeedbackPost {
  id: string;
  title: string;
  description: string;
  author_name: string;
  is_own: boolean;
  status:
    "open" | "planned" | "in_progress" | "done" | "rejected" | "need_info";
  score: number;
  upvotes: number;
  downvotes: number;
  comment_count: number;
  unread_count: number;
  user_vote: "up" | "down" | null;
  created_at: string;
  updated_at: string;
}

interface CreateFeedbackBody {
  school_id: string;
  title: string;
  description: string;
}

/** Proxy GET /api/parent/me/feedback?school_id=&sort= → backend. */
export const GET = proxyGet<BackendFeedbackPost[]>("/parent/me/feedback");

/** Proxy POST /api/parent/me/feedback → backend. */
export const POST = proxyPost<BackendFeedbackPost, CreateFeedbackBody>(
  "/parent/me/feedback",
);
