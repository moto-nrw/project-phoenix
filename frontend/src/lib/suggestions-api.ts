// lib/suggestions-api.ts
// Client-side API functions for suggestions/voting board

import { authFetch } from "./api-helpers";
import type {
  BackendSuggestion,
  CreateSuggestionRequest,
  Suggestion,
  SortOption,
  UpdateSuggestionRequest,
  VoteRequest,
} from "./suggestions-helpers";
import { mapSuggestionResponse } from "./suggestions-helpers";

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
