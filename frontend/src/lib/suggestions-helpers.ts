// lib/suggestions-helpers.ts
// Type definitions and helper functions for suggestions/voting board

export interface BackendSuggestion {
  id: number;
  title: string;
  description: string;
  author_id: number;
  author_name: string;
  status:
    | "open"
    | "planned"
    | "in_progress"
    | "done"
    | "rejected"
    | "need_info";
  score: number;
  upvotes: number;
  downvotes: number;
  user_vote: "up" | "down" | null;
  created_at: string;
  updated_at: string;
}

export interface Suggestion {
  id: string;
  title: string;
  description: string;
  authorId: string;
  authorName: string;
  status:
    | "open"
    | "planned"
    | "in_progress"
    | "done"
    | "rejected"
    | "need_info";
  score: number;
  upvotes: number;
  downvotes: number;
  userVote: "up" | "down" | null;
  createdAt: string;
  updatedAt: string;
}

export interface CreateSuggestionRequest {
  title: string;
  description: string;
}

export interface UpdateSuggestionRequest {
  title: string;
  description: string;
}

export interface VoteRequest {
  direction: "up" | "down";
}

export function mapSuggestionResponse(data: BackendSuggestion): Suggestion {
  return {
    id: data.id.toString(),
    title: data.title,
    description: data.description,
    authorId: data.author_id.toString(),
    authorName: data.author_name,
    status: data.status,
    score: data.score,
    upvotes: data.upvotes,
    downvotes: data.downvotes,
    userVote: data.user_vote,
    createdAt: data.created_at,
    updatedAt: data.updated_at,
  };
}

export const STATUS_LABELS: Record<Suggestion["status"], string> = {
  open: "Offen",
  planned: "Geplant",
  in_progress: "In Bearbeitung",
  done: "Umgesetzt",
  rejected: "Abgelehnt",
  need_info: "Rückfrage",
};

export const STATUS_STYLES: Record<Suggestion["status"], string> = {
  open: "bg-gray-100 text-gray-700",
  planned: "bg-blue-100 text-blue-700",
  in_progress: "bg-yellow-100 text-yellow-800",
  done: "bg-green-100 text-green-700",
  rejected: "bg-red-100 text-red-700",
  need_info: "bg-purple-100 text-purple-700",
};

export type SortOption = "score" | "newest" | "status";

export const SORT_LABELS: Record<SortOption, string> = {
  score: "Beliebteste",
  newest: "Neueste",
  status: "Nach Status",
};
