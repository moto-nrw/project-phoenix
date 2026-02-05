import { operatorFetch } from "./api-helpers";
import type {
  BackendOperatorSuggestion,
  OperatorSuggestion,
  OperatorComment,
  BackendOperatorComment,
} from "./suggestions-helpers";
import {
  mapOperatorSuggestion,
  mapOperatorComment,
} from "./suggestions-helpers";

class OperatorSuggestionsService {
  async fetchAll(
    status?: string,
    search?: string,
  ): Promise<OperatorSuggestion[]> {
    const params = new URLSearchParams();
    if (status && status !== "all") params.set("status", status);
    if (search) params.set("search", search);
    const qs = params.toString();
    const url = `/api/operator/suggestions${qs ? `?${qs}` : ""}`;
    const data = await operatorFetch<BackendOperatorSuggestion[]>(url);
    return data.map(mapOperatorSuggestion);
  }

  async fetchById(id: string): Promise<OperatorSuggestion> {
    const data = await operatorFetch<BackendOperatorSuggestion>(
      `/api/operator/suggestions/${id}`,
    );
    return mapOperatorSuggestion(data);
  }

  async updateStatus(id: string, status: string): Promise<OperatorSuggestion> {
    const data = await operatorFetch<BackendOperatorSuggestion>(
      `/api/operator/suggestions/${id}/status`,
      { method: "PUT", body: { status } },
    );
    return mapOperatorSuggestion(data);
  }

  async addComment(
    id: string,
    content: string,
    isInternal: boolean,
  ): Promise<OperatorComment> {
    const data = await operatorFetch<BackendOperatorComment>(
      `/api/operator/suggestions/${id}/comments`,
      { method: "POST", body: { content, is_internal: isInternal } },
    );
    return mapOperatorComment(data);
  }

  async deleteComment(id: string, commentId: string): Promise<void> {
    await operatorFetch(
      `/api/operator/suggestions/${id}/comments/${commentId}`,
      { method: "DELETE" },
    );
  }
}

export const operatorSuggestionsService = new OperatorSuggestionsService();
