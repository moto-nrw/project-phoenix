// lib/active-service.ts
import { getSession } from "next-auth/react";
import { env } from "~/env";
import api from "./api";
import {
  mapActiveGroupResponse,
  mapVisitResponse,
  mapSupervisorResponse,
  mapCombinedGroupResponse,
  mapGroupMappingResponse,
  mapAnalyticsResponse,
  mapSchulhofStatusResponse,
  mapToggleSupervisionResponse,
  prepareActiveGroupForBackend,
  prepareVisitForBackend,
  prepareSupervisorForBackend,
  prepareCombinedGroupForBackend,
  prepareGroupMappingForBackend,
  type ActiveGroup,
  type Visit,
  type Supervisor,
  type CombinedGroup,
  type GroupMapping,
  type Analytics,
  type SchulhofStatus,
  type ToggleSupervisionResponse,
  type BackendActiveGroup,
  type BackendVisit,
  type BackendSupervisor,
  type BackendCombinedGroup,
  type BackendGroupMapping,
  type BackendAnalytics,
  type BackendSchulhofStatus,
  type BackendToggleSupervisionResponse,
  type CreateActiveGroupInput,
  type CreateVisitInput,
  type CreateSupervisorInput,
  type CreateCombinedGroupInput,
} from "./active-helpers";

// Generic API response interface
interface ApiResponse<T> {
  data: T;
  message?: string;
  status?: string;
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
  const session = await getSession();
  const fetchOptions: RequestInit = {
    method,
    headers: {
      Authorization: `Bearer ${session?.user?.token}`,
      "Content-Type": "application/json",
    },
  };
  if (body !== undefined) {
    fetchOptions.body = JSON.stringify(body);
  }

  const response = await fetch(url, fetchOptions);

  if (!response.ok) {
    const errorText = await response.text();
    console.error(`${operationName} error: ${response.status}`, errorText);
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
        backendPath,
        body,
      );
      return responseData.data;
    }
  } catch (error) {
    console.error(`${operationName} error:`, error);
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
      await executeBackendFetch<unknown>(method, backendPath, body);
    }
  } catch (error) {
    console.error(`${operationName} error:`, error);
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
      const response = await executeBackendFetch<unknown>("GET", backendPath);
      const items = extractArrayFromResponse<TBackend>(response);
      return items.map(mapper);
    }
  } catch (error) {
    console.error(`${operationName} error:`, error);
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
      `${env.NEXT_PUBLIC_API_URL}/active/groups${suffix}`,
      mapActiveGroupResponse,
      "Get active groups",
    );
  },

  getActiveGroup: async (id: string): Promise<ActiveGroup> => {
    return proxyGet<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}`,
      mapActiveGroupResponse,
      "Get active group",
    );
  },

  getActiveGroupsByRoom: async (roomId: string): Promise<ActiveGroup[]> => {
    return proxyGetArray<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups/room/${roomId}`,
      `${env.NEXT_PUBLIC_API_URL}/active/groups/room/${roomId}`,
      mapActiveGroupResponse,
      "Get active groups by room",
    );
  },

  getActiveGroupsByGroup: async (groupId: string): Promise<ActiveGroup[]> => {
    return proxyGetArray<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups/group/${groupId}`,
      `${env.NEXT_PUBLIC_API_URL}/active/groups/group/${groupId}`,
      mapActiveGroupResponse,
      "Get active groups by group",
    );
  },

  getActiveGroupVisits: async (id: string): Promise<Visit[]> => {
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/groups/${id}/visits`,
      `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}/visits`,
      mapVisitResponse,
      "Get active group visits",
    );
  },

  // Bulk fetch visits with student display data (optimized for SSE - single query)
  getActiveGroupVisitsWithDisplay: async (id: string): Promise<Visit[]> => {
    const session = await getSession();
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
      console.error(
        `Get visits with display error: ${response.status}`,
        errorText,
      );
      throw new Error(`Get visits with display failed: ${response.status}`);
    }

    const responseData = (await response.json()) as ApiResponse<BackendVisit[]>;
    return responseData.data.map(mapVisitResponse);
  },

  getActiveGroupSupervisors: async (id: string): Promise<Supervisor[]> => {
    return proxyGetPaginated<BackendSupervisor, Supervisor>(
      `/api/active/groups/${id}/supervisors`,
      `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}/supervisors`,
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
      `${env.NEXT_PUBLIC_API_URL}/active/groups`,
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
      `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}`,
      backendData,
      mapActiveGroupResponse,
      "Update active group",
    );
  },

  deleteActiveGroup: async (id: string): Promise<void> => {
    return proxyDelete(
      `/api/active/groups/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}`,
      "Delete active group",
    );
  },

  endActiveGroup: async (id: string): Promise<ActiveGroup> => {
    return proxyPostNoBody<BackendActiveGroup, ActiveGroup>(
      `/api/active/groups/${id}/end`,
      `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}/end`,
      mapActiveGroupResponse,
      "End active group",
    );
  },

  // Visits
  getVisits: async (filters?: { active?: boolean }): Promise<Visit[]> => {
    const suffix = buildActiveFilterSuffix(filters);
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/visits${suffix}`,
      `${env.NEXT_PUBLIC_API_URL}/active/visits${suffix}`,
      mapVisitResponse,
      "Get visits",
    );
  },

  getVisit: async (id: string): Promise<Visit> => {
    return proxyGet<BackendVisit, Visit>(
      `/api/active/visits/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/visits/${id}`,
      mapVisitResponse,
      "Get visit",
    );
  },

  getStudentVisits: async (studentId: string): Promise<Visit[]> => {
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/visits/student/${studentId}`,
      `${env.NEXT_PUBLIC_API_URL}/active/visits/student/${studentId}`,
      mapVisitResponse,
      "Get student visits",
    );
  },

  getStudentCurrentVisit: async (studentId: string): Promise<Visit | null> => {
    return proxyGetNullable<BackendVisit, Visit>(
      `/api/active/visits/student/${studentId}/current`,
      `${env.NEXT_PUBLIC_API_URL}/active/visits/student/${studentId}/current`,
      mapVisitResponse,
      "Get student current visit",
    );
  },

  getVisitsByGroup: async (groupId: string): Promise<Visit[]> => {
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/visits/group/${groupId}`,
      `${env.NEXT_PUBLIC_API_URL}/active/visits/group/${groupId}`,
      mapVisitResponse,
      "Get visits by group",
    );
  },

  createVisit: async (visit: CreateVisitInput): Promise<Visit> => {
    const backendData = prepareVisitForBackend(visit);
    return proxyPost<BackendVisit, Visit>(
      "/api/active/visits",
      `${env.NEXT_PUBLIC_API_URL}/active/visits`,
      backendData,
      mapVisitResponse,
      "Create visit",
    );
  },

  updateVisit: async (id: string, visit: Partial<Visit>): Promise<Visit> => {
    const backendData = prepareVisitForBackend(visit);
    return proxyPut<BackendVisit, Visit>(
      `/api/active/visits/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/visits/${id}`,
      backendData,
      mapVisitResponse,
      "Update visit",
    );
  },

  deleteVisit: async (id: string): Promise<void> => {
    return proxyDelete(
      `/api/active/visits/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/visits/${id}`,
      "Delete visit",
    );
  },

  endVisit: async (id: string): Promise<Visit> => {
    return proxyPostNoBody<BackendVisit, Visit>(
      `/api/active/visits/${id}/end`,
      `${env.NEXT_PUBLIC_API_URL}/active/visits/${id}/end`,
      mapVisitResponse,
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
      `${env.NEXT_PUBLIC_API_URL}/active/supervisors${suffix}`,
      mapSupervisorResponse,
      "Get supervisors",
    );
  },

  getSupervisor: async (id: string): Promise<Supervisor> => {
    return proxyGet<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/supervisors/${id}`,
      mapSupervisorResponse,
      "Get supervisor",
    );
  },

  getStaffSupervisions: async (staffId: string): Promise<Supervisor[]> => {
    return proxyGetArray<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/staff/${staffId}`,
      `${env.NEXT_PUBLIC_API_URL}/active/supervisors/staff/${staffId}`,
      mapSupervisorResponse,
      "Get staff supervisions",
    );
  },

  getStaffActiveSupervisions: async (
    staffId: string,
  ): Promise<Supervisor[]> => {
    return proxyGetArray<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/staff/${staffId}/active`,
      `${env.NEXT_PUBLIC_API_URL}/active/supervisors/staff/${staffId}/active`,
      mapSupervisorResponse,
      "Get staff active supervisions",
    );
  },

  getSupervisorsByGroup: async (groupId: string): Promise<Supervisor[]> => {
    return proxyGetArray<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/group/${groupId}`,
      `${env.NEXT_PUBLIC_API_URL}/active/supervisors/group/${groupId}`,
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
      `${env.NEXT_PUBLIC_API_URL}/active/supervisors`,
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
      `${env.NEXT_PUBLIC_API_URL}/active/supervisors/${id}`,
      backendData,
      mapSupervisorResponse,
      "Update supervisor",
    );
  },

  deleteSupervisor: async (id: string): Promise<void> => {
    return proxyDelete(
      `/api/active/supervisors/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/supervisors/${id}`,
      "Delete supervisor",
    );
  },

  endSupervision: async (id: string): Promise<Supervisor> => {
    return proxyPostNoBody<BackendSupervisor, Supervisor>(
      `/api/active/supervisors/${id}/end`,
      `${env.NEXT_PUBLIC_API_URL}/active/supervisors/${id}/end`,
      mapSupervisorResponse,
      "End supervision",
    );
  },

  // Combined Groups
  getCombinedGroups: async (filters?: {
    active?: boolean;
  }): Promise<CombinedGroup[]> => {
    const suffix = buildActiveFilterSuffix(filters);
    return proxyGetArray<BackendCombinedGroup, CombinedGroup>(
      `/api/active/combined${suffix}`,
      `${env.NEXT_PUBLIC_API_URL}/active/combined${suffix}`,
      mapCombinedGroupResponse,
      "Get combined groups",
    );
  },

  getActiveCombinedGroups: async (): Promise<CombinedGroup[]> => {
    return proxyGetArray<BackendCombinedGroup, CombinedGroup>(
      "/api/active/combined/active",
      `${env.NEXT_PUBLIC_API_URL}/active/combined/active`,
      mapCombinedGroupResponse,
      "Get active combined groups",
    );
  },

  getCombinedGroup: async (id: string): Promise<CombinedGroup> => {
    return proxyGet<BackendCombinedGroup, CombinedGroup>(
      `/api/active/combined/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}`,
      mapCombinedGroupResponse,
      "Get combined group",
    );
  },

  getCombinedGroupGroups: async (id: string): Promise<ActiveGroup[]> => {
    return proxyGetArray<BackendActiveGroup, ActiveGroup>(
      `/api/active/combined/${id}/groups`,
      `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}/groups`,
      mapActiveGroupResponse,
      "Get combined group groups",
    );
  },

  createCombinedGroup: async (
    combinedGroup: CreateCombinedGroupInput,
  ): Promise<CombinedGroup> => {
    const backendData = prepareCombinedGroupForBackend(combinedGroup);
    return proxyPost<BackendCombinedGroup, CombinedGroup>(
      "/api/active/combined",
      `${env.NEXT_PUBLIC_API_URL}/active/combined`,
      backendData,
      mapCombinedGroupResponse,
      "Create combined group",
    );
  },

  updateCombinedGroup: async (
    id: string,
    combinedGroup: Partial<CombinedGroup>,
  ): Promise<CombinedGroup> => {
    const backendData = prepareCombinedGroupForBackend(combinedGroup);
    return proxyPut<BackendCombinedGroup, CombinedGroup>(
      `/api/active/combined/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}`,
      backendData,
      mapCombinedGroupResponse,
      "Update combined group",
    );
  },

  deleteCombinedGroup: async (id: string): Promise<void> => {
    return proxyDelete(
      `/api/active/combined/${id}`,
      `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}`,
      "Delete combined group",
    );
  },

  endCombinedGroup: async (id: string): Promise<CombinedGroup> => {
    return proxyPostNoBody<BackendCombinedGroup, CombinedGroup>(
      `/api/active/combined/${id}/end`,
      `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}/end`,
      mapCombinedGroupResponse,
      "End combined group",
    );
  },

  // Group Mappings
  getGroupMappingsByGroup: async (groupId: string): Promise<GroupMapping[]> => {
    return proxyGetArray<BackendGroupMapping, GroupMapping>(
      `/api/active/mappings/group/${groupId}`,
      `${env.NEXT_PUBLIC_API_URL}/active/mappings/group/${groupId}`,
      mapGroupMappingResponse,
      "Get group mappings by group",
    );
  },

  getGroupMappingsByCombined: async (
    combinedId: string,
  ): Promise<GroupMapping[]> => {
    return proxyGetArray<BackendGroupMapping, GroupMapping>(
      `/api/active/mappings/combined/${combinedId}`,
      `${env.NEXT_PUBLIC_API_URL}/active/mappings/combined/${combinedId}`,
      mapGroupMappingResponse,
      "Get group mappings by combined",
    );
  },

  addGroupToCombination: async (
    activeGroupId: string,
    combinedGroupId: string,
  ): Promise<GroupMapping> => {
    const backendData = prepareGroupMappingForBackend({
      activeGroupId,
      combinedGroupId,
    });
    return proxyPost<BackendGroupMapping, GroupMapping>(
      "/api/active/mappings/add",
      `${env.NEXT_PUBLIC_API_URL}/active/mappings/add`,
      backendData,
      mapGroupMappingResponse,
      "Add group to combination",
    );
  },

  removeGroupFromCombination: async (
    activeGroupId: string,
    combinedGroupId: string,
  ): Promise<void> => {
    const backendData = prepareGroupMappingForBackend({
      activeGroupId,
      combinedGroupId,
    });
    return proxyPostVoid(
      "/api/active/mappings/remove",
      `${env.NEXT_PUBLIC_API_URL}/active/mappings/remove`,
      backendData,
      "Remove group from combination",
    );
  },

  // Analytics
  getAnalyticsCounts: async (): Promise<Analytics> => {
    return proxyGet<BackendAnalytics, Analytics>(
      "/api/active/analytics/counts",
      `${env.NEXT_PUBLIC_API_URL}/active/analytics/counts`,
      mapAnalyticsResponse,
      "Get analytics counts",
    );
  },

  getRoomUtilization: async (roomId: string): Promise<Analytics> => {
    return proxyGet<BackendAnalytics, Analytics>(
      `/api/active/analytics/room/${roomId}/utilization`,
      `${env.NEXT_PUBLIC_API_URL}/active/analytics/room/${roomId}/utilization`,
      mapAnalyticsResponse,
      "Get room utilization",
    );
  },

  getStudentAttendance: async (studentId: string): Promise<Analytics> => {
    return proxyGet<BackendAnalytics, Analytics>(
      `/api/active/analytics/student/${studentId}/attendance`,
      `${env.NEXT_PUBLIC_API_URL}/active/analytics/student/${studentId}/attendance`,
      mapAnalyticsResponse,
      "Get student attendance",
    );
  },

  // Unclaimed Groups (Deviceless Claiming)
  getUnclaimedGroups: async (): Promise<ActiveGroup[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/active/groups/unclaimed"
      : `${env.NEXT_PUBLIC_API_URL}/active/groups/unclaimed`;

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
        console.warn(
          "[active-service] Unexpected unclaimed groups response shape:",
          payload,
        );
      }

      return [];
    };

    try {
      if (useProxyApi) {
        const session = await getSession();
        const response = await fetch(url, {
          headers: {
            Authorization: `Bearer ${session?.user?.token}`,
            "Content-Type": "application/json",
          },
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(
            `Get unclaimed groups error: ${response.status}`,
            errorText,
          );
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
      console.error("Get unclaimed groups error:", error);
      throw error;
    }
  },

  claimActiveGroup: async (groupId: string): Promise<void> => {
    return proxyPostVoid(
      `/api/active/groups/${groupId}/claim`,
      `${env.NEXT_PUBLIC_API_URL}/active/groups/${groupId}/claim`,
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
      `${env.NEXT_PUBLIC_API_URL}/active/visits/student/${studentId}/checkout`,
      {},
      "Checkout student",
    );
  },

  // Schulhof (Schoolyard) - Permanent Tab Functions

  /**
   * Get the current Schulhof status including room info, supervisors, and student count.
   */
  getSchulhofStatus: async (): Promise<SchulhofStatus> => {
    return proxyGet<BackendSchulhofStatus, SchulhofStatus>(
      "/api/active/schulhof/status",
      `${env.NEXT_PUBLIC_API_URL}/active/schulhof/status`,
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
    return proxyPost<BackendToggleSupervisionResponse, ToggleSupervisionResponse>(
      "/api/active/schulhof/supervise",
      `${env.NEXT_PUBLIC_API_URL}/active/schulhof/supervise`,
      { action },
      mapToggleSupervisionResponse,
      "Toggle Schulhof supervision",
    );
  },
};
