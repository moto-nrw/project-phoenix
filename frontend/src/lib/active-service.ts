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

export const activeService = {
    // Active Groups
    getActiveGroups: async (filters?: { active?: boolean }): Promise<ActiveGroup[]> => {
        const params = new URLSearchParams();
        if (filters?.active !== undefined) params.append("active", filters.active.toString());

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
                    console.error(`Get active groups error: ${response.status}`, errorText);
                    throw new Error(`Get active groups failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActiveGroup[]>;
                return responseData.data.map(mapActiveGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendActiveGroup[]>>(url, { params });
                return response.data.data.map(mapActiveGroupResponse);
            }
        } catch (error) {
            console.error("Get active groups error:", error);
            throw error;
        }
    },

    getActiveGroup: async (id: string): Promise<ActiveGroup> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/groups/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}`;

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
                    console.error(`Get active group error: ${response.status}`, errorText);
                    throw new Error(`Get active group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActiveGroup>;
                return mapActiveGroupResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendActiveGroup>>(url);
                return mapActiveGroupResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get active group error:", error);
            throw error;
        }
    },

    getActiveGroupsByRoom: async (roomId: string): Promise<ActiveGroup[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/groups/room/${roomId}`
            : `${env.NEXT_PUBLIC_API_URL}/active/groups/room/${roomId}`;

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
                    console.error(`Get active groups by room error: ${response.status}`, errorText);
                    throw new Error(`Get active groups by room failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActiveGroup[]>;
                return responseData.data.map(mapActiveGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendActiveGroup[]>>(url);
                return response.data.data.map(mapActiveGroupResponse);
            }
        } catch (error) {
            console.error("Get active groups by room error:", error);
            throw error;
        }
    },

    getActiveGroupsByGroup: async (groupId: string): Promise<ActiveGroup[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/groups/group/${groupId}`
            : `${env.NEXT_PUBLIC_API_URL}/active/groups/group/${groupId}`;

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
                    console.error(`Get active groups by group error: ${response.status}`, errorText);
                    throw new Error(`Get active groups by group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActiveGroup[]>;
                return responseData.data.map(mapActiveGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendActiveGroup[]>>(url);
                return response.data.data.map(mapActiveGroupResponse);
            }
        } catch (error) {
            console.error("Get active groups by group error:", error);
            throw error;
        }
    },

    getActiveGroupVisits: async (id: string): Promise<Visit[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/groups/${id}/visits`
            : `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}/visits`;

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
                    console.error(`Get active group visits error: ${response.status}`, errorText);
                    throw new Error(`Get active group visits failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendVisit[]>;
                return responseData.data.map(mapVisitResponse);
            } else {
                const response = await api.get<ApiResponse<BackendVisit[]>>(url);
                return response.data.data.map(mapVisitResponse);
            }
        } catch (error) {
            console.error("Get active group visits error:", error);
            throw error;
        }
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

        if (!response.ok) {
            const errorText = await response.text();
            console.error(`Get visits with display error: ${response.status}`, errorText);
            throw new Error(`Get visits with display failed: ${response.status}`);
        }

        const responseData = await response.json() as ApiResponse<BackendVisit[]>;
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
                    console.error(`Get active group supervisors error: ${response.status}`, errorText);
                    throw new Error(`Get active group supervisors failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendSupervisor[]>;
                return responseData.data.map(mapSupervisorResponse);
            } else {
                const response = await api.get<ApiResponse<BackendSupervisor[]>>(url);
                return response.data.data.map(mapSupervisorResponse);
            }
        } catch (error) {
            console.error("Get active group supervisors error:", error);
            throw error;
        }
    },

    createActiveGroup: async (activeGroup: Omit<ActiveGroup, "id" | "isActive" | "createdAt" | "updatedAt">): Promise<ActiveGroup> => {
        const backendData = prepareActiveGroupForBackend(activeGroup);

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/active/groups"
            : `${env.NEXT_PUBLIC_API_URL}/active/groups`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Create active group error: ${response.status}`, errorText);
                    throw new Error(`Create active group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActiveGroup>;
                return mapActiveGroupResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendActiveGroup>>(url, backendData);
                return mapActiveGroupResponse(response.data.data);
            }
        } catch (error) {
            console.error("Create active group error:", error);
            throw error;
        }
    },

    updateActiveGroup: async (id: string, activeGroup: Partial<ActiveGroup>): Promise<ActiveGroup> => {
        const backendData = prepareActiveGroupForBackend(activeGroup);

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/groups/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Update active group error: ${response.status}`, errorText);
                    throw new Error(`Update active group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActiveGroup>;
                return mapActiveGroupResponse(responseData.data);
            } else {
                const response = await api.put<ApiResponse<BackendActiveGroup>>(url, backendData);
                return mapActiveGroupResponse(response.data.data);
            }
        } catch (error) {
            console.error("Update active group error:", error);
            throw error;
        }
    },

    deleteActiveGroup: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/groups/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Delete active group error: ${response.status}`, errorText);
                    throw new Error(`Delete active group failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Delete active group error:", error);
            throw error;
        }
    },

    endActiveGroup: async (id: string): Promise<ActiveGroup> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/groups/${id}/end`
            : `${env.NEXT_PUBLIC_API_URL}/active/groups/${id}/end`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`End active group error: ${response.status}`, errorText);
                    throw new Error(`End active group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActiveGroup>;
                return mapActiveGroupResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendActiveGroup>>(url);
                return mapActiveGroupResponse(response.data.data);
            }
        } catch (error) {
            console.error("End active group error:", error);
            throw error;
        }
    },

    // Visits
    getVisits: async (filters?: { active?: boolean }): Promise<Visit[]> => {
        const params = new URLSearchParams();
        if (filters?.active !== undefined) params.append("active", filters.active.toString());

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

                const responseData = await response.json() as ApiResponse<BackendVisit[]>;
                return responseData.data.map(mapVisitResponse);
            } else {
                const response = await api.get<ApiResponse<BackendVisit[]>>(url, { params });
                return response.data.data.map(mapVisitResponse);
            }
        } catch (error) {
            console.error("Get visits error:", error);
            throw error;
        }
    },

    getVisit: async (id: string): Promise<Visit> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/visits/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/visits/${id}`;

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
                    console.error(`Get visit error: ${response.status}`, errorText);
                    throw new Error(`Get visit failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendVisit>;
                return mapVisitResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendVisit>>(url);
                return mapVisitResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get visit error:", error);
            throw error;
        }
    },

    getStudentVisits: async (studentId: string): Promise<Visit[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/visits/student/${studentId}`
            : `${env.NEXT_PUBLIC_API_URL}/active/visits/student/${studentId}`;

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
                    console.error(`Get student visits error: ${response.status}`, errorText);
                    throw new Error(`Get student visits failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendVisit[]>;
                return responseData.data.map(mapVisitResponse);
            } else {
                const response = await api.get<ApiResponse<BackendVisit[]>>(url);
                return response.data.data.map(mapVisitResponse);
            }
        } catch (error) {
            console.error("Get student visits error:", error);
            throw error;
        }
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
                    console.error(`Get student current visit error: ${response.status}`, errorText);
                    throw new Error(`Get student current visit failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendVisit | null>;
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
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/visits/group/${groupId}`
            : `${env.NEXT_PUBLIC_API_URL}/active/visits/group/${groupId}`;

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
                    console.error(`Get visits by group error: ${response.status}`, errorText);
                    throw new Error(`Get visits by group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendVisit[]>;
                return responseData.data.map(mapVisitResponse);
            } else {
                const response = await api.get<ApiResponse<BackendVisit[]>>(url);
                return response.data.data.map(mapVisitResponse);
            }
        } catch (error) {
            console.error("Get visits by group error:", error);
            throw error;
        }
    },

    createVisit: async (visit: Omit<Visit, "id" | "isActive" | "createdAt" | "updatedAt">): Promise<Visit> => {
        const backendData = prepareVisitForBackend(visit);

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/active/visits"
            : `${env.NEXT_PUBLIC_API_URL}/active/visits`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Create visit error: ${response.status}`, errorText);
                    throw new Error(`Create visit failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendVisit>;
                return mapVisitResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendVisit>>(url, backendData);
                return mapVisitResponse(response.data.data);
            }
        } catch (error) {
            console.error("Create visit error:", error);
            throw error;
        }
    },

    updateVisit: async (id: string, visit: Partial<Visit>): Promise<Visit> => {
        const backendData = prepareVisitForBackend(visit);

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/visits/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/visits/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Update visit error: ${response.status}`, errorText);
                    throw new Error(`Update visit failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendVisit>;
                return mapVisitResponse(responseData.data);
            } else {
                const response = await api.put<ApiResponse<BackendVisit>>(url, backendData);
                return mapVisitResponse(response.data.data);
            }
        } catch (error) {
            console.error("Update visit error:", error);
            throw error;
        }
    },

    deleteVisit: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/visits/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/visits/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Delete visit error: ${response.status}`, errorText);
                    throw new Error(`Delete visit failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Delete visit error:", error);
            throw error;
        }
    },

    endVisit: async (id: string): Promise<Visit> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/visits/${id}/end`
            : `${env.NEXT_PUBLIC_API_URL}/active/visits/${id}/end`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`End visit error: ${response.status}`, errorText);
                    throw new Error(`End visit failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendVisit>;
                return mapVisitResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendVisit>>(url);
                return mapVisitResponse(response.data.data);
            }
        } catch (error) {
            console.error("End visit error:", error);
            throw error;
        }
    },

    // Supervisors
    getSupervisors: async (filters?: { active?: boolean }): Promise<Supervisor[]> => {
        const params = new URLSearchParams();
        if (filters?.active !== undefined) params.append("active", filters.active.toString());

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

                const responseData = await response.json() as ApiResponse<BackendSupervisor[]>;
                return responseData.data.map(mapSupervisorResponse);
            } else {
                const response = await api.get<ApiResponse<BackendSupervisor[]>>(url, { params });
                return response.data.data.map(mapSupervisorResponse);
            }
        } catch (error) {
            console.error("Get supervisors error:", error);
            throw error;
        }
    },

    getSupervisor: async (id: string): Promise<Supervisor> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/supervisors/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/supervisors/${id}`;

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
                    console.error(`Get supervisor error: ${response.status}`, errorText);
                    throw new Error(`Get supervisor failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendSupervisor>;
                return mapSupervisorResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendSupervisor>>(url);
                return mapSupervisorResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get supervisor error:", error);
            throw error;
        }
    },

    getStaffSupervisions: async (staffId: string): Promise<Supervisor[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/supervisors/staff/${staffId}`
            : `${env.NEXT_PUBLIC_API_URL}/active/supervisors/staff/${staffId}`;

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
                    console.error(`Get staff supervisions error: ${response.status}`, errorText);
                    throw new Error(`Get staff supervisions failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendSupervisor[]>;
                return responseData.data.map(mapSupervisorResponse);
            } else {
                const response = await api.get<ApiResponse<BackendSupervisor[]>>(url);
                return response.data.data.map(mapSupervisorResponse);
            }
        } catch (error) {
            console.error("Get staff supervisions error:", error);
            throw error;
        }
    },

    getStaffActiveSupervisions: async (staffId: string): Promise<Supervisor[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/supervisors/staff/${staffId}/active`
            : `${env.NEXT_PUBLIC_API_URL}/active/supervisors/staff/${staffId}/active`;

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
                    console.error(`Get staff active supervisions error: ${response.status}`, errorText);
                    throw new Error(`Get staff active supervisions failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendSupervisor[]>;
                return responseData.data.map(mapSupervisorResponse);
            } else {
                const response = await api.get<ApiResponse<BackendSupervisor[]>>(url);
                return response.data.data.map(mapSupervisorResponse);
            }
        } catch (error) {
            console.error("Get staff active supervisions error:", error);
            throw error;
        }
    },

    getSupervisorsByGroup: async (groupId: string): Promise<Supervisor[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/supervisors/group/${groupId}`
            : `${env.NEXT_PUBLIC_API_URL}/active/supervisors/group/${groupId}`;

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
                    console.error(`Get supervisors by group error: ${response.status}`, errorText);
                    throw new Error(`Get supervisors by group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendSupervisor[]>;
                return responseData.data.map(mapSupervisorResponse);
            } else {
                const response = await api.get<ApiResponse<BackendSupervisor[]>>(url);
                return response.data.data.map(mapSupervisorResponse);
            }
        } catch (error) {
            console.error("Get supervisors by group error:", error);
            throw error;
        }
    },

    createSupervisor: async (supervisor: Omit<Supervisor, "id" | "isActive" | "createdAt" | "updatedAt">): Promise<Supervisor> => {
        const backendData = prepareSupervisorForBackend(supervisor);

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/active/supervisors"
            : `${env.NEXT_PUBLIC_API_URL}/active/supervisors`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Create supervisor error: ${response.status}`, errorText);
                    throw new Error(`Create supervisor failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendSupervisor>;
                return mapSupervisorResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendSupervisor>>(url, backendData);
                return mapSupervisorResponse(response.data.data);
            }
        } catch (error) {
            console.error("Create supervisor error:", error);
            throw error;
        }
    },

    updateSupervisor: async (id: string, supervisor: Partial<Supervisor>): Promise<Supervisor> => {
        const backendData = prepareSupervisorForBackend(supervisor);

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/supervisors/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/supervisors/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Update supervisor error: ${response.status}`, errorText);
                    throw new Error(`Update supervisor failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendSupervisor>;
                return mapSupervisorResponse(responseData.data);
            } else {
                const response = await api.put<ApiResponse<BackendSupervisor>>(url, backendData);
                return mapSupervisorResponse(response.data.data);
            }
        } catch (error) {
            console.error("Update supervisor error:", error);
            throw error;
        }
    },

    deleteSupervisor: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/supervisors/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/supervisors/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Delete supervisor error: ${response.status}`, errorText);
                    throw new Error(`Delete supervisor failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Delete supervisor error:", error);
            throw error;
        }
    },

    endSupervision: async (id: string): Promise<Supervisor> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/supervisors/${id}/end`
            : `${env.NEXT_PUBLIC_API_URL}/active/supervisors/${id}/end`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`End supervision error: ${response.status}`, errorText);
                    throw new Error(`End supervision failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendSupervisor>;
                return mapSupervisorResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendSupervisor>>(url);
                return mapSupervisorResponse(response.data.data);
            }
        } catch (error) {
            console.error("End supervision error:", error);
            throw error;
        }
    },

    // Combined Groups
    getCombinedGroups: async (filters?: { active?: boolean }): Promise<CombinedGroup[]> => {
        const params = new URLSearchParams();
        if (filters?.active !== undefined) params.append("active", filters.active.toString());

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
                    console.error(`Get combined groups error: ${response.status}`, errorText);
                    throw new Error(`Get combined groups failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendCombinedGroup[]>;
                return responseData.data.map(mapCombinedGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendCombinedGroup[]>>(url, { params });
                return response.data.data.map(mapCombinedGroupResponse);
            }
        } catch (error) {
            console.error("Get combined groups error:", error);
            throw error;
        }
    },

    getActiveCombinedGroups: async (): Promise<CombinedGroup[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/active/combined/active"
            : `${env.NEXT_PUBLIC_API_URL}/active/combined/active`;

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
                    console.error(`Get active combined groups error: ${response.status}`, errorText);
                    throw new Error(`Get active combined groups failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendCombinedGroup[]>;
                return responseData.data.map(mapCombinedGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendCombinedGroup[]>>(url);
                return response.data.data.map(mapCombinedGroupResponse);
            }
        } catch (error) {
            console.error("Get active combined groups error:", error);
            throw error;
        }
    },

    getCombinedGroup: async (id: string): Promise<CombinedGroup> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/combined/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}`;

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
                    console.error(`Get combined group error: ${response.status}`, errorText);
                    throw new Error(`Get combined group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendCombinedGroup>;
                return mapCombinedGroupResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendCombinedGroup>>(url);
                return mapCombinedGroupResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get combined group error:", error);
            throw error;
        }
    },

    getCombinedGroupGroups: async (id: string): Promise<ActiveGroup[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/combined/${id}/groups`
            : `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}/groups`;

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
                    console.error(`Get combined group groups error: ${response.status}`, errorText);
                    throw new Error(`Get combined group groups failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActiveGroup[]>;
                return responseData.data.map(mapActiveGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendActiveGroup[]>>(url);
                return response.data.data.map(mapActiveGroupResponse);
            }
        } catch (error) {
            console.error("Get combined group groups error:", error);
            throw error;
        }
    },

    createCombinedGroup: async (combinedGroup: Omit<CombinedGroup, "id" | "isActive" | "createdAt" | "updatedAt">): Promise<CombinedGroup> => {
        const backendData = prepareCombinedGroupForBackend(combinedGroup);

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/active/combined"
            : `${env.NEXT_PUBLIC_API_URL}/active/combined`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Create combined group error: ${response.status}`, errorText);
                    throw new Error(`Create combined group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendCombinedGroup>;
                return mapCombinedGroupResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendCombinedGroup>>(url, backendData);
                return mapCombinedGroupResponse(response.data.data);
            }
        } catch (error) {
            console.error("Create combined group error:", error);
            throw error;
        }
    },

    updateCombinedGroup: async (id: string, combinedGroup: Partial<CombinedGroup>): Promise<CombinedGroup> => {
        const backendData = prepareCombinedGroupForBackend(combinedGroup);

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/combined/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Update combined group error: ${response.status}`, errorText);
                    throw new Error(`Update combined group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendCombinedGroup>;
                return mapCombinedGroupResponse(responseData.data);
            } else {
                const response = await api.put<ApiResponse<BackendCombinedGroup>>(url, backendData);
                return mapCombinedGroupResponse(response.data.data);
            }
        } catch (error) {
            console.error("Update combined group error:", error);
            throw error;
        }
    },

    deleteCombinedGroup: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/combined/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Delete combined group error: ${response.status}`, errorText);
                    throw new Error(`Delete combined group failed: ${response.status}`);
                }
            } else {
                await api.delete(url);
            }
        } catch (error) {
            console.error("Delete combined group error:", error);
            throw error;
        }
    },

    endCombinedGroup: async (id: string): Promise<CombinedGroup> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/combined/${id}/end`
            : `${env.NEXT_PUBLIC_API_URL}/active/combined/${id}/end`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`End combined group error: ${response.status}`, errorText);
                    throw new Error(`End combined group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendCombinedGroup>;
                return mapCombinedGroupResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendCombinedGroup>>(url);
                return mapCombinedGroupResponse(response.data.data);
            }
        } catch (error) {
            console.error("End combined group error:", error);
            throw error;
        }
    },

    // Group Mappings
    getGroupMappingsByGroup: async (groupId: string): Promise<GroupMapping[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/mappings/group/${groupId}`
            : `${env.NEXT_PUBLIC_API_URL}/active/mappings/group/${groupId}`;

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
                    console.error(`Get group mappings by group error: ${response.status}`, errorText);
                    throw new Error(`Get group mappings by group failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendGroupMapping[]>;
                return responseData.data.map(mapGroupMappingResponse);
            } else {
                const response = await api.get<ApiResponse<BackendGroupMapping[]>>(url);
                return response.data.data.map(mapGroupMappingResponse);
            }
        } catch (error) {
            console.error("Get group mappings by group error:", error);
            throw error;
        }
    },

    getGroupMappingsByCombined: async (combinedId: string): Promise<GroupMapping[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/mappings/combined/${combinedId}`
            : `${env.NEXT_PUBLIC_API_URL}/active/mappings/combined/${combinedId}`;

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
                    console.error(`Get group mappings by combined error: ${response.status}`, errorText);
                    throw new Error(`Get group mappings by combined failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendGroupMapping[]>;
                return responseData.data.map(mapGroupMappingResponse);
            } else {
                const response = await api.get<ApiResponse<BackendGroupMapping[]>>(url);
                return response.data.data.map(mapGroupMappingResponse);
            }
        } catch (error) {
            console.error("Get group mappings by combined error:", error);
            throw error;
        }
    },

    addGroupToCombination: async (activeGroupId: string, combinedGroupId: string): Promise<GroupMapping> => {
        const backendData = prepareGroupMappingForBackend({ activeGroupId, combinedGroupId });

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/active/mappings/add"
            : `${env.NEXT_PUBLIC_API_URL}/active/mappings/add`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Add group to combination error: ${response.status}`, errorText);
                    throw new Error(`Add group to combination failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendGroupMapping>;
                return mapGroupMappingResponse(responseData.data);
            } else {
                const response = await api.post<ApiResponse<BackendGroupMapping>>(url, backendData);
                return mapGroupMappingResponse(response.data.data);
            }
        } catch (error) {
            console.error("Add group to combination error:", error);
            throw error;
        }
    },

    removeGroupFromCombination: async (activeGroupId: string, combinedGroupId: string): Promise<void> => {
        const backendData = prepareGroupMappingForBackend({ activeGroupId, combinedGroupId });

        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/active/mappings/remove"
            : `${env.NEXT_PUBLIC_API_URL}/active/mappings/remove`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify(backendData),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Remove group from combination error: ${response.status}`, errorText);
                    throw new Error(`Remove group from combination failed: ${response.status}`);
                }
            } else {
                await api.post(url, backendData);
            }
        } catch (error) {
            console.error("Remove group from combination error:", error);
            throw error;
        }
    },

    // Analytics
    getAnalyticsCounts: async (): Promise<Analytics> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/active/analytics/counts"
            : `${env.NEXT_PUBLIC_API_URL}/active/analytics/counts`;

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
                    console.error(`Get analytics counts error: ${response.status}`, errorText);
                    throw new Error(`Get analytics counts failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendAnalytics>;
                return mapAnalyticsResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendAnalytics>>(url);
                return mapAnalyticsResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get analytics counts error:", error);
            throw error;
        }
    },

    getRoomUtilization: async (roomId: string): Promise<Analytics> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/analytics/room/${roomId}/utilization`
            : `${env.NEXT_PUBLIC_API_URL}/active/analytics/room/${roomId}/utilization`;

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
                    console.error(`Get room utilization error: ${response.status}`, errorText);
                    throw new Error(`Get room utilization failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendAnalytics>;
                return mapAnalyticsResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendAnalytics>>(url);
                return mapAnalyticsResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get room utilization error:", error);
            throw error;
        }
    },

    getStudentAttendance: async (studentId: string): Promise<Analytics> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/analytics/student/${studentId}/attendance`
            : `${env.NEXT_PUBLIC_API_URL}/active/analytics/student/${studentId}/attendance`;

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
                    console.error(`Get student attendance error: ${response.status}`, errorText);
                    throw new Error(`Get student attendance failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendAnalytics>;
                return mapAnalyticsResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendAnalytics>>(url);
                return mapAnalyticsResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get student attendance error:", error);
            throw error;
        }
    },

    // Unclaimed Groups (Deviceless Claiming)
    getUnclaimedGroups: async (): Promise<ActiveGroup[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/active/groups/unclaimed"
            : `${env.NEXT_PUBLIC_API_URL}/active/groups/unclaimed`;

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
                    console.error(`Get unclaimed groups error: ${response.status}`, errorText);
                    throw new Error(`Get unclaimed groups failed: ${response.status}`);
                }

                const responseData = await response.json() as unknown;

                // Safely extract array from various response shapes
                let rawData: BackendActiveGroup[] = [];

                if (Array.isArray(responseData)) {
                    // Backend returned raw array
                    rawData = responseData as BackendActiveGroup[];
                } else if (responseData && typeof responseData === 'object') {
                    const dataObj = responseData as Record<string, unknown>;

                    if (Array.isArray(dataObj.data)) {
                        // Standard { data: [...] } response
                        rawData = dataObj.data as BackendActiveGroup[];
                    } else if (dataObj.data && typeof dataObj.data === 'object') {
                        const nestedData = dataObj.data as Record<string, unknown>;
                        if (Array.isArray(nestedData.items)) {
                            // Paginated { data: { items: [...] } } response
                            rawData = nestedData.items as BackendActiveGroup[];
                        } else {
                            // Unexpected nested object shape
                            console.warn("[active-service] Unexpected unclaimed groups response shape:", responseData);
                        }
                    } else if (dataObj.data !== undefined && dataObj.data !== null) {
                        // data field exists but is neither array nor object with items
                        console.warn("[active-service] Unexpected unclaimed groups response shape:", responseData);
                    }
                }

                return rawData.map(mapActiveGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendActiveGroup[]>>(url);
                return response.data.data.map(mapActiveGroupResponse);
            }
        } catch (error) {
            console.error("Get unclaimed groups error:", error);
            throw error;
        }
    },

    claimActiveGroup: async (groupId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/active/groups/${groupId}/claim`
            : `${env.NEXT_PUBLIC_API_URL}/active/groups/${groupId}/claim`;

        try {
            if (useProxyApi) {
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    headers: {
                        Authorization: `Bearer ${session?.user?.token}`,
                        "Content-Type": "application/json",
                    },
                    body: JSON.stringify({ role: "supervisor" }),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`Claim group error: ${response.status}`, errorText);
                    throw new Error(`Claim group failed: ${response.status}`);
                }
            } else {
                await api.post(url, { role: "supervisor" });
            }
        } catch (error) {
            console.error("Claim group error:", error);
            throw error;
        }
    },
};