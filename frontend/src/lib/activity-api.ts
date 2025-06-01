// lib/activity-api.ts
import { getSession } from "next-auth/react";
import { env } from "~/env";
import api from "./api";
import { handleAuthFailure } from "./auth-api";

// Standardized error handling function for activities API
function handleActivityApiError(error: unknown, context: string): never {
  
  // If we have a structured error message with status code
  if (error instanceof Error) {
    const regex = /API error \((\d+)\):/;
    const match = regex.exec(error.message);
    if (match?.[1]) {
      const status = parseInt(match[1], 10);
      const errorMessage = `Failed to ${context}: ${error.message}`;
      throw new Error(JSON.stringify({
        status,
        message: errorMessage,
        code: `ACTIVITY_API_ERROR_${status}`
      }));
    }
  }
  
  // Default error response
  throw new Error(JSON.stringify({
    status: 500,
    message: `Failed to ${context}: ${error instanceof Error ? error.message : "Unknown error"}`,
    code: "ACTIVITY_API_ERROR_UNKNOWN"
  }));
}
import {
    mapActivityResponse,
    mapActivityCategoryResponse,
    mapSupervisorResponse,
    mapActivityStudentResponse,
    mapActivityScheduleResponse,
    mapTimeframeResponse,
    prepareActivityForBackend,
    prepareActivityScheduleForBackend,
    type Activity,
    type ActivityCategory,
    type CreateActivityRequest,
    type UpdateActivityRequest,
    type ActivityFilter,
    type BackendActivity,
    type BackendActivityCategory,
    type BackendSupervisor,
    type BackendActivitySupervisor,
    type Supervisor,
    type ActivityStudent,
    type BackendActivityStudent,
    type ActivitySchedule,
    type BackendActivitySchedule,
    type Timeframe,
    type BackendTimeframe
} from "./activity-helpers";

// Generic API response interface
interface ApiResponse<T> {
    data: T;
    message?: string;
    status?: string;
}

// Available time slot type
interface AvailableTimeSlot {
    weekday: string;
    timeframe_id?: string;
}

export type { 
    Activity,
    ActivityCategory,
    Supervisor,
    BackendActivity,
    BackendActivityCategory,
    BackendSupervisor,
    BackendActivitySupervisor,
    ActivityStudent,
    BackendActivityStudent,
    ActivitySchedule,
    BackendActivitySchedule,
    Timeframe,
    BackendTimeframe
};

// Get all activities
export async function fetchActivities(filters?: ActivityFilter): Promise<Activity[]> {
    const params = new URLSearchParams();
    if (filters?.search) params.append("search", filters.search);
    if (filters?.category_id) params.append("category_id", filters.category_id);
    if (filters?.is_open_ags !== undefined) params.append("is_open_ags", filters.is_open_ags.toString());

    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi
        ? "/api/activities"
        : `${env.NEXT_PUBLIC_API_URL}/api/activities`;

    const queryString = params.toString();
    if (queryString) {
        url += `?${queryString}`;
    }

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
            throw new Error(`API error: ${response.status}`);
        }

        const responseData = await response.json() as ApiResponse<Activity[]> | Activity[];
        
        // Extract the array from the response wrapper if needed
        if (responseData && typeof responseData === 'object' && 'data' in responseData) {
            return responseData.data;
        }
        return responseData;
    } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get<ApiResponse<BackendActivity[]>>(url);
        return Array.isArray(response.data.data)
            ? response.data.data.map(mapActivityResponse)
            : [];
    }
}

// Fetch a single activity by ID (wrapper for consistency with other fetch functions)
export async function fetchActivity(id: string): Promise<Activity> {
    return getActivity(id);
}

// Get a single activity by ID
export async function getActivity(id: string): Promise<Activity> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${id}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${id}`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<Activity> | Activity;
            
            // Extract the data from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return responseData.data;
            }
            return responseData;
        } else {
            const response = await api.get<ApiResponse<BackendActivity>>(url);
            return mapActivityResponse(response.data.data);
        }
    } catch (error) {
        throw error;
    }
}

// Get enrolled students for an activity
export async function getEnrolledStudents(activityId: string): Promise<ActivityStudent[]> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/students`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/students`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<BackendActivityStudent[]> | BackendActivityStudent[];
            
            // Extract the array from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return Array.isArray(responseData.data)
                    ? responseData.data.map(mapActivityStudentResponse)
                    : [];
            }
            return Array.isArray(responseData)
                ? responseData.map(mapActivityStudentResponse)
                : [];
        } else {
            const response = await api.get<ApiResponse<BackendActivityStudent[]>>(url);
            return Array.isArray(response.data.data)
                ? response.data.data.map(mapActivityStudentResponse)
                : [];
        }
    } catch (error) {
        handleActivityApiError(error, "fetch enrolled students");
    }
}

// Enroll a student in an activity
export async function enrollStudent(activityId: string, studentData: { studentId: string }): Promise<{ success: boolean }> {
    const useProxyApi = typeof window !== "undefined";
    // Update URL to match backend endpoint structure which expects the studentId in the URL path
    const url = useProxyApi
        ? `/api/activities/${activityId}/enroll/${studentData.studentId}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/enroll/${studentData.studentId}`;
    
    // No request body needed since backend extracts IDs from URL path

    try {
        if (useProxyApi) {
            const session = await getSession();
            const response = await fetch(url, {
                method: "POST",
                credentials: "include",
                headers: {
                    "Content-Type": "application/json",
                    ...(session?.user?.token && {
                        Authorization: `Bearer ${session.user.token}`,
                    }),
                },
                // No body needed
            });

            if (!response.ok) {
                throw new Error(`API error: ${response.status}`);
            }

            return { success: true };
        } else {
            // Send empty object as body
            await api.post(url, {});
            return { success: true };
        }
    } catch (error) {
        throw error;
    }
}

// Unenroll a student from an activity
export async function unenrollStudent(activityId: string, studentId: string): Promise<void> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/students/${studentId}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/students/${studentId}`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }
        } else {
            await api.delete(url);
        }
    } catch (error) {
        throw error;
    }
}

// Create a new activity
export async function createActivity(data: CreateActivityRequest): Promise<Activity> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? "/api/activities"
        : `${env.NEXT_PUBLIC_API_URL}/api/activities`;

    // No need to prepare for backend - data already in correct format
    
    // Create a safe default activity to return in case of issues
    const safeActivity: Activity = {
        id: '0',
        name: data.name || '',
        max_participant: data.max_participants || 0,
        is_open_ags: false,
        supervisor_id: data.supervisor_ids?.[0] ? String(data.supervisor_ids[0]) : '',
        ag_category_id: String(data.category_id || ''),
        created_at: new Date(),
        updated_at: new Date(),
        participant_count: 0,
        times: [],
        students: []
    };

    try {
        if (useProxyApi) {
            const session = await getSession();
            const response = await fetch(url, {
                method: "POST",
                credentials: "include",
                headers: {
                    "Content-Type": "application/json",
                    ...(session?.user?.token && {
                        Authorization: `Bearer ${session.user.token}`,
                    }),
                },
                body: JSON.stringify(data),
            });

            if (!response.ok) {
                throw new Error(`API error: ${response.status}`);
            }

            try {
                const responseData = await response.json() as unknown;
                
                // Try to extract data regardless of format
                if (responseData) {
                    // Handle wrapped response { status/success: "success", data: Activity }
                    if (typeof responseData === 'object' && responseData !== null) {
                        if ('data' in responseData && responseData.data) {
                            // Try to extract ID and update safeActivity if possible
                            if (typeof responseData.data === 'object' && responseData.data !== null && 'id' in responseData.data) {
                                safeActivity.id = String(responseData.data.id);
                                
                                // If it's a full BackendActivity, map it
                                if ('name' in responseData.data && 
                                    'max_participants' in responseData.data && 
                                    'category_id' in responseData.data) {
                                    return mapActivityResponse(responseData.data as BackendActivity);
                                }
                            }
                            return responseData.data as Activity;
                        }
                        // Handle direct response with ID
                        else if ('id' in responseData) {
                            safeActivity.id = String(responseData.id);
                            
                            // If it's a full BackendActivity, map it
                            if ('name' in responseData && 
                                'max_participants' in responseData && 
                                'category_id' in responseData) {
                                return mapActivityResponse(responseData as BackendActivity);
                            }
                        }
                    }
                }
                
                // If we got a response but couldn't extract meaningful data, return safe activity
                return safeActivity;
            } catch {
                // Even if parsing fails, we know the POST was successful, so return safe activity
                return safeActivity;
            }
        } else {
            try {
                const response = await api.post<ApiResponse<BackendActivity>>(
                    url,
                    data // Send the CreateActivityRequest directly
                );
                
                // Try to handle various response formats safely
                if (response && typeof response === 'object') {
                    if ('data' in response && response.data) {
                        if (typeof response.data === 'object') {
                            // Check if it's a wrapped response with data property
                            if ('data' in response.data && typeof response.data.data === 'object') {
                                if ('id' in response.data.data) {
                                    safeActivity.id = String(response.data.data.id);
                                    
                                    // Full backend activity format
                                    if ('name' in response.data.data && 
                                        'max_participants' in response.data.data && 
                                        'category_id' in response.data.data) {
                                        return mapActivityResponse(response.data.data);
                                    }
                                }
                            } 
                            // Direct backend activity in data
                            else if ('id' in response.data) {
                                safeActivity.id = String(response.data.id);
                                
                                // Full backend activity format
                                if ('name' in response.data && 
                                    'max_participants' in response.data && 
                                    'category_id' in response.data) {
                                    return mapActivityResponse(response.data as unknown as BackendActivity);
                                }
                            }
                        }
                    }
                }
                
                // Fallback to safe activity if we couldn't extract proper data
                return safeActivity;
            } catch (apiError) {
                throw apiError;
            }
        }
    } catch (error) {
        throw error;
    }
}

// Update an activity
export async function updateActivity(id: string, data: UpdateActivityRequest): Promise<Activity> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${id}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${id}`;

    // Convert UpdateActivityRequest to a format compatible with prepareActivityForBackend
    const activityData: Partial<Activity> = {
        name: data.name,
        max_participant: data.max_participants,
        is_open_ags: data.is_open,
        ag_category_id: String(data.category_id),
        supervisor_id: data.supervisor_ids && data.supervisor_ids.length > 0 ? String(data.supervisor_ids[0]) : undefined
    };
    
    const backendData = prepareActivityForBackend(activityData);

    try {
        if (useProxyApi) {
            const session = await getSession();
            const response = await fetch(url, {
                method: "PUT",
                credentials: "include",
                headers: {
                    "Content-Type": "application/json",
                    ...(session?.user?.token && {
                        Authorization: `Bearer ${session.user.token}`,
                    }),
                },
                body: JSON.stringify(data),
            });

            if (!response.ok) {
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<Activity> | Activity;
            
            // Extract the data from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return responseData.data;
            }
            return responseData;
        } else {
            const response = await api.put<ApiResponse<BackendActivity>>(
                url,
                backendData
            );
            return mapActivityResponse(response.data.data);
        }
    } catch (error) {
        throw error;
    }
}

// Delete an activity
export async function deleteActivity(id: string): Promise<void> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${id}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${id}`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }
        } else {
            await api.delete(url);
        }
    } catch (error) {
        throw error;
    }
}

// Get all categories
export async function getCategories(): Promise<ActivityCategory[]> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? "/api/activities/categories"
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/categories`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<ActivityCategory[]> | ActivityCategory[];
            
            // Extract the array from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return responseData.data;
            }
            return responseData;
        } else {
            const response = await api.get<ApiResponse<BackendActivityCategory[]>>(url);
            return Array.isArray(response.data.data)
                ? response.data.data.map(mapActivityCategoryResponse)
                : [];
        }
    } catch (error) {
        handleActivityApiError(error, "fetch categories");
    }
}

// Get all supervisors
export async function getSupervisors(): Promise<Array<{ id: string; name: string }>> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? "/api/activities/supervisors"
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/supervisors`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<Array<{ id: string; name: string }>> | Array<{ id: string; name: string }>;
            
            // Extract the array from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return responseData.data;
            }
            return responseData;
        } else {
            const response = await api.get<ApiResponse<BackendSupervisor[]>>(url);
            return Array.isArray(response.data.data)
                ? response.data.data.map(mapSupervisorResponse)
                : [];
        }
    } catch {
        return [];
    }
}

// Get schedules for an activity
export async function getActivitySchedules(activityId: string): Promise<ActivitySchedule[]> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/schedules`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<BackendActivitySchedule[]> | BackendActivitySchedule[];
            
            // Extract the array from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return Array.isArray(responseData.data)
                    ? responseData.data.map(mapActivityScheduleResponse)
                    : [];
            }
            return Array.isArray(responseData)
                ? responseData.map(mapActivityScheduleResponse)
                : [];
        } else {
            const response = await api.get<ApiResponse<BackendActivitySchedule[]>>(url);
            return Array.isArray(response.data.data)
                ? response.data.data.map(mapActivityScheduleResponse)
                : [];
        }
    } catch {
        return [];
    }
}

// Get a single schedule for an activity
export async function getActivitySchedule(activityId: string, scheduleId: string): Promise<ActivitySchedule | null> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/schedules/${scheduleId}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules/${scheduleId}`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<BackendActivitySchedule> | BackendActivitySchedule;
            
            // Extract the data from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return mapActivityScheduleResponse(responseData.data);
            }
            return mapActivityScheduleResponse(responseData);
        } else {
            const response = await api.get<ApiResponse<BackendActivitySchedule>>(url);
            return mapActivityScheduleResponse(response.data.data);
        }
    } catch {
        return null;
    }
}

// Get all available timeframes
export async function getTimeframes(): Promise<Timeframe[]> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? "/api/schedules/timeframes"
        : `${env.NEXT_PUBLIC_API_URL}/api/schedules/timeframes`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as unknown;
            
            // Handle different response structures
            if (responseData && typeof responseData === 'object' && responseData !== null) {
                // If it's a wrapped response with data property
                if ('data' in responseData && responseData.data) {
                    if (Array.isArray(responseData.data)) {
                        // Check if it's already frontend types or needs mapping
                        if (responseData.data.length > 0 && responseData.data[0] && 
                            typeof responseData.data[0] === 'object' && responseData.data[0] !== null && 
                            'id' in responseData.data[0] && typeof (responseData.data[0] as { id: unknown }).id === 'string') {
                            return responseData.data as Timeframe[];
                        }
                        return (responseData.data as BackendTimeframe[]).map(mapTimeframeResponse);
                    }
                    return [];
                }
                // If it's an array directly
                else if (Array.isArray(responseData)) {
                    if (responseData.length > 0 && responseData[0] && 
                        typeof responseData[0] === 'object' && responseData[0] !== null &&
                        'id' in responseData[0]) {
                        // Check if it's already frontend types or needs mapping
                        if ('id' in responseData[0] && typeof (responseData[0] as { id: unknown }).id === 'string') {
                            return responseData as Timeframe[];
                        }
                        return (responseData as BackendTimeframe[]).map(mapTimeframeResponse);
                    }
                    return [];
                }
            }
            return [];
        } else {
            const response = await api.get<ApiResponse<BackendTimeframe[]>>(url);
            if (response?.data && Array.isArray(response.data.data)) {
                return response.data.data.map(mapTimeframeResponse);
            }
            return [];
        }
    } catch (error) {
        handleActivityApiError(error, "fetch timeframes");
    }
}

// Get available time slots
export async function getAvailableTimeSlots(activityId: string, date?: string): Promise<AvailableTimeSlot[]> {
    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi
        ? `/api/activities/${activityId}/schedules/available`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules/available`;
    
    // Add date parameter if provided
    if (date) {
        url += `?date=${encodeURIComponent(date)}`;
    }

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<AvailableTimeSlot[]> | AvailableTimeSlot[];
            
            // Extract the array from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return responseData.data || [];
            }
            return responseData || [];
        } else {
            const response = await api.get<ApiResponse<AvailableTimeSlot[]>>(url);
            return response.data.data || [];
        }
    } catch {
        return [];
    }
}

// Create a new schedule for an activity
export async function createActivitySchedule(activityId: string, scheduleData: Partial<ActivitySchedule>): Promise<ActivitySchedule> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/schedules`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules`;

    // Prepare backend data
    const backendData = prepareActivityScheduleForBackend(scheduleData);

    try {
        if (useProxyApi) {
            const session = await getSession();
            const response = await fetch(url, {
                method: "POST",
                credentials: "include",
                headers: {
                    "Content-Type": "application/json",
                    ...(session?.user?.token && {
                        Authorization: `Bearer ${session.user.token}`,
                    }),
                },
                body: JSON.stringify(backendData),
            });

            if (!response.ok) {
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<BackendActivitySchedule> | BackendActivitySchedule;
            
            // Extract the data from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return mapActivityScheduleResponse(responseData.data);
            }
            return mapActivityScheduleResponse(responseData);
        } else {
            const response = await api.post<ApiResponse<BackendActivitySchedule>>(url, backendData);
            return mapActivityScheduleResponse(response.data.data);
        }
    } catch (error) {
        handleActivityApiError(error, "create activity schedule");
    }
}

// Update a schedule for an activity
export async function updateActivitySchedule(activityId: string, scheduleId: string, scheduleData: Partial<ActivitySchedule>): Promise<ActivitySchedule | null> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/schedules/${scheduleId}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules/${scheduleId}`;

    // Prepare backend data
    const backendData = prepareActivityScheduleForBackend(scheduleData);

    try {
        if (useProxyApi) {
            const session = await getSession();
            const response = await fetch(url, {
                method: "PUT",
                credentials: "include",
                headers: {
                    "Content-Type": "application/json",
                    ...(session?.user?.token && {
                        Authorization: `Bearer ${session.user.token}`,
                    }),
                },
                body: JSON.stringify(backendData),
            });

            if (!response.ok) {
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<BackendActivitySchedule> | BackendActivitySchedule;
            
            // Extract the data from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return mapActivityScheduleResponse(responseData.data);
            }
            return mapActivityScheduleResponse(responseData);
        } else {
            const response = await api.put<ApiResponse<BackendActivitySchedule>>(url, backendData);
            return mapActivityScheduleResponse(response.data.data);
        }
    } catch {
        return null;
    }
}

// Delete a schedule for an activity
export async function deleteActivitySchedule(activityId: string, scheduleId: string): Promise<boolean> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/schedules/${scheduleId}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/schedules/${scheduleId}`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }
            
            return true;
        } else {
            await api.delete(url);
            return true;
        }
    } catch {
        return false;
    }
}

// Get all supervisors assigned to an activity
export async function getActivitySupervisors(activityId: string): Promise<Array<{ id: string; staff_id: string; is_primary: boolean; name: string }>> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/supervisors`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<BackendActivitySupervisor[]> | BackendActivitySupervisor[];
            
            // Extract the array from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return Array.isArray(responseData.data)
                    ? responseData.data.map(s => ({
                        id: String(s.id),
                        staff_id: String(s.staff_id),
                        is_primary: s.is_primary,
                        name: s.first_name && s.last_name ? `${s.first_name} ${s.last_name}` : `Supervisor ${s.id}`
                    }))
                    : [];
            }
            return Array.isArray(responseData)
                ? responseData.map(s => ({
                    id: String(s.id),
                    staff_id: String(s.staff_id),
                    is_primary: s.is_primary,
                    name: s.first_name && s.last_name ? `${s.first_name} ${s.last_name}` : `Supervisor ${s.id}`
                }))
                : [];
        } else {
            const response = await api.get<ApiResponse<BackendActivitySupervisor[]>>(url);
            return Array.isArray(response.data.data)
                ? response.data.data.map(s => ({
                    id: String(s.id),
                    staff_id: String(s.staff_id),
                    is_primary: s.is_primary,
                    name: s.first_name && s.last_name ? `${s.first_name} ${s.last_name}` : `Supervisor ${s.id}`
                }))
                : [];
        }
    } catch {
        return [];
    }
}

// Get available supervisors for an activity (not yet assigned)
export async function getAvailableSupervisors(activityId: string): Promise<Array<{ id: string; name: string }>> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/supervisors/available`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors/available`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<BackendSupervisor[]> | BackendSupervisor[];
            
            // Extract the array from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return Array.isArray(responseData.data)
                    ? responseData.data.map(mapSupervisorResponse)
                    : [];
            }
            return Array.isArray(responseData)
                ? responseData.map(mapSupervisorResponse)
                : [];
        } else {
            const response = await api.get<ApiResponse<BackendSupervisor[]>>(url);
            return Array.isArray(response.data.data)
                ? response.data.data.map(mapSupervisorResponse)
                : [];
        }
    } catch {
        return [];
    }
}

// Assign a supervisor to an activity
export async function assignSupervisor(activityId: string, supervisorData: { staff_id: string; is_primary?: boolean }): Promise<boolean> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/supervisors`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors`;

    // Convert staff_id to number for backend and set is_primary if defined
    const backendData = {
        staff_id: parseInt(supervisorData.staff_id, 10),
        is_primary: supervisorData.is_primary
    };

    try {
        if (useProxyApi) {
            const session = await getSession();
            const response = await fetch(url, {
                method: "POST",
                credentials: "include",
                headers: {
                    "Content-Type": "application/json",
                    ...(session?.user?.token && {
                        Authorization: `Bearer ${session.user.token}`,
                    }),
                },
                body: JSON.stringify(backendData),
            });

            if (!response.ok) {
                throw new Error(`API error: ${response.status}`);
            }
            
            return true;
        } else {
            await api.post(url, backendData);
            return true;
        }
    } catch {
        return false;
    }
}

// Update supervisor role (e.g., set/unset primary status)
export async function updateSupervisorRole(activityId: string, supervisorId: string, roleData: { is_primary: boolean }): Promise<boolean> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/supervisors/${supervisorId}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors/${supervisorId}`;

    try {
        if (useProxyApi) {
            const session = await getSession();
            const response = await fetch(url, {
                method: "PUT",
                credentials: "include",
                headers: {
                    "Content-Type": "application/json",
                    ...(session?.user?.token && {
                        Authorization: `Bearer ${session.user.token}`,
                    }),
                },
                body: JSON.stringify(roleData),
            });

            if (!response.ok) {
                throw new Error(`API error: ${response.status}`);
            }
            
            return true;
        } else {
            await api.put(url, roleData);
            return true;
        }
    } catch {
        return false;
    }
}

// Remove a supervisor from an activity
export async function removeSupervisor(activityId: string, supervisorId: string): Promise<boolean> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/supervisors/${supervisorId}`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/supervisors/${supervisorId}`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }
            
            return true;
        } else {
            await api.delete(url);
            return true;
        }
    } catch {
        return false;
    }
}

// Get all students eligible for enrollment (not yet enrolled)
export async function getAvailableStudents(activityId: string, filters?: { search?: string; group_id?: string }): Promise<Array<{ id: string; name: string; school_class: string }>> {
    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi
        ? `/api/activities/${activityId}/students`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/students`;
    
    // Build query parameters - always include available=true
    const params = new URLSearchParams();
    params.append("available", "true");
    
    // Add additional filters if provided
    if (filters) {
        if (filters.search) params.append("search", filters.search);
        if (filters.group_id) params.append("group_id", filters.group_id);
    }
    
    const queryString = params.toString();
    if (queryString) {
        url += `?${queryString}`;
    }

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<Array<{ id: number; name: string; school_class: string }>> | Array<{ id: number; name: string; school_class: string }>;
            
            // Extract the array from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return Array.isArray(responseData.data)
                    ? responseData.data.map(s => ({
                        id: String(s.id),
                        name: s.name,
                        school_class: s.school_class
                    }))
                    : [];
            }
            return Array.isArray(responseData)
                ? responseData.map(s => ({
                    id: String(s.id),
                    name: s.name,
                    school_class: s.school_class
                }))
                : [];
        } else {
            const response = await api.get<ApiResponse<Array<{ id: number; name: string; school_class: string }>>>(url);
            return Array.isArray(response.data.data)
                ? response.data.data.map(s => ({
                    id: String(s.id),
                    name: s.name,
                    school_class: s.school_class
                }))
                : [];
        }
    } catch {
        return [];
    }
}

// Get activities a student is enrolled in
export async function getStudentEnrollments(studentId: string): Promise<Activity[]> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/students/${studentId}/activities`
        : `${env.NEXT_PUBLIC_API_URL}/api/students/${studentId}/activities`;

    try {
        if (useProxyApi) {
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
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<BackendActivity[]> | BackendActivity[];
            
            // Extract the array from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return Array.isArray(responseData.data)
                    ? responseData.data.map(mapActivityResponse)
                    : [];
            }
            return Array.isArray(responseData)
                ? responseData.map(mapActivityResponse)
                : [];
        } else {
            const response = await api.get<ApiResponse<BackendActivity[]>>(url);
            return Array.isArray(response.data.data)
                ? response.data.data.map(mapActivityResponse)
                : [];
        }
    } catch {
        return [];
    }
}

// Batch update student enrollments (add or remove multiple students at once)
export async function updateGroupEnrollments(activityId: string, data: { student_ids: string[] }): Promise<boolean> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${activityId}/students`
        : `${env.NEXT_PUBLIC_API_URL}/api/activities/${activityId}/students`;

    // The API expects student_ids as an array of strings when using proxy API
    const requestData = useProxyApi ? data : {
        student_ids: data.student_ids.map(id => parseInt(id, 10))
    };

    try {
        if (useProxyApi) {
            let session = await getSession();
            
            // Check if we have a valid session
            if (!session?.user?.token) {
                throw new Error("No authentication token available. Please log in again.");
            }
            
            let response = await fetch(url, {
                method: "PUT",
                credentials: "include",
                headers: {
                    "Content-Type": "application/json",
                    Authorization: `Bearer ${session.user.token}`,
                },
                body: JSON.stringify(requestData),
            });

            // Handle 401 by trying to refresh token
            if (response.status === 401) {
                console.log("Token expired, attempting to refresh...");
                const refreshSuccessful = await handleAuthFailure();
                
                if (refreshSuccessful) {
                    // Get the new session with updated token
                    session = await getSession();
                    
                    if (session?.user?.token) {
                        // Retry the request with new token
                        response = await fetch(url, {
                            method: "PUT",
                            credentials: "include",
                            headers: {
                                "Content-Type": "application/json",
                                Authorization: `Bearer ${session.user.token}`,
                            },
                            body: JSON.stringify(requestData),
                        });
                    }
                }
            }

            if (!response.ok) {
                // Provide more specific error messages
                if (response.status === 401) {
                    throw new Error("Authentication expired. Please log in again.");
                } else if (response.status === 403) {
                    throw new Error("You don't have permission to modify enrollments.");
                } else {
                    throw new Error(`API error: ${response.status}`);
                }
            }
            
            return true;
        } else {
            await api.put(url, requestData);
            return true;
        }
    } catch (error) {
        console.error('Error updating group enrollments:', error);
        throw error; // Re-throw to let caller handle it
    }
}