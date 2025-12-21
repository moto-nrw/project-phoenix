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
  type BackendActiveGroup,
  type BackendVisit,
  type BackendSupervisor,
  type BackendCombinedGroup,
  type BackendGroupMapping,
  type BackendAnalytics,
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
 * Core fetch function that handles proxy vs backend routing, auth, and errors.
 * Returns discriminated union to allow callers to handle response appropriately.
 */
async function coreFetch(
  method: HttpMethod,
  proxyPath: string,
  backendPath: string,
  operationName: string,
  body?: unknown,
): Promise<
  { isProxy: true; response: Response } | { isProxy: false; data: unknown }
> {
  const useProxyApi = typeof window !== "undefined";
  const url = useProxyApi ? proxyPath : backendPath;

  try {
    if (useProxyApi) {
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

      return { isProxy: true, response };
    } else {
      let axiosResponse: { data: unknown };
      switch (method) {
        case "GET":
          axiosResponse = await api.get(url);
          break;
        case "POST":
          axiosResponse =
            body !== undefined
              ? await api.post(url, body)
              : await api.post(url);
          break;
        case "PUT":
          axiosResponse = await api.put(url, body);
          break;
        case "DELETE":
          axiosResponse = await api.delete(url);
          break;
      }
      return { isProxy: false, data: axiosResponse.data };
    }
  } catch (error) {
    console.error(`${operationName} error:`, error);
    throw error;
  }
}

/**
 * GET request returning a single item
 */
async function proxyGet<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend> {
  const result = await coreFetch("GET", proxyPath, backendPath, operationName);
  if (result.isProxy) {
    const responseData =
      (await result.response.json()) as ApiResponse<TBackend>;
    return mapper(responseData.data);
  } else {
    const axiosData = result.data as ApiResponse<TBackend>;
    return mapper(axiosData.data);
  }
}

/**
 * GET request returning an array of items
 */
async function proxyGetArray<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend[]> {
  const result = await coreFetch("GET", proxyPath, backendPath, operationName);
  if (result.isProxy) {
    const responseData = (await result.response.json()) as ApiResponse<
      TBackend[]
    >;
    return responseData.data.map(mapper);
  } else {
    const axiosData = result.data as ApiResponse<TBackend[]>;
    return axiosData.data.map(mapper);
  }
}

/**
 * POST request with body, returning a single item
 */
async function proxyPost<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  body: unknown,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend> {
  const result = await coreFetch(
    "POST",
    proxyPath,
    backendPath,
    operationName,
    body,
  );
  if (result.isProxy) {
    const responseData =
      (await result.response.json()) as ApiResponse<TBackend>;
    return mapper(responseData.data);
  } else {
    const axiosData = result.data as ApiResponse<TBackend>;
    return mapper(axiosData.data);
  }
}

/**
 * POST request without body, returning a single item
 */
async function proxyPostNoBody<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend> {
  const result = await coreFetch("POST", proxyPath, backendPath, operationName);
  if (result.isProxy) {
    const responseData =
      (await result.response.json()) as ApiResponse<TBackend>;
    return mapper(responseData.data);
  } else {
    const axiosData = result.data as ApiResponse<TBackend>;
    return mapper(axiosData.data);
  }
}

/**
 * PUT request with body, returning a single item
 */
async function proxyPut<TBackend, TFrontend>(
  proxyPath: string,
  backendPath: string,
  body: unknown,
  mapper: (data: TBackend) => TFrontend,
  operationName: string,
): Promise<TFrontend> {
  const result = await coreFetch(
    "PUT",
    proxyPath,
    backendPath,
    operationName,
    body,
  );
  if (result.isProxy) {
    const responseData =
      (await result.response.json()) as ApiResponse<TBackend>;
    return mapper(responseData.data);
  } else {
    const axiosData = result.data as ApiResponse<TBackend>;
    return mapper(axiosData.data);
  }
}

/**
 * DELETE request returning void
 */
async function proxyDelete(
  proxyPath: string,
  backendPath: string,
  operationName: string,
): Promise<void> {
  await coreFetch("DELETE", proxyPath, backendPath, operationName);
}

/**
 * POST request with body, returning void
 */
async function proxyPostVoid(
  proxyPath: string,
  backendPath: string,
  body: unknown,
  operationName: string,
): Promise<void> {
  await coreFetch("POST", proxyPath, backendPath, operationName, body);
}

export const activeService = {
  // Active Groups
  getActiveGroups: async (filters?: {
    active?: boolean;
  }): Promise<ActiveGroup[]> => {
    const params = new URLSearchParams();
    if (filters?.active !== undefined)
      params.append("active", filters.active.toString());

    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi
      ? "/api/active/groups"
      : `${env.NEXT_PUBLIC_API_URL}/active/groups`;

    const queryString = params.toString();
    if (queryString) {
      url += `?${queryString}`;
    }

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
            `Get active groups error: ${response.status}`,
            errorText,
          );
          throw new Error(`Get active groups failed: ${response.status}`);
        }

        const responseData = (await response.json()) as unknown;
        const groups =
          extractArrayFromResponse<BackendActiveGroup>(responseData);
        return groups.map(mapActiveGroupResponse);
      } else {
        const response = await api.get<unknown>(url, {
          params,
        });
        const groups = extractArrayFromResponse<BackendActiveGroup>(
          response.data,
        );
        return groups.map(mapActiveGroupResponse);
      }
    } catch (error) {
      console.error("Get active groups error:", error);
      throw error;
    }
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
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/active/groups/${id}/supervisors`
      : `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}/supervisors`;

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
            `Get active group supervisors error: ${response.status}`,
            errorText,
          );
          throw new Error(
            `Get active group supervisors failed: ${response.status}`,
          );
        }

        const responseData = (await response.json()) as unknown;
        const supervisors =
          extractArrayFromResponse<BackendSupervisor>(responseData);
        return supervisors.map(mapSupervisorResponse);
      } else {
        const response = await api.get<unknown>(url);
        const supervisors = extractArrayFromResponse<BackendSupervisor>(
          response.data,
        );
        return supervisors.map(mapSupervisorResponse);
      }
    } catch (error) {
      console.error("Get active group supervisors error:", error);
      throw error;
    }
  },

  createActiveGroup: async (
    activeGroup: Omit<
      ActiveGroup,
      "id" | "isActive" | "createdAt" | "updatedAt"
    >,
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
    const params = new URLSearchParams();
    if (filters?.active !== undefined)
      params.append("active", filters.active.toString());

    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi
      ? "/api/active/visits"
      : `${env.NEXT_PUBLIC_API_URL}/active/visits`;

    const queryString = params.toString();
    if (queryString) {
      url += `?${queryString}`;
    }

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
          console.error(`Get visits error: ${response.status}`, errorText);
          throw new Error(`Get visits failed: ${response.status}`);
        }

        const responseData = (await response.json()) as ApiResponse<
          BackendVisit[]
        >;
        return responseData.data.map(mapVisitResponse);
      } else {
        const response = await api.get<ApiResponse<BackendVisit[]>>(url, {
          params,
        });
        return response.data.data.map(mapVisitResponse);
      }
    } catch (error) {
      console.error("Get visits error:", error);
      throw error;
    }
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
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/active/visits/student/${studentId}/current`
      : `${env.NEXT_PUBLIC_API_URL}/active/visits/student/${studentId}/current`;

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
            `Get student current visit error: ${response.status}`,
            errorText,
          );
          throw new Error(
            `Get student current visit failed: ${response.status}`,
          );
        }

        const responseData =
          (await response.json()) as ApiResponse<BackendVisit | null>;
        return responseData.data ? mapVisitResponse(responseData.data) : null;
      } else {
        const response = await api.get<ApiResponse<BackendVisit | null>>(url);
        return response.data.data ? mapVisitResponse(response.data.data) : null;
      }
    } catch (error) {
      console.error("Get student current visit error:", error);
      throw error;
    }
  },

  getVisitsByGroup: async (groupId: string): Promise<Visit[]> => {
    return proxyGetArray<BackendVisit, Visit>(
      `/api/active/visits/group/${groupId}`,
      `${env.NEXT_PUBLIC_API_URL}/active/visits/group/${groupId}`,
      mapVisitResponse,
      "Get visits by group",
    );
  },

  createVisit: async (
    visit: Omit<Visit, "id" | "isActive" | "createdAt" | "updatedAt">,
  ): Promise<Visit> => {
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
    const params = new URLSearchParams();
    if (filters?.active !== undefined)
      params.append("active", filters.active.toString());

    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi
      ? "/api/active/supervisors"
      : `${env.NEXT_PUBLIC_API_URL}/active/supervisors`;

    const queryString = params.toString();
    if (queryString) {
      url += `?${queryString}`;
    }

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
          console.error(`Get supervisors error: ${response.status}`, errorText);
          throw new Error(`Get supervisors failed: ${response.status}`);
        }

        const responseData = (await response.json()) as ApiResponse<
          BackendSupervisor[]
        >;
        return responseData.data.map(mapSupervisorResponse);
      } else {
        const response = await api.get<ApiResponse<BackendSupervisor[]>>(url, {
          params,
        });
        return response.data.data.map(mapSupervisorResponse);
      }
    } catch (error) {
      console.error("Get supervisors error:", error);
      throw error;
    }
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
    supervisor: Omit<Supervisor, "id" | "isActive" | "createdAt" | "updatedAt">,
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
    const params = new URLSearchParams();
    if (filters?.active !== undefined)
      params.append("active", filters.active.toString());

    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi
      ? "/api/active/combined"
      : `${env.NEXT_PUBLIC_API_URL}/active/combined`;

    const queryString = params.toString();
    if (queryString) {
      url += `?${queryString}`;
    }

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
            `Get combined groups error: ${response.status}`,
            errorText,
          );
          throw new Error(`Get combined groups failed: ${response.status}`);
        }

        const responseData = (await response.json()) as ApiResponse<
          BackendCombinedGroup[]
        >;
        return responseData.data.map(mapCombinedGroupResponse);
      } else {
        const response = await api.get<ApiResponse<BackendCombinedGroup[]>>(
          url,
          { params },
        );
        return response.data.data.map(mapCombinedGroupResponse);
      }
    } catch (error) {
      console.error("Get combined groups error:", error);
      throw error;
    }
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
    combinedGroup: Omit<
      CombinedGroup,
      "id" | "isActive" | "createdAt" | "updatedAt"
    >,
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
    const useProxyApi = typeof window !== "undefined";
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
};
