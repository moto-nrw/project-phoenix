// lib/activity-api.ts
import { getSession } from "next-auth/react";
import { env } from "~/env";
import api from "./api";
import {
    mapActivityResponse,
    mapActivityCategoryResponse,
    mapActivityTimeResponse,
    mapActivityStudentResponse,
    prepareActivityForBackend,
    prepareActivityTimeForBackend,
    type Activity,
    type ActivityCategory,
    type ActivityTime,
    type ActivityStudent,
    type BackendActivity,
    type BackendActivityCategory,
    type BackendActivityTime,
    type BackendActivityStudent
} from "./activity-helpers";

// Generic API response interface
interface ApiResponse<T> {
    data: T;
    message?: string;
    status?: string;
}

export type { 
    Activity,
    ActivityCategory,
    ActivityTime,
    ActivityStudent,
    BackendActivity,
    BackendActivityCategory,
    BackendActivityTime,
    BackendActivityStudent
};

export const activityService = {
    // Get all activities
    getActivities: async (filters?: { 
        search?: string; 
        categoryId?: string;
        isOpenAgs?: boolean;
    }): Promise<Activity[]> => {
        const params = new URLSearchParams();
        if (filters?.search) params.append("search", filters.search);
        if (filters?.categoryId) params.append("category_id", filters.categoryId);
        if (filters?.isOpenAgs !== undefined) params.append("is_open_ags", filters.isOpenAgs.toString());

        const useProxyApi = typeof window !== "undefined";
        let url = useProxyApi
            ? "/api/activities"
            : `${env.NEXT_PUBLIC_API_URL}/activities`;

        const queryString = params.toString();
        if (queryString) {
            url += `?${queryString}`;
        }

        try {
            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "GET",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                const data = await response.json() as ApiResponse<BackendActivity[]>;
                return Array.isArray(data.data)
                    ? data.data.map(mapActivityResponse)
                    : [];
            } else {
                // Server-side: use axios with the API URL directly
                const response = await api.get<ApiResponse<BackendActivity[]>>(url);
                return Array.isArray(response.data.data)
                    ? response.data.data.map(mapActivityResponse)
                    : [];
            }
        } catch (error) {
            console.error("Error fetching activities:", error);
            return [];
        }
    },

    // Get a single activity by ID
    getActivity: async (id: string): Promise<Activity> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/activities/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;

        try {
            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "GET",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                const data = await response.json() as ApiResponse<BackendActivity>;
                return mapActivityResponse(data.data);
            } else {
                // Server-side: use axios with the API URL directly
                const response = await api.get<ApiResponse<BackendActivity>>(url);
                return mapActivityResponse(response.data.data);
            }
        } catch (error) {
            console.error(`Error fetching activity ${id}:`, error);
            throw error;
        }
    },

    // Create a new activity
    createActivity: async (activity: Partial<Activity>): Promise<Activity> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/activities"
            : `${env.NEXT_PUBLIC_API_URL}/activities`;

        try {
            const payload = prepareActivityForBackend(activity);

            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                    body: JSON.stringify(payload),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                const data = await response.json() as ApiResponse<BackendActivity>;
                return mapActivityResponse(data.data);
            } else {
                // Server-side: use axios with the API URL directly
                const response = await api.post<ApiResponse<BackendActivity>>(url, payload);
                return mapActivityResponse(response.data.data);
            }
        } catch (error) {
            console.error("Error creating activity:", error);
            throw error;
        }
    },

    // Update an existing activity
    updateActivity: async (id: string, activity: Partial<Activity>): Promise<Activity> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/activities/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;

        try {
            const payload = prepareActivityForBackend(activity);

            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "PUT",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                    body: JSON.stringify(payload),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                const data = await response.json() as ApiResponse<BackendActivity>;
                return mapActivityResponse(data.data);
            } else {
                // Server-side: use axios with the API URL directly
                const response = await api.put<ApiResponse<BackendActivity>>(url, payload);
                return mapActivityResponse(response.data.data);
            }
        } catch (error) {
            console.error(`Error updating activity ${id}:`, error);
            throw error;
        }
    },

    // Delete an activity
    deleteActivity: async (id: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/activities/${id}`
            : `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;

        try {
            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                return;
            } else {
                // Server-side: use axios with the API URL directly
                await api.delete(url);
                return;
            }
        } catch (error) {
            console.error(`Error deleting activity ${id}:`, error);
            throw error;
        }
    },

    // Get all activity categories
    getCategories: async (): Promise<ActivityCategory[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? "/api/activities/categories"
            : `${env.NEXT_PUBLIC_API_URL}/activities/categories`;

        try {
            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "GET",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                const data = await response.json() as ApiResponse<BackendActivityCategory[]>;
                return Array.isArray(data.data)
                    ? data.data.map(mapActivityCategoryResponse)
                    : [];
            } else {
                // Server-side: use axios with the API URL directly
                const response = await api.get<ApiResponse<BackendActivityCategory[]>>(url);
                return Array.isArray(response.data.data)
                    ? response.data.data.map(mapActivityCategoryResponse)
                    : [];
            }
        } catch (error) {
            console.error("Error fetching activity categories:", error);
            return [];
        }
    },

    // Add an activity time slot
    addTimeSlot: async (
        activityId: string, 
        timeSlot: Partial<ActivityTime>
    ): Promise<ActivityTime> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/activities/${activityId}/times`
            : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/times`;

        try {
            const payload = prepareActivityTimeForBackend(timeSlot);

            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                    body: JSON.stringify(payload),
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                const data = await response.json() as ApiResponse<BackendActivityTime>;
                return mapActivityTimeResponse(data.data);
            } else {
                // Server-side: use axios with the API URL directly
                const response = await api.post<ApiResponse<BackendActivityTime>>(url, payload);
                return mapActivityTimeResponse(response.data.data);
            }
        } catch (error) {
            console.error(`Error adding time slot to activity ${activityId}:`, error);
            throw error;
        }
    },

    // Delete an activity time slot
    deleteTimeSlot: async (activityId: string, timeSlotId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/activities/${activityId}/times/${timeSlotId}`
            : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/times/${timeSlotId}`;

        try {
            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                return;
            } else {
                // Server-side: use axios with the API URL directly
                await api.delete(url);
                return;
            }
        } catch (error) {
            console.error(`Error deleting time slot ${timeSlotId} from activity ${activityId}:`, error);
            throw error;
        }
    },

    // Enroll a student in an activity
    enrollStudent: async (activityId: string, studentId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/activities/${activityId}/students/${studentId}`
            : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/students/${studentId}`;

        try {
            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "POST",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                return;
            } else {
                // Server-side: use axios with the API URL directly
                await api.post(url);
                return;
            }
        } catch (error) {
            console.error(`Error enrolling student ${studentId} in activity ${activityId}:`, error);
            throw error;
        }
    },

    // Unenroll a student from an activity
    unenrollStudent: async (activityId: string, studentId: string): Promise<void> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/activities/${activityId}/students/${studentId}`
            : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/students/${studentId}`;

        try {
            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "DELETE",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                return;
            } else {
                // Server-side: use axios with the API URL directly
                await api.delete(url);
                return;
            }
        } catch (error) {
            console.error(`Error unenrolling student ${studentId} from activity ${activityId}:`, error);
            throw error;
        }
    },

    // Get enrolled students for an activity
    getEnrolledStudents: async (activityId: string): Promise<ActivityStudent[]> => {
        const useProxyApi = typeof window !== "undefined";
        const url = useProxyApi
            ? `/api/activities/${activityId}/students`
            : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/students`;

        try {
            if (useProxyApi) {
                // Browser environment: use fetch with our Next.js API route
                const session = await getSession();
                const response = await fetch(url, {
                    method: "GET",
                    credentials: "include",
                    headers: session?.user?.token
                        ? {
                            Authorization: `Bearer ${session.user.token}`,
                            "Content-Type": "application/json",
                        }
                        : undefined,
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    console.error(`API error: ${response.status}`, errorText);
                    throw new Error(`API error: ${response.status}`);
                }

                const data = await response.json() as ApiResponse<BackendActivityStudent[]>;
                return Array.isArray(data.data)
                    ? data.data.map(mapActivityStudentResponse)
                    : [];
            } else {
                // Server-side: use axios with the API URL directly
                const response = await api.get<ApiResponse<BackendActivityStudent[]>>(url);
                return Array.isArray(response.data.data)
                    ? response.data.data.map(mapActivityStudentResponse)
                    : [];
            }
        } catch (error) {
            console.error(`Error fetching enrolled students for activity ${activityId}:`, error);
            return [];
        }
    }
};

export default activityService;