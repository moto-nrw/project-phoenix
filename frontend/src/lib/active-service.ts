// lib/active-service.ts
import { getCachedSession, sessionFetch } from "./session-cache";
import api from "./api";
import { resolveApiUrl } from "./api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ActiveService" });
import {
  mapActiveGroupResponse,
  mapVisitResponse,
  mapSupervisorResponse,
  mapSchulhofStatusResponse,
  mapToggleSupervisionResponse,
  prepareActiveGroupForBackend,
  prepareVisitForBackend,
  prepareSupervisorForBackend,
  type ActiveGroup,
  type Visit,
  type Supervisor,
  type SchulhofStatus,
  type ToggleSupervisionResponse,
  type BackendActiveGroup,
  type BackendVisit,
  type BackendSupervisor,
  type BackendSchulhofStatus,
  type BackendToggleSupervisionResponse,
  type CreateActiveGroupInput,
  type CreateVisitInput,
  type CreateSupervisorInput,
  type TrackingIndicatorsResponse,
} from "./active-helpers";

// Generic API response interface
interface ApiResponse<T> {
  data: T;
  message?: string;
  status?: string;
}

interface TransitAssignSkipped {
  student_id: number;
  reason: string;
}

interface TransitAssignResult {
  assigned: number[];
  skipped: TransitAssignSkipped[];
  active_group_id: number;
  room_id: number;
}

interface StudentMoveSkipped {
  student_id: number;
  reason: string;
}

export interface StudentMoveResult {
  moved: number[];
  unchanged: number[];
  skipped: StudentMoveSkipped[];
  active_group_id?: number | null;
  room_id?: number | null;
}

export interface StudentMoveSummary {
  successCount: number;
  notPresentCount: number;
  otherSkippedCount: number;
}

export function summarizeStudentMoveResult(
  result: StudentMoveResult,
): StudentMoveSummary {
  const notPresentCount = result.skipped.filter(
    (item) => item.reason === "not_present",
  ).length;
  return {
    successCount: result.moved.length + result.unchanged.length,
    notPresentCount,
    otherSkippedCount: result.skipped.length - notPresentCount,
  };
}

interface BackendResponseEnvelope<T> {
  data: T;
  message?: string;
  status?: string;
  success?: boolean;
}

// Helper to extract array from potentially paginated response
function extractArrayFromResponse<T>(response: unknown): T[] {
  if (!response || typeof response !== "object") {
    return [];
  }

  const obj = response as Record<string, unknown>;

  // Check if response.data is an array (simple response)
  if (Array.isArray(obj.data)) {
    return obj.data as T[];
  }

  // Check if response.data.data is an array (paginated response)
  if (obj.data && typeof obj.data === "object") {
    const dataObj = obj.data as Record<string, unknown>;
    if (Array.isArray(dataObj.data)) {
      return dataObj.data as T[];
    }
  }

  return [];
}

function unwrapBackendEnvelope<T>(payload: T | BackendResponseEnvelope<T>): T {
  if (!payload || typeof payload !== "object") {
    return payload as T;
  }

  const obj = payload as Record<string, unknown>;
  if (
    "data" in obj &&
    ("status" in obj || "message" in obj || "success" in obj)
  ) {
    return obj.data as T;
  }

  return payload as T;
}

function mapVisitPayload(
  payload: BackendVisit | BackendResponseEnvelope<BackendVisit>,
): Visit {
  return mapVisitResponse(unwrapBackendEnvelope(payload));
}

function mapNullableVisitPayload(
  payload: BackendVisit | BackendResponseEnvelope<BackendVisit | null>,
): Visit | null {
  const visit = unwrapBackendEnvelope(payload);
  return visit ? mapVisitResponse(visit) : null;
}

// ============================================================================
// Proxy Fetch Helpers - Reduce boilerplate for proxy/backend API calls
// ============================================================================

type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";

function resolveBackendUrl(proxyPath: string, backendPath: string): string {
  return resolveApiUrl(proxyPath, backendPath);
}

/**
 * Execute proxy fetch request (browser context).
 * Handles session auth, headers, and error responses.
 */
async function executeProxyFetch(
  method: HttpMethod,
  url: string,
  operationName: string,
  body?: unknown,
): Promise<Response> {
  const response = await sessionFetch(url, {
    method,
    ...(body !== undefined && { body: JSON.stringify(body) }),
  });

  if (!response.ok) {
    const errorText = await response.text();
    logger.error("proxy fetch failed", {
      operation: operationName,
      status: response.status,
      error: errorText,
    });
    throw new Error(`${operationName} failed: ${response.status}`);
  }

  return response;
}

/**
 * Execute backend axios request (server context).
 */
async function executeBackendFetch<T>(
  method: HttpMethod,
  url: string,
  body?: unknown,
): Promise<T> {
  let response: { data: unknown };
  switch (method) {
    case "GET":
      response = await api.get(url);
      break;
    case "POST":
      response =
        body === undefined ? await api.post(url) : await api.post(url, body);
      break;
    case "PUT":
      response = await api.put(url, body);
      break;
    case "DELETE":
      response = await api.delete(url);
      break;
  }
  return response.data as T;
}

/**
 * Core fetch function that handles proxy vs backend routing, auth, errors, and response parsing.
 */
async function coreFetch<T>(
  method: HttpMethod,
  proxyPath: string,
  backendPath: string,
  operationName: string,
  body?: unknown,
): Promise<T> {
  const useProxyApi = globalThis.window !== undefined;

  try {
    if (useProxyApi) {
      const response = await executeProxyFetch(
        method,
        proxyPath,
        operationName,
        body,
      );
      const responseData = (await response.json()) as ApiResponse<T>;
      return responseData.data;
    } else {
      const responseData = await executeBackendFetch<ApiResponse<T>>(
        method,
        resolveBackendUrl(proxyPath, backendPath),
        body,
      );
      return responseData.data;
    }
  } catch (error) {
    logger.error("core fetch failed", {
      operation: operationName,
      error: String(error),
    });
    throw error;
  }
}

/** Core fetch for void operations (DELETE, POST without response). */
async function coreFetchVoid(
  method: HttpMethod,
  proxyPath: string,
  backendPath: string,
  operationName: string,
  body?: unknown,
): Promise<void> {
  const useProxyApi = globalThis.window !== undefined;

  try {
    if (useProxyApi) {
      await executeProxyFetch(method, proxyPath, operationName, body);
    } else {
      await executeBackendFetch<unknown>(
        method,
        resolveBackendUrl(proxyPath, backendPath),
        body,
      );
    }
  } catch (error) {
    logger.error("core fetch void failed", {
      operation: operationName,
      error: String(error),
    });
    throw error;
  }
}

/** GET request returning a single mapped item */
async function proxyGet<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend> {
  const data = await coreFetch<TBackend>(
    "GET",
    proxyPath,
    backendPath,
    operationName,
  );
  return mapper(data);
}

/** GET request returning a nullable mapped item */
async function proxyGetNullable<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend | null> {
  const data = await coreFetch<TBackend | null>(
    "GET",
    proxyPath,
    backendPath,
    operationName,
  );
  return data ? mapper(data) : null;
}

/** GET request returning an array of mapped items */
async function proxyGetArray<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend[]> {
  const data = await coreFetch<TBackend[]>(
    "GET",
    proxyPath,
    backendPath,
    operationName,
  );
  return data.map(mapper);
}

/** GET request returning paginated array (extracts from nested response) */
async function proxyGetPaginated<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend[]> {
  const useProxyApi = globalThis.window !== undefined;

  try {
    if (useProxyApi) {
      const response = await executeProxyFetch("GET", proxyPath, operationName);
      const responseData = (await response.json()) as unknown;
      const items = extractArrayFromResponse<TBackend>(responseData);
      return items.map(mapper);
    } else {
      const response = await executeBackendFetch<unknown>(
        "GET",
        resolveBackendUrl(proxyPath, backendPath),
      );
      const items = extractArrayFromResponse<TBackend>(response);
      return items.map(mapper);
    }
  } catch (error) {
    logger.error("paginated fetch failed", {
      operation: operationName,
      error: String(error),
    });
    throw error;
  }
}

/** POST request with body, returning a single mapped item */
async function proxyPost<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  body: unknown,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend> {
  const data = await coreFetch<TBackend>(
    "POST",
    proxyPath,
    backendPath,
    operationName,
    body,
  );
  return mapper(data);
}

/** POST request without body, returning a single mapped item */
async function proxyPostNoBody<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend> {
  const data = await coreFetch<TBackend>(
    "POST",
    proxyPath,
    backendPath,
    operationName,
  );
  return mapper(data);
}

/** PUT request with body, returning a single mapped item */
async function proxyPut<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  body: unknown,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend> {
  const data = await coreFetch<TBackend>(
    "PUT",
    proxyPath,
    backendPath,
    operationName,
    body,
  );
  return mapper(data);
}

/** DELETE request returning void */
async function proxyDelete(
  proxyPath: string,
  backendPath: string,
  operationName: string,
): Promise<void> {
  await coreFetchVoid("DELETE", proxyPath, backendPath, operationName);
}

/** POST request with body, returning void */
async function proxyPostVoid(
  proxyPath: string,
  backendPath: string,
  body: unknown,
  operationName: string,
): Promise<void> {
  await coreFetchVoid("POST", proxyPath, backendPath, operationName, body);
}

/** Build query string suffix for active filter */
function buildActiveFilterSuffix(filters?: { active?: boolean }): string {
  if (filters?.active === undefined) {
    return "";
  }
  return `?active=${filters.active.toString()}`;
}

export const activeService = {
  // Active Groups
  getActiveGroups: async (filters?: {
    active?: boolean;
  }): Promise<ActiveGroup[]> => {
    const suffix = buildActiveFilterSuffix(filters);
    return proxyGetPaginated<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups${suffix}`,
      `/active/groups${suffix}`,
      mapActiveGroupResponse,
      "Get active groups",
    );
  },

  getActiveGroup: async (id: string): Promise<ActiveGroup> => {
    return proxyGet<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups/${id}`,
      `/active/groups/${id}`,
      mapActiveGroupResponse,
      "Get active group",
    );
  },

  getActiveGroupsByRoom: async (roomId: string): Promise<ActiveGroup[]> => {
    return proxyGetArray<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups/room/${roomId}`,
      `/active/groups/room/${roomId}`,
      mapActiveGroupResponse,
      "Get active groups by room",
    );
  },

  getActiveGroupsByGroup: async (groupId: string): Promise<ActiveGroup[]> => {
    return proxyGetArray<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups/group/${groupId}`,
      `/active/groups/group/${groupId}`,
      mapActiveGroupResponse,
      "Get active groups by group",
    );
  },

  getActiveGroupVisits: async (id: string): Promise<Visit[]> => {
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/groups/${id}/visits`,
      `/active/groups/${id}/visits`,
      mapVisitResponse,
      "Get active group visits",
    );
  },

  // Bulk fetch visits with student display data (optimized for SSE - single query)
  getActiveGroupVisitsWithDisplay: async (id: string): Promise<Visit[]> => {
    const session = await getCachedSession();
    const response = await fetch(`/api/active/groups/${id}/visits/display`, {
      headers: {
        Authorization: `Bearer ${session?.user?.token}`,
        "Content-Type": "application/json",
      },
    });

    if (response.status === 404) {
      return [];
    }

    if (!response.ok) {
      const errorText = await response.text();
      logger.error("get visits with display failed", {
        status: response.status,
        error: errorText,
      });
      throw new Error(`Get visits with display failed: ${response.status}`);
    }

    const responseData = (await response.json()) as ApiResponse<BackendVisit[]>;
    return responseData.data.map(mapVisitResponse);
  },

  getActiveGroupSupervisors: async (id: string): Promise<Supervisor[]> => {
    return proxyGetPaginated<BackendSupervisor, Supervisor>(
      `/api/active/groups/${id}/supervisors`,
      `/active/groups/${id}/supervisors`,
      mapSupervisorResponse,
      "Get active group supervisors",
    );
  },

  createActiveGroup: async (
    activeGroup: CreateActiveGroupInput,
  ): Promise<ActiveGroup> => {
    const backendData = prepareActiveGroupForBackend(activeGroup);
    return proxyPost<BackendActiveGroup, ActiveGroup>(
      "/api/active/groups",
      "/active/groups",
      backendData,
      mapActiveGroupResponse,
      "Create active group",
    );
  },

  updateActiveGroup: async (
    id: string,
    activeGroup: Partial<ActiveGroup>,
  ): Promise<ActiveGroup> => {
    const backendData = prepareActiveGroupForBackend(activeGroup);
    return proxyPut<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups/${id}`,
      `/active/groups/${id}`,
      backendData,
      mapActiveGroupResponse,
      "Update active group",
    );
  },

  deleteActiveGroup: async (id: string): Promise<void> => {
    return proxyDelete(
      `/api/active/groups/${id}`,
      `/active/groups/${id}`,
      "Delete active group",
    );
  },

  endActiveGroup: async (id: string): Promise<ActiveGroup> => {
    return proxyPostNoBody<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups/${id}/end`,
      `/active/groups/${id}/end`,
      mapActiveGroupResponse,
      "End active group",
    );
  },

  // Visits
  getVisits: async (filters?: { active?: boolean }): Promise<Visit[]> => {
    const suffix = buildActiveFilterSuffix(filters);
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/visits${suffix}`,
      `/active/visits${suffix}`,
      mapVisitResponse,
      "Get visits",
    );
  },

  getVisit: async (id: string): Promise<Visit> => {
    return proxyGet<
      BackendVisit | BackendResponseEnvelope<BackendVisit>,
      Visit
    >(
      `/api/active/visits/${id}`,
      `/active/visits/${id}`,
      mapVisitPayload,
      "Get visit",
    );
  },

  getStudentVisits: async (studentId: string): Promise<Visit[]> => {
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/visits/student/${studentId}`,
      `/active/visits/student/${studentId}`,
      mapVisitResponse,
      "Get student visits",
    );
  },

  getStudentCurrentVisit: async (studentId: string): Promise<Visit | null> => {
    return proxyGetNullable<
      BackendVisit | BackendResponseEnvelope<BackendVisit | null>,
      Visit | null
    >(
      `/api/active/visits/student/${studentId}/current`,
      `/active/visits/student/${studentId}/current`,
      mapNullableVisitPayload,
      "Get student current visit",
    );
  },

  getVisitsByGroup: async (groupId: string): Promise<Visit[]> => {
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/visits/group/${groupId}`,
      `/active/visits/group/${groupId}`,
      mapVisitResponse,
      "Get visits by group",
    );
  },

  createVisit: async (visit: CreateVisitInput): Promise<Visit> => {
    const backendData = prepareVisitForBackend(visit);
    return proxyPost<
      BackendVisit | BackendResponseEnvelope<BackendVisit>,
      Visit
    >(
      "/api/active/visits",
      "/active/visits",
      backendData,
      mapVisitPayload,
      "Create visit",
    );
  },

  updateVisit: async (id: string, visit: Partial<Visit>): Promise<Visit> => {
    const backendData = prepareVisitForBackend(visit);
    return proxyPut<
      BackendVisit | BackendResponseEnvelope<BackendVisit>,
      Visit
    >(
      `/api/active/visits/${id}`,
      `/active/visits/${id}`,
      backendData,
      mapVisitPayload,
      "Update visit",
    );
  },

  deleteVisit: async (id: string): Promise<void> => {
    return proxyDelete(
      `/api/active/visits/${id}`,
      `/active/visits/${id}`,
      "Delete visit",
    );
  },

  endVisit: async (id: string): Promise<Visit> => {
    return proxyPostNoBody<
      BackendVisit | BackendResponseEnvelope<BackendVisit>,
      Visit
    >(
      `/api/active/visits/${id}/end`,
      `/active/visits/${id}/end`,
      mapVisitPayload,
      "End visit",
    );
  },

  // Supervisors
  getSupervisors: async (filters?: {
    active?: boolean;
  }): Promise<Supervisor[]> => {
    const suffix = buildActiveFilterSuffix(filters);
    return proxyGetArray<BackendSupervisor, Supervisor>(
      `/api/active/supervisors${suffix}`,
      `/active/supervisors${suffix}`,
      mapSupervisorResponse,
      "Get supervisors",
    );
  },

  getSupervisor: async (id: string): Promise<Supervisor> => {
    return proxyGet<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/${id}`,
      `/active/supervisors/${id}`,
      mapSupervisorResponse,
      "Get supervisor",
    );
  },

  getStaffSupervisions: async (staffId: string): Promise<Supervisor[]> => {
    return proxyGetArray<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/staff/${staffId}`,
      `/active/supervisors/staff/${staffId}`,
      mapSupervisorResponse,
      "Get staff supervisions",
    );
  },

  getStaffActiveSupervisions: async (
    staffId: string,
  ): Promise<Supervisor[]> => {
    return proxyGetArray<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/staff/${staffId}/active`,
      `/active/supervisors/staff/${staffId}/active`,
      mapSupervisorResponse,
      "Get staff active supervisions",
    );
  },

  getSupervisorsByGroup: async (groupId: string): Promise<Supervisor[]> => {
    return proxyGetArray<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/group/${groupId}`,
      `/active/supervisors/group/${groupId}`,
      mapSupervisorResponse,
      "Get supervisors by group",
    );
  },

  createSupervisor: async (
    supervisor: CreateSupervisorInput,
  ): Promise<Supervisor> => {
    const backendData = prepareSupervisorForBackend(supervisor);
    return proxyPost<BackendSupervisor, Supervisor>(
      "/api/active/supervisors",
      "/active/supervisors",
      backendData,
      mapSupervisorResponse,
      "Create supervisor",
    );
  },

  updateSupervisor: async (
    id: string,
    supervisor: Partial<Supervisor>,
  ): Promise<Supervisor> => {
    const backendData = prepareSupervisorForBackend(supervisor);
    return proxyPut<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/${id}`,
      `/active/supervisors/${id}`,
      backendData,
      mapSupervisorResponse,
      "Update supervisor",
    );
  },

  deleteSupervisor: async (id: string): Promise<void> => {
    return proxyDelete(
      `/api/active/supervisors/${id}`,
      `/active/supervisors/${id}`,
      "Delete supervisor",
    );
  },

  endSupervision: async (id: string): Promise<Supervisor> => {
    return proxyPostNoBody<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/${id}/end`,
      `/active/supervisors/${id}/end`,
      mapSupervisorResponse,
      "End supervision",
    );
  },

  // Unclaimed Groups (Deviceless Claiming)
  getUnclaimedGroups: async (): Promise<ActiveGroup[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const proxyPath = "/api/active/groups/unclaimed";
    const backendPath = "/active/groups/unclaimed";
    const url = resolveBackendUrl(proxyPath, backendPath);

    const metadataKeys = new Set([
      "status",
      "message",
      "success",
      "code",
      "meta",
      "pagination",
    ]);

    const extractGroupArray = (
      payload: unknown,
    ): BackendActiveGroup[] | undefined => {
      if (Array.isArray(payload)) {
        return payload as BackendActiveGroup[];
      }

      if (payload && typeof payload === "object") {
        const obj = payload as Record<string, unknown>;

        if ("data" in obj) {
          const fromData = extractGroupArray(obj.data);
          if (fromData !== undefined) {
            return fromData;
          }
        }

        if ("items" in obj) {
          const fromItems = extractGroupArray(obj.items);
          if (fromItems !== undefined) {
            return fromItems;
          }
        }
      }

      return undefined;
    };

    const payloadIsEffectivelyEmpty = (payload: unknown): boolean => {
      if (payload === null || payload === undefined) {
        return true;
      }

      if (Array.isArray(payload)) {
        return payload.length === 0;
      }

      if (typeof payload === "object") {
        const obj = payload as Record<string, unknown>;

        if ("data" in obj) {
          if (!payloadIsEffectivelyEmpty(obj.data)) {
            return false;
          }
        }

        if ("items" in obj) {
          if (!payloadIsEffectivelyEmpty(obj.items)) {
            return false;
          }
        }

        const remainingKeys = Object.keys(obj).filter(
          (key) => key !== "data" && key !== "items",
        );
        const nonMetaKeys = remainingKeys.filter(
          (key) => !metadataKeys.has(key),
        );

        if (nonMetaKeys.length > 0) {
          return false;
        }

        return true;
      }

      return false;
    };

    const parseUnclaimedGroupsPayload = (
      payload: unknown,
    ): BackendActiveGroup[] => {
      const extracted = extractGroupArray(payload);

      if (extracted !== undefined) {
        return extracted;
      }

      if (!payloadIsEffectivelyEmpty(payload)) {
        logger.warn("unexpected unclaimed groups response shape", {
          payload: JSON.stringify(payload),
        });
      }

      return [];
    };

    try {
      if (useProxyApi) {
        const session = await getCachedSession();
        const response = await fetch(url, {
          headers: {
            Authorization: `Bearer ${session?.user?.token}`,
            "Content-Type": "application/json",
          },
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("get unclaimed groups failed", {
            status: response.status,
            error: errorText,
          });
          throw new Error(`Get unclaimed groups failed: ${response.status}`);
        }

        const responseData = (await response.json()) as unknown;
        const rawGroups = parseUnclaimedGroupsPayload(responseData);
        return rawGroups.map(mapActiveGroupResponse);
      } else {
        const response = await api.get<unknown>(url);
        const rawGroups = parseUnclaimedGroupsPayload(response.data);
        return rawGroups.map(mapActiveGroupResponse);
      }
    } catch (error) {
      logger.error("get unclaimed groups error", { error: String(error) });
      throw error;
    }
  },

  claimActiveGroup: async (groupId: string): Promise<void> => {
    return proxyPostVoid(
      `/api/active/groups/${groupId}/claim`,
      `/active/groups/${groupId}/claim`,
      { role: "supervisor" },
      "Claim group",
    );
  },

  /**
   * Checkout a student for the day (daily checkout).
   * This ends their current visit AND toggles their attendance to checked_out.
   */
  checkoutStudent: async (studentId: string): Promise<void> => {
    return proxyPostVoid(
      `/api/active/visits/student/${studentId}/checkout`,
      `/active/visits/student/${studentId}/checkout`,
      {},
      "Checkout student",
    );
  },

  assignTransitStudents: async (
    studentIds: string[],
    activeGroupId: string,
  ): Promise<TransitAssignResult> => {
    return coreFetch<TransitAssignResult>(
      "POST",
      "/api/active/visits/transit/assign",
      "/active/visits/transit/assign",
      "Assign transit students",
      {
        student_ids: studentIds.map((id) => Number.parseInt(id, 10)),
        active_group_id: Number.parseInt(activeGroupId, 10),
      },
    );
  },

  moveStudentsToActiveGroup: async (
    studentIds: string[],
    activeGroupId: string,
  ): Promise<StudentMoveResult> => {
    return coreFetch<StudentMoveResult>(
      "POST",
      "/api/active/visits/move-to-group",
      "/active/visits/move-to-group",
      "Move students to active group",
      {
        student_ids: studentIds.map((id) => Number.parseInt(id, 10)),
        target_active_group_id: Number.parseInt(activeGroupId, 10),
      },
    );
  },

  // Schulhof (Schoolyard) - Permanent Tab Functions

  /**
   * Get the current Schulhof status including room info, supervisors, and student count.
   */
  getSchulhofStatus: async (): Promise<SchulhofStatus> => {
    return proxyGet<BackendSchulhofStatus, SchulhofStatus>(
      "/api/active/schulhof/status",
      "/active/schulhof/status",
      mapSchulhofStatusResponse,
      "Get Schulhof status",
    );
  },

  /**
   * Toggle Schulhof supervision for the current user.
   * @param action - "start" to begin supervising, "stop" to end supervision
   */
  toggleSchulhofSupervision: async (
    action: "start" | "stop",
  ): Promise<ToggleSupervisionResponse> => {
    return proxyPost<
      BackendToggleSupervisionResponse,
      ToggleSupervisionResponse
    >(
      "/api/active/schulhof/supervise",
      "/active/schulhof/supervise",
      { action },
      mapToggleSupervisionResponse,
      "Toggle Schulhof supervision",
    );
  },

  /**
   * Fetch tracking indicators for a set of students.
   * Returns configured labels and per-student match results (boolean array aligned with labels).
   * Returns empty labels/results when the feature is disabled or no labels are configured.
   */
  getTrackingIndicators: async (
    studentIds: string[],
  ): Promise<TrackingIndicatorsResponse> => {
    const empty: TrackingIndicatorsResponse = { labels: [], results: {} };
    if (studentIds.length === 0) return empty;

    const session = await getCachedSession();
    if (!session?.user?.token) return empty;

    const response = await fetch("/api/active/tracking-indicators", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        student_ids: studentIds.map((id) => Number.parseInt(id, 10)),
      }),
    });

    if (!response.ok) {
      logger.warn("tracking_indicators_fetch_failed", {
        status: response.status,
      });
      return empty;
    }

    const data =
      (await response.json()) as ApiResponse<TrackingIndicatorsResponse>;
    return data.data;
  },
};
