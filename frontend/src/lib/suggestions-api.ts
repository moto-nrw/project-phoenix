// lib/suggestions-api.ts
// Client-side API functions for suggestions/voting board

import { authFetch } from "./api-helpers";
import type {
  BackendComment,
  BackendSuggestion,
  CreateSuggestionRequest,
  Suggestion,
  SuggestionComment,
  SortOption,
  UpdateSuggestionRequest,
  VoteRequest,
} from "./suggestions-helpers";
import {
  mapCommentResponse,
  mapSuggestionResponse,
} from "./suggestions-helpers";

// Proxy route wrapper envelope: { success: true, data: <payload> }
interface ProxyListResponse {
  success: boolean;
  data: BackendSuggestion[];
}

interface ProxySingleResponse {
  success: boolean;
  data: BackendSuggestion;
}

export async function fetchSuggestions(
  sort: SortOption = "score",
): Promise<Suggestion[]> {
  const response = await authFetch<ProxyListResponse>(
    `/api/suggestions?sort=${sort}`,
  );
  return response.data.map(mapSuggestionResponse);
}

export async function createSuggestion(
  data: CreateSuggestionRequest,
): Promise<Suggestion> {
  const response = await authFetch<ProxySingleResponse>("/api/suggestions", {
    method: "POST",
    body: data,
  });
  return mapSuggestionResponse(response.data);
}

export async function updateSuggestion(
  id: string,
  data: UpdateSuggestionRequest,
): Promise<Suggestion> {
  const response = await authFetch<ProxySingleResponse>(
    `/api/suggestions/${id}`,
    { method: "PUT", body: data },
  );
  return mapSuggestionResponse(response.data);
}

export async function deleteSuggestion(id: string): Promise<void> {
  await authFetch(`/api/suggestions/${id}`, { method: "DELETE" });
}

export async function voteSuggestion(
  id: string,
  direction: VoteRequest["direction"],
): Promise<Suggestion> {
  const response = await authFetch<ProxySingleResponse>(
    `/api/suggestions/${id}/vote`,
    { method: "POST", body: { direction } },
  );
  return mapSuggestionResponse(response.data);
}

export async function removeVote(id: string): Promise<Suggestion> {
  const response = await authFetch<ProxySingleResponse>(
    `/api/suggestions/${id}/vote`,
    { method: "DELETE" },
  );
  return mapSuggestionResponse(response.data);
}

// --- Comments ---

interface ProxyCommentListResponse {
  success: boolean;
  data: BackendComment[];
}

export async function fetchComments(
  postId: string,
): Promise<SuggestionComment[]> {
  const response = await authFetch<ProxyCommentListResponse>(
    `/api/suggestions/${postId}/comments`,
  );
  return response.data.map(mapCommentResponse);
}

export async function createComment(
  postId: string,
  content: string,
): Promise<void> {
  await authFetch(`/api/suggestions/${postId}/comments`, {
    method: "POST",
    body: { content },
  });
}

export async function deleteComment(
  postId: string,
  commentId: string,
): Promise<void> {
  await authFetch(`/api/suggestions/${postId}/comments/${commentId}`, {
    method: "DELETE",
  });
}

export async function markCommentsRead(postId: string): Promise<void> {
  await authFetch(`/api/suggestions/${postId}/comments/read`, {
    method: "POST",
  });
}

interface ProxyUnreadCountResponse {
  success: boolean;
  data: { unread_count: number };
}

export async function fetchUnreadCount(): Promise<number> {
  const response = await authFetch<ProxyUnreadCountResponse>(
    "/api/suggestions/unread-count",
  );
  return response.data.unread_count;
}
