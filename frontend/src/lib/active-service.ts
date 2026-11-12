// lib/active-service.ts
import { hasSessionAccess } from "./session-access";
import { getCachedSession, sessionFetch } from "./session-cache";
import api from "./api";
import { createLogger } from "~/lib/logger";
import {
  apiErrorFromBody,
  transportFetch,
  type ApiError,
} from "~/lib/api-error";

const logger = createLogger({ component: "ActiveService" });
import {
  mapActiveGroupResponse,
  mapVisitResponse,
  mapSupervisorResponse,
  mapSchulhofStatusResponse,
  type ActiveGroup,
  type Visit,
  type Supervisor,
  type SchulhofStatus,
  type BackendActiveGroup,
  type BackendVisit,
  type BackendSupervisor,
  type BackendSchulhofStatus,
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

// ============================================================================
// Proxy Fetch Helpers - Reduce boilerplate for proxy/backend API calls
// ============================================================================

type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";

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
    // Code and details stay on the error (ADR 0006), so a refusal such as a
    // full room or activity can name itself in the UI.
    throw proxyError(
      `${operationName} failed: ${response.status}`,
      response.status,
      errorText,
    );
  }

  return response;
}

function proxyError(
  message: string,
  status: number,
  errorText: string,
): ApiError {
  let body: unknown;
  try {
    body = JSON.parse(errorText);
  } catch {
    body = undefined;
  }
  return apiErrorFromBody(message, status, body);
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
        proxyPath,
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
      await executeBackendFetch<unknown>(method, proxyPath, body);
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
      const response = await executeBackendFetch<unknown>("GET", proxyPath);
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
function buildActiveFilterSuffix(filters?: {
  active?: boolean;
  activeGroupIds?: readonly string[];
}): string {
  const params = new URLSearchParams();
  if (filters?.active !== undefined) {
    params.set("active", filters.active.toString());
  }
  if (filters?.activeGroupIds && filters.activeGroupIds.length > 0) {
    params.set("active_group_ids", filters.activeGroupIds.join(","));
  }
  const query = params.toString();
  return query ? `?${query}` : "";
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

  getActiveGroupSupervisors: async (id: string): Promise<Supervisor[]> => {
    return proxyGetPaginated<BackendSupervisor, Supervisor>(
      `/api/active/groups/${id}/supervisors`,
      `/active/groups/${id}/supervisors`,
      mapSupervisorResponse,
      "Get active group supervisors",
    );
  },

  // Visits
  getVisits: async (filters?: {
    active?: boolean;
    activeGroupIds?: readonly string[];
  }): Promise<Visit[]> => {
    const suffix = buildActiveFilterSuffix(filters);
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/visits${suffix}`,
      `/active/visits${suffix}`,
      mapVisitResponse,
      "Get visits",
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

  endSupervision: async (id: string): Promise<Supervisor> => {
    return proxyPostNoBody<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/${id}/end`,
      `/active/supervisors/${id}/end`,
      mapSupervisorResponse,
      "End supervision",
    );
  },

  claimActiveGroup: async (groupId: string): Promise<void> => {
    return proxyPostVoid(
      `/api/active/groups/${groupId}/claim`,
      `/active/groups/${groupId}/claim`,
      { role: "supervisor" },
      "Claim group",
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

  /**
   * Records that the children now use a released room (#3066): an independent
   * room stay, not participation in an activity running there. Rejects with an
   * ApiError whose `code` is `rooms.not_released` when the release was removed
   * after the room list was loaded, or a full room's code and details.
   */
  moveStudentsToOpenRoom: async (
    studentIds: string[],
    roomId: string,
  ): Promise<StudentMoveResult> => {
    const response = await sessionFetch("/api/active/visits/move-to-room", {
      method: "POST",
      body: JSON.stringify({
        student_ids: studentIds.map((id) => Number.parseInt(id, 10)),
        target_room_id: Number.parseInt(roomId, 10),
      }),
    });
    if (!response.ok) {
      const error = proxyError(
        `Move students to open room failed: ${response.status}`,
        response.status,
        await response.text(),
      );
      logger.error("proxy fetch failed", {
        operation: "Move students to open room",
        status: response.status,
        code: error.code,
      });
      throw error;
    }
    const payload = (await response.json()) as ApiResponse<StudentMoveResult>;
    return payload.data;
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
   * Fetch tracking indicators for a set of students.
   * Returns configured labels and per-student match results (boolean array aligned with labels).
   * Returns empty labels/results when the feature is disabled or no labels are configured.
   */
  getTrackingIndicators: async (
    studentIds: string[],
  ): Promise<TrackingIndicatorsResponse> => {
    const empty: TrackingIndicatorsResponse = { labels: [], results: {} };
    if (studentIds.length === 0) return empty;

    if (!hasSessionAccess(await getCachedSession())) return empty;

    const response = await transportFetch("/api/active/tracking-indicators", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        student_ids: studentIds.map((id) => Number.parseInt(id, 10)),
      }),
    });

    if (!response.ok) {
      // Bewusst still: die Markierungen sind eine Zusatzinfo; ohne sie zeigt
      // die Liste die Kinder unverändert.
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
