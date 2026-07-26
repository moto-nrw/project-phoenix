import type { ActivityCategory } from "./activity-helpers";

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
}

function unwrapReferences<T>(body: ReferenceEnvelope<T> | T[]): T[] {
  return Array.isArray(body) ? body : (body.data ?? []);
}

async function readReferenceResponse<T>(response: Response): Promise<T> {
  if (response.ok === false) {
    throw new Error(`Reference request failed (${response.status})`);
  }
  return (await response.json()) as T;
}

export async function fetchPlannerRooms(): Promise<PlannerRoomReference[]> {
  const response = await fetch("/api/rooms", { credentials: "include" });
  const body = await readReferenceResponse<
    ReferenceEnvelope<PlannerRoomReference> | PlannerRoomReference[]
  >(response);
  return unwrapReferences(body);
}

export async function fetchPlannerActivityCategories(): Promise<
  ActivityCategory[]
> {
  const response = await fetch("/api/activities/categories", {
    credentials: "include",
  });
  const body = await readReferenceResponse<
    ReferenceEnvelope<ActivityCategory> | ActivityCategory[]
  >(response);
  return unwrapReferences(body);
}

export async function fetchPlannerGroups(): Promise<PlannerGroupReference[]> {
  const response = await fetch("/api/groups?page_size=1000", {
    credentials: "include",
  });
  const body = await readReferenceResponse<
    ReferenceEnvelope<PlannerGroupReference> | PlannerGroupReference[]
  >(response);
  return unwrapReferences(body);
}
