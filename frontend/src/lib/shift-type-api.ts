// API client for tenant-defined shift types (Schichtarten, #1836).
// Admin CRUD goes through /api/staff/shift-types (backend /api/shift-types,
// time_tracking:manage).

import { sessionFetch } from "./session-cache";
import {
  mapShiftType,
  type BackendShiftType,
  type ShiftType,
} from "./shift-type-helpers";

interface ShiftTypePayload {
  name: string;
  /** Hex color, e.g. "#83CD2D" */
  color: string;
  description: string;
  isActive: boolean;
  /**
   * Timetable-Kategorien mapped to this shift type (#1837 follow-up). When
   * provided (including an empty array) it replaces the current mapping;
   * omitted leaves the mapping untouched (matches the backend's nil semantics).
   */
  categoryIds?: string[];
}

export class ShiftTypeApiError extends Error {
  readonly status: number;
  readonly detail: string;

  constructor(status: number, detail: string) {
    super(`HTTP ${status}: ${detail}`);
    this.name = "ShiftTypeApiError";
    this.status = status;
    this.detail = detail;
  }
}

function toBackendBody(payload: ShiftTypePayload) {
  return {
    name: payload.name,
    color: payload.color,
    description: payload.description,
    is_active: payload.isActive,
    // Only send category_ids when the caller manages the mapping; omitting it
    // leaves the existing Kategorie↔Schichtart links untouched server-side.
    ...(payload.categoryIds !== undefined
      ? { category_ids: payload.categoryIds.map(Number) }
      : {}),
  };
}

async function readError(
  response: Response,
  fallback: string,
): Promise<ShiftTypeApiError> {
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
  return new ShiftTypeApiError(response.status, detail || fallback);
}

async function readList(response: Response): Promise<ShiftType[]> {
  if (!response.ok) {
    throw await readError(
      response,
      "Schichtarten konnten nicht geladen werden",
    );
  }
  const json = (await response.json()) as { data: BackendShiftType[] | null };
  return (json.data ?? []).map(mapShiftType);
}

async function readOne(response: Response): Promise<ShiftType> {
  if (!response.ok) {
    throw await readError(
      response,
      "Schichtart konnte nicht gespeichert werden",
    );
  }
  const json = (await response.json()) as { data: BackendShiftType };
  return mapShiftType(json.data);
}

class ShiftTypeService {
  async getShiftTypes(): Promise<ShiftType[]> {
    const response = await sessionFetch("/api/staff/shift-types");
    return readList(response);
  }

  async createShiftType(payload: ShiftTypePayload): Promise<ShiftType> {
    const response = await sessionFetch("/api/staff/shift-types", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(toBackendBody(payload)),
    });
    return readOne(response);
  }

  async updateShiftType(
    id: string,
    payload: ShiftTypePayload,
  ): Promise<ShiftType> {
    const response = await sessionFetch(`/api/staff/shift-types/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(toBackendBody(payload)),
    });
    return readOne(response);
  }

  async deleteShiftType(id: string): Promise<void> {
    const response = await sessionFetch(`/api/staff/shift-types/${id}`, {
      method: "DELETE",
    });
    if (!response.ok) {
      throw await readError(
        response,
        "Schichtart konnte nicht gelöscht werden",
      );
    }
  }

  /** Seeds the example shift types (idempotent) and returns the full list. */
  async createDefaults(): Promise<ShiftType[]> {
    const response = await sessionFetch("/api/staff/shift-types/defaults", {
      method: "POST",
    });
    return readList(response);
  }
}

export const shiftTypeService = new ShiftTypeService();
