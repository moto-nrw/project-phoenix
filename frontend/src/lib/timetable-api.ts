/**
 * API client for the timetable feature.
 *
 * All calls go through Next.js proxy routes under `/api/timetable/*` which
 * inject the JWT server-side. The client mirrors the surface of the backend:
 * - getWeek(from, to)            → GET    /instances?from=&to=
 * - start(id), complete(id),     → POST   /instances/{id}/{action}
 *   cancel(id)
 * - materialize(from?, to?)      → POST   /materialize
 *
 * Spontaneous create (POST /instances) and edit (PUT /instances/{id}) are
 * intentionally not exposed yet — the backend handlers will land in a
 * follow-up PR.
 */

import { getSession } from "next-auth/react";
import { createLogger } from "./logger";
import type {
  BackendEnrichedInstance,
  BackendInstanceStatusResult,
  BackendMaterializeResult,
  BackendStartInstanceResult,
  BackendWeeklyInstancesResponse,
  CreateInstanceBody,
  EnrichedInstance,
  InstanceStatusResult,
  MaterializeResult,
  StartInstanceResult,
  WeeklyInstancesResponse,
} from "./timetable-types";
import {
  mapInstance,
  mapInstanceStatusResult,
  mapMaterializeResult,
  mapStartInstanceResult,
  mapWeeklyInstances,
} from "./timetable-helpers";

const logger = createLogger({ component: "TimetableApiClient" });

interface ApiEnvelope<T> {
  status: string;
  message?: string;
  data: T;
}

class TimetableApiError extends Error {
  readonly httpStatus: number;
  readonly code?: string;

  constructor(message: string, httpStatus: number, code?: string) {
    super(message);
    this.name = "TimetableApiError";
    this.httpStatus = httpStatus;
    this.code = code;
  }
}

async function unwrap<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `Anfrage fehlgeschlagen (HTTP ${response.status})`;
    let code: string | undefined;
    try {
      const body = (await response.json()) as { error?: string; code?: string };
      if (body.error) message = body.error;
      code = body.code;
    } catch {
      // Body wasn't JSON; keep the generic message.
    }
    throw new TimetableApiError(message, response.status, code);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const envelope = (await response.json()) as ApiEnvelope<T>;
  return envelope.data;
}

class TimetableService {
  /**
   * GET /api/timetable/instances?from=YYYY-MM-DD&to=YYYY-MM-DD.
   * Returns the materialized instances in the requested window plus
   * lightweight enrichment (room name, type, staff list, counts).
   */
  async getWeek(from: string, to: string): Promise<WeeklyInstancesResponse> {
    const session = await getSession();
    if (!session?.user?.token) {
      throw new TimetableApiError("Nicht angemeldet", 401, "UNAUTHENTICATED");
    }

    const params = new URLSearchParams({ from, to });
    const response = await fetch(`/api/timetable/instances?${params}`, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });

    const raw = await unwrap<BackendWeeklyInstancesResponse>(response);
    return mapWeeklyInstances(raw);
  }

  /**
   * POST /api/timetable/instances — creates a planned instance outside
   * the materialization flow. When activity_group_id is omitted the row
   * is purely spontaneous (is_spontaneous=true server-side). Returns the
   * enriched instance shape so the caller can splice it into the SWR
   * cache without a refetch.
   */
  async create(body: CreateInstanceBody): Promise<EnrichedInstance> {
    const response = await fetch("/api/timetable/instances", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });
    const raw = await unwrap<BackendEnrichedInstance>(response);
    logger.info("instance_created", {
      instance_id: raw.id,
      is_spontaneous: raw.is_spontaneous,
    });
    return mapInstance(raw);
  }

  /**
   * POST /api/timetable/instances/{id}/start.
   * Transitions a planned instance to active and creates the active.group
   * bridge. Returns soft warnings (room/staff/student conflicts) so the
   * caller can surface them.
   */
  async start(instanceId: string): Promise<StartInstanceResult> {
    return this.lifecycle<BackendStartInstanceResult, StartInstanceResult>(
      instanceId,
      "start",
      mapStartInstanceResult,
    );
  }

  /**
   * POST /api/timetable/instances/{id}/complete.
   * Transitions an active instance to completed. Closes the active.group,
   * marks remaining expected students as absent.
   */
  async complete(instanceId: string): Promise<InstanceStatusResult> {
    return this.lifecycle<BackendInstanceStatusResult, InstanceStatusResult>(
      instanceId,
      "complete",
      mapInstanceStatusResult,
    );
  }

  /**
   * POST /api/timetable/instances/{id}/cancel.
   * Cancels a planned or active instance. From active, ends the bridge
   * cleanly via active.EndActivitySession.
   */
  async cancel(instanceId: string): Promise<InstanceStatusResult> {
    return this.lifecycle<BackendInstanceStatusResult, InstanceStatusResult>(
      instanceId,
      "cancel",
      mapInstanceStatusResult,
    );
  }

  private async lifecycle<TBackend, TFront>(
    instanceId: string,
    action: "start" | "complete" | "cancel",
    mapper: (raw: TBackend) => TFront,
  ): Promise<TFront> {
    const response = await fetch(
      `/api/timetable/instances/${instanceId}/${action}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        credentials: "include",
        body: JSON.stringify({}),
      },
    );

    const raw = await unwrap<TBackend>(response);
    logger.info("instance_lifecycle_action", {
      instance_id: instanceId,
      action,
    });
    return mapper(raw);
  }

  /**
   * POST /api/timetable/materialize.
   * When called without arguments, the backend defaults to next Mon–Sun.
   * The planner UI passes the currently visible week so the user can re-run
   * materialization for that window.
   */
  async materialize(from?: string, to?: string): Promise<MaterializeResult> {
    const body: Record<string, string> = {};
    if (from) body.from_date = from;
    if (to) body.to_date = to;

    const response = await fetch("/api/timetable/materialize", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      credentials: "include",
      body: JSON.stringify(body),
    });

    const raw = await unwrap<BackendMaterializeResult>(response);
    logger.info("materialize_completed", {
      from: raw.from,
      to: raw.to,
      created: raw.instances_created,
    });
    return mapMaterializeResult(raw);
  }
}

export const timetableService = new TimetableService();
export { TimetableApiError };
