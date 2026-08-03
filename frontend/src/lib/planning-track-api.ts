import { sessionFetch } from "./session-cache";

interface BackendPlanningTrack {
  id: number;
  name: string;
  color: string;
  sort_order: number;
  archived_at?: string | null;
}

export interface PlanningTrack {
  id: string;
  name: string;
  color: string;
  sortOrder: number;
  archivedAt?: string;
}

interface PlanningTrackPayload {
  name: string;
  color: string;
  sort_order: number;
}

class PlanningTrackApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "PlanningTrackApiError";
    this.status = status;
  }
}

function mapTrack(track: BackendPlanningTrack): PlanningTrack {
  return {
    id: String(track.id),
    name: track.name,
    color: track.color,
    sortOrder: track.sort_order,
    archivedAt: track.archived_at ?? undefined,
  };
}

async function readError(response: Response, fallback: string): Promise<Error> {
  try {
    const body = (await response.json()) as { error?: string };
    return new PlanningTrackApiError(response.status, body.error || fallback);
  } catch {
    return new PlanningTrackApiError(response.status, fallback);
  }
}

async function readOne(response: Response, fallback: string) {
  if (!response.ok) throw await readError(response, fallback);
  const body = (await response.json()) as { data: BackendPlanningTrack };
  return mapTrack(body.data);
}

class PlanningTrackService {
  async list(): Promise<PlanningTrack[]> {
    const response = await sessionFetch("/api/timetable/planning-tracks");
    if (!response.ok) {
      throw await readError(
        response,
        "Planungsspuren konnten nicht geladen werden",
      );
    }
    const body = (await response.json()) as {
      data: BackendPlanningTrack[] | null;
    };
    return (body.data ?? []).map(mapTrack);
  }

  async create(payload: PlanningTrackPayload): Promise<PlanningTrack> {
    const response = await sessionFetch("/api/timetable/planning-tracks", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    return readOne(response, "Planungsspur konnte nicht angelegt werden");
  }

  async update(
    id: string,
    payload: PlanningTrackPayload,
  ): Promise<PlanningTrack> {
    const response = await sessionFetch(
      `/api/timetable/planning-tracks/${id}`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      },
    );
    return readOne(response, "Planungsspur konnte nicht gespeichert werden");
  }

  async reorder(ids: string[]): Promise<PlanningTrack[]> {
    const response = await sessionFetch(
      "/api/timetable/planning-tracks/order",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ids: ids.map(Number) }),
      },
    );
    if (!response.ok) {
      throw await readError(
        response,
        "Reihenfolge konnte nicht gespeichert werden",
      );
    }
    const body = (await response.json()) as {
      data: BackendPlanningTrack[] | null;
    };
    return (body.data ?? []).map(mapTrack);
  }

  async archive(id: string): Promise<void> {
    const response = await sessionFetch(
      `/api/timetable/planning-tracks/${id}`,
      {
        method: "DELETE",
      },
    );
    if (!response.ok) {
      throw await readError(
        response,
        "Planungsspur konnte nicht archiviert werden",
      );
    }
  }

  async restore(id: string): Promise<PlanningTrack> {
    return readOne(
      await sessionFetch(`/api/timetable/planning-tracks/${id}/restore`, {
        method: "POST",
      }),
      "Planungsspur konnte nicht wiederhergestellt werden",
    );
  }
}

export const planningTrackService = new PlanningTrackService();
