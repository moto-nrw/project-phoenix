import type { ActivityCategory } from "./activity-helpers";
import { apiErrorFromResponse, transportFetch } from "./api-error";

export interface PlannerRoomReference {
  readonly id: number | string;
  readonly name?: string;
  readonly room_name?: string;
  readonly building?: string | null;
}

export interface PlannerGroupReference {
  readonly id: number | string;
  readonly name: string;
}

interface ReferenceEnvelope<T> {
  readonly data?: T[];
  readonly pagination?: { readonly total_pages?: number };
}

function unwrapReferences<T>(body: ReferenceEnvelope<T> | T[]): T[] {
  return Array.isArray(body) ? body : (body.data ?? []);
}

async function readReferenceResponse<T>(response: Response): Promise<T> {
  if (response.ok === false) {
    throw await apiErrorFromResponse(
      response,
      `Reference request failed (${response.status})`,
    );
  }
  return (await response.json()) as T;
}

export async function fetchPlannerRooms(): Promise<PlannerRoomReference[]> {
  const pageSize = 1000;
  const rooms: PlannerRoomReference[] = [];
  let totalPages = 1;

  for (let page = 1; page <= totalPages; page += 1) {
    const response = await transportFetch(
      `/api/rooms?page=${page}&page_size=${pageSize}`,
      { credentials: "include" },
    );
    const body = await readReferenceResponse<
      ReferenceEnvelope<PlannerRoomReference> | PlannerRoomReference[]
    >(response);
    rooms.push(...unwrapReferences(body));
    if (Array.isArray(body) === false) {
      totalPages = body.pagination?.total_pages ?? 1;
    }
  }

  return rooms;
}

export async function fetchPlannerActivityCategories(): Promise<
  ActivityCategory[]
> {
  const response = await transportFetch("/api/activities/categories", {
    credentials: "include",
  });
  const body = await readReferenceResponse<
    ReferenceEnvelope<ActivityCategory> | ActivityCategory[]
  >(response);
  return unwrapReferences(body);
}

export async function fetchPlannerGroups(): Promise<PlannerGroupReference[]> {
  const response = await transportFetch("/api/groups?page_size=1000", {
    credentials: "include",
  });
  const body = await readReferenceResponse<
    ReferenceEnvelope<PlannerGroupReference> | PlannerGroupReference[]
  >(response);
  return unwrapReferences(body);
}
