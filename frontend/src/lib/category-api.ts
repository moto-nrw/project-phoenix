// API client for tenant-defined activity categories (#2131).
//
// Admin CRUD goes through /api/activities/categories (backend
// activities:manage_categories). Archiving is a DELETE — the backend never
// removes a category, so existing Termine and Aktivitäten keep theirs.

import { sessionFetch } from "./session-cache";
import type { ActivityCategory } from "./activity-helpers";

interface CategoryPayload {
  name: string;
  description: string;
  /** Hex color, e.g. "#83CD2D". Empty string clears it. */
  color: string;
}

export class CategoryApiError extends Error {
  readonly status: number;
  readonly detail: string;

  constructor(status: number, detail: string) {
    super(`HTTP ${status}: ${detail}`);
    this.name = "CategoryApiError";
    this.status = status;
    this.detail = detail;
  }
}

async function readError(
  response: Response,
  fallback: string,
): Promise<CategoryApiError> {
  let detail = "";
  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.includes("application/json")) {
    try {
      const body = (await response.json()) as { error?: string };
      detail = body.error ?? "";
    } catch {
      detail = "";
    }
  } else {
    detail = await response.text();
  }
  return new CategoryApiError(response.status, detail || fallback);
}

async function readOne(
  response: Response,
  fallback: string,
): Promise<ActivityCategory> {
  if (!response.ok) {
    throw await readError(response, fallback);
  }
  const json = (await response.json()) as { data: ActivityCategory };
  return json.data;
}

class CategoryService {
  /**
   * Lists the categories for the manage screen: archived ones included so they
   * can be restored, and usage counts opted into. The pickers deliberately do
   * NOT use this — a plain list skips the extra usage aggregate server-side.
   */
  async getManagedCategories(): Promise<ActivityCategory[]> {
    const response = await sessionFetch(
      "/api/activities/categories?include_archived=true&with_usage=true",
    );
    if (!response.ok) {
      throw await readError(
        response,
        "Kategorien konnten nicht geladen werden",
      );
    }
    const json = (await response.json()) as {
      data: ActivityCategory[] | null;
    };
    return json.data ?? [];
  }

  async createCategory(payload: CategoryPayload): Promise<ActivityCategory> {
    const response = await sessionFetch("/api/activities/categories", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    return readOne(response, "Kategorie konnte nicht angelegt werden");
  }

  async updateCategory(
    id: string,
    payload: CategoryPayload,
  ): Promise<ActivityCategory> {
    const response = await sessionFetch(`/api/activities/categories/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    return readOne(response, "Kategorie konnte nicht gespeichert werden");
  }

  /** Archives the category. Nothing is deleted server-side. */
  async archiveCategory(id: string): Promise<void> {
    const response = await sessionFetch(`/api/activities/categories/${id}`, {
      method: "DELETE",
    });
    if (!response.ok) {
      throw await readError(
        response,
        "Kategorie konnte nicht archiviert werden",
      );
    }
  }

  async restoreCategory(id: string): Promise<ActivityCategory> {
    const response = await sessionFetch(
      `/api/activities/categories/${id}/restore`,
      { method: "POST" },
    );
    return readOne(response, "Kategorie konnte nicht wiederhergestellt werden");
  }
}

export const categoryService = new CategoryService();
