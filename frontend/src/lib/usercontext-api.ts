// lib/usercontext-api.ts
import { getSession } from "next-auth/react";
import { env } from "~/env";
import api from "./api";
import {
    mapEducationalGroupResponse,
    mapActivityGroupResponse,
    mapActiveGroupResponse,
    mapUserProfileResponse,
    mapStaffResponse,
    mapTeacherResponse,
    type EducationalGroup,
    type ActivityGroup,
    type ActiveGroup,
    type UserProfile,
    type Staff,
    type Teacher,
    type BackendEducationalGroup,
    type BackendActivityGroup,
    type BackendActiveGroup,
    type BackendUserProfile,
    type BackendStaff,
    type BackendTeacher,
} from "./usercontext-helpers";

// Generic API response interface
interface ApiResponse<T> {
    success: boolean;
    message: string;
    data: T;
}

export const userContextService = {
    // Get current user profile
    getCurrentUser: async (): Promise<UserProfile> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/me"
            : `${env.NEXT_PUBLIC_API_URL}/me`;

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
                    console.error(`Get current user error: ${response.status}`, errorText);
                    throw new Error(`Get current user failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendUserProfile>;
                return mapUserProfileResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendUserProfile>>(url);
                return mapUserProfileResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get current user error:", error);
            throw error;
        }
    },

    // Get current user's staff profile
    getCurrentStaff: async (): Promise<Staff> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/me/staff"
            : `${env.NEXT_PUBLIC_API_URL}/me/staff`;

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
                    console.error(`Get current staff error: ${response.status}`, errorText);
                    throw new Error(`Get current staff failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendStaff>;
                return mapStaffResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendStaff>>(url);
                return mapStaffResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get current staff error:", error);
            throw error;
        }
    },

    // Get current user's teacher profile
    getCurrentTeacher: async (): Promise<Teacher> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/me/teacher"
            : `${env.NEXT_PUBLIC_API_URL}/me/teacher`;

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
                    console.error(`Get current teacher error: ${response.status}`, errorText);
                    throw new Error(`Get current teacher failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendTeacher>;
                return mapTeacherResponse(responseData.data);
            } else {
                const response = await api.get<ApiResponse<BackendTeacher>>(url);
                return mapTeacherResponse(response.data.data);
            }
        } catch (error) {
            console.error("Get current teacher error:", error);
            throw error;
        }
    },

    // Get educational groups for current user
    getMyEducationalGroups: async (): Promise<EducationalGroup[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/me/groups"
            : `${env.NEXT_PUBLIC_API_URL}/me/groups`;

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
                    console.error(`Get educational groups error: ${response.status}`, errorText);
                    throw new Error(`Get educational groups failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendEducationalGroup[]>;
                return responseData.data.map(mapEducationalGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendEducationalGroup[]>>(url);
                return response.data.data.map(mapEducationalGroupResponse);
            }
        } catch (error) {
            console.error("Get educational groups error:", error);
            throw error;
        }
    },

    // Get activity groups for current user
    getMyActivityGroups: async (): Promise<ActivityGroup[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/me/groups/activity"
            : `${env.NEXT_PUBLIC_API_URL}/me/groups/activity`;

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
                    console.error(`Get activity groups error: ${response.status}`, errorText);
                    throw new Error(`Get activity groups failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActivityGroup[]>;
                return responseData.data.map(mapActivityGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendActivityGroup[]>>(url);
                return response.data.data.map(mapActivityGroupResponse);
            }
        } catch (error) {
            console.error("Get activity groups error:", error);
            throw error;
        }
    },

    // Get active groups for current user
    getMyActiveGroups: async (): Promise<ActiveGroup[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/me/groups/active"
            : `${env.NEXT_PUBLIC_API_URL}/me/groups/active`;

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
                const response = await api.get<ApiResponse<BackendActiveGroup[]>>(url);
                return response.data.data.map(mapActiveGroupResponse);
            }
        } catch (error) {
            console.error("Get active groups error:", error);
            throw error;
        }
    },

    // Get supervised groups for current user
    getMySupervisedGroups: async (): Promise<ActiveGroup[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/me/groups/supervised"
            : `${env.NEXT_PUBLIC_API_URL}/me/groups/supervised`;

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
                    console.error(`Get supervised groups error: ${response.status}`, errorText);
                    throw new Error(`Get supervised groups failed: ${response.status}`);
                }

                const responseData = await response.json() as ApiResponse<BackendActiveGroup[]>;
                return responseData.data.map(mapActiveGroupResponse);
            } else {
                const response = await api.get<ApiResponse<BackendActiveGroup[]>>(url);
                return response.data.data.map(mapActiveGroupResponse);
            }
        } catch (error) {
            console.error("Get supervised groups error:", error);
            throw error;
        }
    },

    // Check if user has any educational groups (convenience method)
    hasEducationalGroups: async (): Promise<boolean> => {
        try {
            const groups = await userContextService.getMyEducationalGroups();
            return groups.length > 0;
        } catch (error) {
            console.error("Error checking for educational groups:", error);
            return false;
        }
    },
};