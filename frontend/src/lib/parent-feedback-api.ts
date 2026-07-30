// lib/parent-feedback-api.ts
// Client for the parents portal's feedback board (#1678).
//
// The board works exactly like the staff suggestion board — same entries, votes
// and threads — with two differences that shape this file:
//   1. every call is scoped to one school (school_id), because a guardian can
//      have children at several, and
//   2. guardians are pseudonymous, so entries carry no author id and ownership
//      arrives as is_own.

import { authFetch } from "./api-helpers";
import type {
  Suggestion,
  SuggestionComment,
  SortOption,
} from "./suggestions-helpers";
import type { SuggestionsBoardApi } from "./suggestions-board-api";

/** Event name the feedback badge listens on (the staff board uses its own). */
export const PARENT_FEEDBACK_UNREAD_EVENT = "parent-feedback-unread-refresh";

interface BackendFeedbackPost {
  id: string;
  title: string;
  description: string;
  author_name: string;
  is_own: boolean;
  status: Suggestion["status"];
  score: number;
  upvotes: number;
  downvotes: number;
  comment_count: number;
  unread_count: number;
  user_vote: "up" | "down" | null;
  created_at: string;
  updated_at: string;
}

interface BackendFeedbackComment {
  id: string;
  content: string;
  author_name: string;
  author_type: SuggestionComment["authorType"];
  is_own: boolean;
  created_at: string;
}

export interface FeedbackSchool {
  id: string;
  name: string;
}

interface ProxyResponse<T> {
  success: boolean;
  data: T;
}

/**
 * Maps a board entry onto the shared Suggestion shape so the staff board's
 * card, vote buttons and comment accordion render it unchanged. authorId stays
 * empty on purpose: there is no id to show, and ownership travels in isOwn.
 */
function mapFeedbackPost(data: BackendFeedbackPost): Suggestion {
  return {
    id: data.id,
    title: data.title,
    description: data.description,
    authorId: "",
    authorName: data.author_name,
    isOwn: data.is_own,
    status: data.status,
    score: data.score,
    upvotes: data.upvotes,
    downvotes: data.downvotes,
    commentCount: data.comment_count,
    unreadCount: data.unread_count,
    userVote: data.user_vote,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

function mapFeedbackComment(data: BackendFeedbackComment): SuggestionComment {
  return {
    id: data.id,
    content: data.content,
    authorId: "",
    authorName: data.author_name,
    authorType: data.author_type,
    isOwn: data.is_own,
    createdAt: data.created_at,
  };
}

export async function fetchFeedbackSchools(): Promise<FeedbackSchool[]> {
  const response = await authFetch<
    ProxyResponse<{ school_id: string; school_name: string }[]>
  >("/api/parent/me/feedback/schools");
  return response.data.map((school) => ({
    id: school.school_id,
    name: school.school_name,
  }));
}

export async function fetchFeedback(
  schoolId: string,
  sort: SortOption = "score",
): Promise<Suggestion[]> {
  const response = await authFetch<ProxyResponse<BackendFeedbackPost[]>>(
    `/api/parent/me/feedback?school_id=${encodeURIComponent(schoolId)}&sort=${sort}`,
  );
  return response.data.map(mapFeedbackPost);
}

export async function createFeedback(
  schoolId: string,
  title: string,
  description: string,
): Promise<Suggestion> {
  const response = await authFetch<ProxyResponse<BackendFeedbackPost>>(
    "/api/parent/me/feedback",
    { method: "POST", body: { school_id: schoolId, title, description } },
  );
  return mapFeedbackPost(response.data);
}

export async function updateFeedback(
  schoolId: string,
  id: string,
  title: string,
  description: string,
): Promise<Suggestion> {
  const response = await authFetch<ProxyResponse<BackendFeedbackPost>>(
    `/api/parent/me/feedback/${id}`,
    { method: "PUT", body: { school_id: schoolId, title, description } },
  );
  return mapFeedbackPost(response.data);
}

export async function deleteFeedback(
  schoolId: string,
  id: string,
): Promise<void> {
  await authFetch(
    `/api/parent/me/feedback/${id}?school_id=${encodeURIComponent(schoolId)}`,
    { method: "DELETE" },
  );
}

export async function fetchFeedbackUnreadCount(): Promise<number> {
  const response = await authFetch<ProxyResponse<{ unread_count: number }>>(
    "/api/parent/me/feedback/unread-count",
  );
  return response.data.unread_count;
}

/**
 * Binds the board adapter to one school, so the shared components can vote and
 * comment without knowing that this board is school-scoped at all.
 */
export function parentFeedbackBoardApi(schoolId: string): SuggestionsBoardApi {
  const query = `?school_id=${encodeURIComponent(schoolId)}`;
  return {
    vote: async (id, direction) => {
      const response = await authFetch<ProxyResponse<BackendFeedbackPost>>(
        `/api/parent/me/feedback/${id}/vote`,
        { method: "POST", body: { school_id: schoolId, direction } },
      );
      return mapFeedbackPost(response.data);
    },
    removeVote: async (id) => {
      const response = await authFetch<ProxyResponse<BackendFeedbackPost>>(
        `/api/parent/me/feedback/${id}/vote${query}`,
        { method: "DELETE" },
      );
      return mapFeedbackPost(response.data);
    },
    fetchComments: async (postId) => {
      const response = await authFetch<ProxyResponse<BackendFeedbackComment[]>>(
        `/api/parent/me/feedback/${postId}/comments${query}`,
      );
      return response.data.map(mapFeedbackComment);
    },
    createComment: async (postId, content) => {
      await authFetch(`/api/parent/me/feedback/${postId}/comments`, {
        method: "POST",
        body: { school_id: schoolId, content },
      });
    },
    deleteComment: async (postId, commentId) => {
      await authFetch(
        `/api/parent/me/feedback/${postId}/comments/${commentId}${query}`,
        { method: "DELETE" },
      );
    },
    markCommentsRead: async (postId) => {
      await authFetch(`/api/parent/me/feedback/${postId}/comments/read`, {
        method: "POST",
        body: { school_id: schoolId },
      });
    },
  };
}
