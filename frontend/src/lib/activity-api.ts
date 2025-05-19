// lib/activity-api.ts
import { getSession } from "next-auth/react";
import { env } from "~/env";
import api from "./api";
import {
    mapActivityResponse,
    mapActivityCategoryResponse,
    mapSupervisorResponse,
    prepareActivityForBackend,
    type Activity,
    type ActivityCategory,
    type CreateActivityRequest,
    type UpdateActivityRequest,
    type ActivityFilter,
    type BackendActivity,
    type BackendActivityCategory,
    type BackendSupervisor,
    type Supervisor
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
    Supervisor,
    BackendActivity,
    BackendActivityCategory,
    BackendSupervisor
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
    } catch (error) {
        console.error("Error fetching activities:", error);
        return [];
    }
}

// Get a single activity by ID
export async function getActivity(id: string): Promise<Activity> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${id}`
        : `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;

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
                const errorText = await response.text();
                console.error(`API error: ${response.status}`, errorText);
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
        console.error("Error fetching activity:", error);
        throw error;
    }
}

// Create a new activity
export async function createActivity(data: CreateActivityRequest): Promise<Activity> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? "/api/activities"
        : `${env.NEXT_PUBLIC_API_URL}/activities`;

    // No need to prepare for backend - data already in correct format
    console.log('Creating activity with data:', data);
    
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
                const errorText = await response.text();
                console.error(`API error: ${response.status}`, errorText);
                throw new Error(`API error: ${response.status}`);
            }

            try {
                const responseData = await response.json();
                console.log('Raw activity creation response:', responseData);
                
                // Try to extract data regardless of format
                if (responseData) {
                    // Handle wrapped response { status/success: "success", data: Activity }
                    if (typeof responseData === 'object') {
                        if ('data' in responseData && responseData.data) {
                            // Try to extract ID and update safeActivity if possible
                            if (typeof responseData.data === 'object' && 'id' in responseData.data) {
                                safeActivity.id = String(responseData.data.id);
                                
                                // If it's a full BackendActivity, map it
                                if ('name' in responseData.data && 
                                    'max_participants' in responseData.data && 
                                    'category_id' in responseData.data) {
                                    return mapActivityResponse(responseData.data as BackendActivity);
                                }
                            }
                            return responseData.data;
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
            } catch (parseError) {
                console.error("Error parsing activity creation response:", parseError);
                // Even if parsing fails, we know the POST was successful, so return safe activity
                return safeActivity;
            }
        } else {
            try {
                const response = await api.post<any>(
                    url,
                    data // Send the CreateActivityRequest directly
                );
                
                console.log('Server-side API response:', response);
                
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
                                        return mapActivityResponse(response.data.data as BackendActivity);
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
                                    return mapActivityResponse(response.data as BackendActivity);
                                }
                            }
                        }
                    }
                }
                
                // Fallback to safe activity if we couldn't extract proper data
                return safeActivity;
            } catch (apiError) {
                console.error("Error with server-side API call:", apiError);
                throw apiError;
            }
        }
    } catch (error) {
        console.error("Error creating activity:", error);
        throw error;
    }
}

// Update an activity
export async function updateActivity(id: string, data: UpdateActivityRequest): Promise<Activity> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${id}`
        : `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;

    const backendData = prepareActivityForBackend(data);

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
                const errorText = await response.text();
                console.error(`API error: ${response.status}`, errorText);
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
        console.error("Error updating activity:", error);
        throw error;
    }
}

// Delete an activity
export async function deleteActivity(id: string): Promise<void> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/activities/${id}`
        : `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;

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
                const errorText = await response.text();
                console.error(`API error: ${response.status}`, errorText);
                throw new Error(`API error: ${response.status}`);
            }
        } else {
            await api.delete(url);
        }
    } catch (error) {
        console.error("Error deleting activity:", error);
        throw error;
    }
}

// Get all categories
export async function getCategories(): Promise<ActivityCategory[]> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? "/api/activities/categories"
        : `${env.NEXT_PUBLIC_API_URL}/activities/categories`;

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
                const errorText = await response.text();
                console.error(`API error: ${response.status}`, errorText);
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<ActivityCategory[]> | ActivityCategory[];
            console.log('Raw categories response:', responseData);
            
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
        console.error("Error fetching categories:", error);
        return [];
    }
}

// Get all supervisors
export async function getSupervisors(): Promise<Array<{ id: string; name: string }>> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? "/api/activities/supervisors"
        : `${env.NEXT_PUBLIC_API_URL}/activities/supervisors`;

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
                const errorText = await response.text();
                console.error(`API error: ${response.status}`, errorText);
                throw new Error(`API error: ${response.status}`);
            }

            const responseData = await response.json() as ApiResponse<Array<{ id: string; name: string }>> | Array<{ id: string; name: string }>;
            console.log('Raw supervisors response:', responseData);
            
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
    } catch (error) {
        console.error("Error fetching supervisors:", error);
        return [];
    }
}