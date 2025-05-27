// lib/student-api.ts
import { getSession } from "next-auth/react";
import { env } from "~/env";
import api from "./api";
import {
    mapStudentResponse,
    mapStudentsResponse,
    mapStudentDetailResponse,
    type Student,
    type BackendStudent,
    type BackendStudentDetail
} from "./student-helpers";
import {
    mapGroupResponse,
    type Group,
    type BackendGroup
} from "./group-helpers";

// Filter interface for searching students
export interface StudentFilters {
    search?: string;
    school_class?: string;
    group_id?: string;
    location?: string;
    guardian_name?: string;
    first_name?: string;
    last_name?: string;
    page?: number;
    page_size?: number;
}

// Generic API response interface
interface ApiResponse<T> {
    data: T;
    message?: string;
    status?: string;
}

// Paginated response interface
interface PaginatedResponse<T> {
    status: string;
    data: T[];
    pagination: {
        current_page: number;
        page_size: number;
        total_pages: number;
        total_records: number;
    };
    message?: string;
}

// Standardized error handling function for students API
function handleStudentApiError(error: unknown, context: string): never {
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
                code: `STUDENT_API_ERROR_${status}`
            }));
        }
    }
    
    // Default error response
    throw new Error(JSON.stringify({
        status: 500,
        message: `Failed to ${context}: ${error instanceof Error ? error.message : "Unknown error"}`,
        code: "STUDENT_API_ERROR_UNKNOWN"
    }));
}

// Fetch students with filters and pagination
export async function fetchStudents(filters?: StudentFilters): Promise<{
    students: Student[];
    pagination?: {
        current_page: number;
        page_size: number;
        total_pages: number;
        total_records: number;
    };
}> {
    const params = new URLSearchParams();
    
    // Add all filters to query params
    if (filters?.search) params.append("search", filters.search);
    if (filters?.school_class) params.append("school_class", filters.school_class);
    if (filters?.group_id) params.append("group_id", filters.group_id);
    if (filters?.location) params.append("location", filters.location);
    if (filters?.guardian_name) params.append("guardian_name", filters.guardian_name);
    if (filters?.first_name) params.append("first_name", filters.first_name);
    if (filters?.last_name) params.append("last_name", filters.last_name);
    if (filters?.page) params.append("page", filters.page.toString());
    if (filters?.page_size) params.append("page_size", filters.page_size.toString());

    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi
        ? "/api/students"
        : `${env.NEXT_PUBLIC_API_URL}/students`;

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
                throw new Error(`API error (${response.status}): ${response.statusText}`);
            }

            const responseData = await response.json() as Student[] | PaginatedResponse<Student>;
            
            
            // Check if it's a paginated response
            if (responseData && typeof responseData === 'object' && 'data' in responseData && 'pagination' in responseData) {
                const paginatedData = responseData;
                return {
                    students: paginatedData.data,
                    pagination: paginatedData.pagination
                };
            }
            
            // Fallback for non-paginated response (already mapped by Next.js route)
            const students = Array.isArray(responseData) ? responseData : [];
            return {
                students: students
            };
        } else {
            // Server-side: use axios with the API URL directly
            const response = await api.get<PaginatedResponse<BackendStudent>>(url);
            
            if (response.data?.data) {
                return {
                    students: mapStudentsResponse(response.data.data),
                    pagination: response.data.pagination
                };
            }
            
            return { students: [] };
        }
    } catch (error) {
        handleStudentApiError(error, "fetch students");
    }
}

// Fetch a single student by ID
export async function fetchStudent(id: string): Promise<Student> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/students/${id}`
        : `${env.NEXT_PUBLIC_API_URL}/students/${id}`;

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
                throw new Error(`API error (${response.status}): ${response.statusText}`);
            }

            const responseData = await response.json() as ApiResponse<BackendStudentDetail> | BackendStudentDetail;
            
            // Extract the data from the response wrapper if needed
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return mapStudentDetailResponse(responseData.data);
            }
            return mapStudentDetailResponse(responseData);
        } else {
            const response = await api.get<ApiResponse<BackendStudentDetail>>(url);
            return mapStudentDetailResponse(response.data.data);
        }
    } catch (error) {
        handleStudentApiError(error, "fetch student");
    }
}

// Create a new student
export async function createStudent(studentData: {
    first_name: string;
    last_name: string;
    school_class: string;
    guardian_name: string;
    guardian_contact: string;
    group_id?: number;
    tag_id?: string;
    guardian_email?: string;
    guardian_phone?: string;
}): Promise<Student> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? "/api/students"
        : `${env.NEXT_PUBLIC_API_URL}/students`;

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
                body: JSON.stringify(studentData),
            });

            if (!response.ok) {
                throw new Error(`API error (${response.status}): ${response.statusText}`);
            }

            const responseData = await response.json() as ApiResponse<BackendStudent> | BackendStudent;
            
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return mapStudentResponse(responseData.data);
            }
            return mapStudentResponse(responseData);
        } else {
            const response = await api.post<ApiResponse<BackendStudent>>(url, studentData);
            return mapStudentResponse(response.data.data);
        }
    } catch (error) {
        handleStudentApiError(error, "create student");
    }
}

// Update a student
export async function updateStudent(
    id: string,
    studentData: Partial<{
        first_name: string;
        last_name: string;
        school_class: string;
        guardian_name: string;
        guardian_contact: string;
        group_id: number;
        tag_id: string;
        guardian_email: string;
        guardian_phone: string;
    }>
): Promise<Student> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/students/${id}`
        : `${env.NEXT_PUBLIC_API_URL}/students/${id}`;

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
                body: JSON.stringify(studentData),
            });

            if (!response.ok) {
                throw new Error(`API error (${response.status}): ${response.statusText}`);
            }

            const responseData = await response.json() as ApiResponse<BackendStudent> | BackendStudent;
            
            if (responseData && typeof responseData === 'object' && 'data' in responseData) {
                return mapStudentResponse(responseData.data);
            }
            return mapStudentResponse(responseData);
        } else {
            const response = await api.put<ApiResponse<BackendStudent>>(url, studentData);
            return mapStudentResponse(response.data.data);
        }
    } catch (error) {
        handleStudentApiError(error, "update student");
    }
}

// Delete a student
export async function deleteStudent(id: string): Promise<void> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? `/api/students/${id}`
        : `${env.NEXT_PUBLIC_API_URL}/students/${id}`;

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
                throw new Error(`API error (${response.status}): ${response.statusText}`);
            }
        } else {
            await api.delete(url);
        }
    } catch (error) {
        handleStudentApiError(error, "delete student");
    }
}

// Helper function to map attendance filter to location string
export function mapAttendanceFilterToLocation(attendanceFilter: string): string | undefined {
    switch (attendanceFilter) {
        case "in_house":
            return "In House";
        case "wc":
            return "WC";
        case "school_yard":
            return "School Yard";
        case "bus":
            return "Bus";
        case "all":
            return undefined;
        default:
            return undefined;
    }
}

// Fetch all groups (for filter dropdown)
export async function fetchGroups(): Promise<Group[]> {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
        ? "/api/groups"
        : `${env.NEXT_PUBLIC_API_URL}/groups`;

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
                throw new Error(`API error (${response.status}): ${response.statusText}`);
            }

            const responseData = await response.json() as BackendGroup[];
            
            // The Next.js route returns BackendGroup[] directly
            const groups = Array.isArray(responseData) ? responseData : [];
            
            return groups.map(mapGroupResponse);
        } else {
            const response = await api.get<{ data: BackendGroup[] }>(url);
            const groups = response.data.data || [];
            return groups.map(mapGroupResponse);
        }
    } catch (error) {
        console.error("Error fetching groups:", error);
        return [];
    }
}