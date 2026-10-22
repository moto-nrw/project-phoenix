// API client for the school's room categories (#4352). Every category carries
// the badge color its rooms show when they have no color of their own.
// Admin CRUD goes through /api/room-categories (backend /api/room-categories,
// rooms:read to list, rooms:update to change).

import { apiErrorFromResponse } from "./api-error";
import { MOTO_COLOR_PALETTE } from "./location-helper";
import { sessionFetch } from "./session-cache";

export interface RoomCategory {
  id: string;
  name: string;
  /** Hex color, e.g. "#283590" */
  color: string;
  /** Rooms that name this category; a used category cannot be deleted. */
  roomCount: number;
}

interface BackendRoomCategory {
  id: number;
  name: string;
  color: string;
  room_count: number;
}

export interface RoomCategoryPayload {
  name: string;
  color: string;
}

/**
 * Vorschläge für eine neue Kategorie: Farben, die keinen Status bedeuten. Das
 * Backend lehnt jede Statusfarbe ab, damit „Unterwegs“ oder „Krank“ nie wie
 * ein Raum aussieht.
 */
const SUGGESTED_COLORS: readonly string[] = [
  MOTO_COLOR_PALETTE.wine.base,
  MOTO_COLOR_PALETTE.gold.base,
  MOTO_COLOR_PALETTE.coral.base,
  MOTO_COLOR_PALETTE.teal.base,
];

/** Die erste vorgeschlagene Farbe, die noch keine Kategorie trägt. */
export function suggestedRoomCategoryColor(
  categories: readonly RoomCategory[],
): string {
  const used = new Set(categories.map((category) => category.color));
  return (
    SUGGESTED_COLORS.find((color) => !used.has(color)) ??
    MOTO_COLOR_PALETTE.wine.base
  );
}

export function mapRoomCategory(category: BackendRoomCategory): RoomCategory {
  return {
    id: String(category.id),
    name: category.name,
    color: category.color,
    roomCount: category.room_count,
  };
}

async function readOne(response: Response): Promise<RoomCategory> {
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      "Raumkategorie konnte nicht gespeichert werden",
    );
  }
  const json = (await response.json()) as { data: BackendRoomCategory };
  return mapRoomCategory(json.data);
}

class RoomCategoryService {
  async list(): Promise<RoomCategory[]> {
    const response = await sessionFetch("/api/room-categories");
    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        "Raumkategorien konnten nicht geladen werden",
      );
    }
    const json = (await response.json()) as {
      data: BackendRoomCategory[] | null;
    };
    return (json.data ?? []).map(mapRoomCategory);
  }

  async create(payload: RoomCategoryPayload): Promise<RoomCategory> {
    const response = await sessionFetch("/api/room-categories", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    return readOne(response);
  }

  async update(
    id: string,
    payload: RoomCategoryPayload,
  ): Promise<RoomCategory> {
    const response = await sessionFetch(`/api/room-categories/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    return readOne(response);
  }

  async remove(id: string): Promise<void> {
    const response = await sessionFetch(`/api/room-categories/${id}`, {
      method: "DELETE",
    });
    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        "Raumkategorie konnte nicht gelöscht werden",
      );
    }
  }
}

export const roomCategoryService = new RoomCategoryService();

/** Die Raumkategorien als Auswahl für das Raumformular. */
export async function roomCategoryOptions(): Promise<
  Array<{ value: string; label: string }>
> {
  const categories = await roomCategoryService.list();
  return categories.map((category) => ({
    value: category.name,
    label: category.name,
  }));
}
