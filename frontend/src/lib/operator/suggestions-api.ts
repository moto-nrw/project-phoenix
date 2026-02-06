import { operatorFetch } from "./api-helpers";
import type {
  BackendOperatorSuggestion,
  OperatorSuggestion,
} from "./suggestions-helpers";
import { mapOperatorSuggestion } from "./suggestions-helpers";

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

  async updateStatus(id: string, status: string): Promise<void> {
    await operatorFetch<unknown>(`/api/operator/suggestions/${id}/status`, {
      method: "PUT",
      body: { status },
    });
  }

  async addComment(
    id: string,
    content: string,
    isInternal: boolean,
  ): Promise<void> {
    await operatorFetch<unknown>(`/api/operator/suggestions/${id}/comments`, {
      method: "POST",
      body: { content, is_internal: isInternal },
    });
  }

  async deleteComment(id: string, commentId: string): Promise<void> {
    await operatorFetch(
      `/api/operator/suggestions/${id}/comments/${commentId}`,
      { method: "DELETE" },
    );
  }

  async markCommentsRead(id: string): Promise<void> {
    await operatorFetch(`/api/operator/suggestions/${id}/comments/read`, {
      method: "POST",
    });
  }

  async fetchUnreadCount(): Promise<number> {
    const data = await operatorFetch<{ unread_count: number }>(
      "/api/operator/suggestions/unread-count",
    );
    return data.unread_count;
  }

  async markPostViewed(id: string): Promise<void> {
    await operatorFetch(`/api/operator/suggestions/${id}/view`, {
      method: "POST",
    });
  }

  async fetchUnviewedCount(): Promise<number> {
    const data = await operatorFetch<{ unviewed_count: number }>(
      "/api/operator/suggestions/unviewed-count",
    );
    return data.unviewed_count;
  }
}

export const operatorSuggestionsService = new OperatorSuggestionsService();
