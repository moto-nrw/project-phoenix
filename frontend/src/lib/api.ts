import axios from "axios";
import type { AxiosError, AxiosRequestConfig, AxiosResponse } from "axios";
import { getSession } from "next-auth/react";
import { env } from "~/env";
import type { ApiResponse } from "./api-helpers";
import {
  mapSingleStudentResponse,
  mapStudentsResponse,
  prepareStudentForBackend,
} from "./student-helpers";
import type { BackendStudent, Student } from "./student-helpers";
import {
  mapSingleGroupResponse,
  mapGroupResponse, // Used in exported function
  prepareGroupForBackend,
  mapSingleCombinedGroupResponse,
  mapCombinedGroupResponse, // Used in exported function
  prepareCombinedGroupForBackend,
  mapGroupsResponse,
  mapCombinedGroupsResponse,
} from "./group-helpers";

// Export functions and types to prevent unused warnings
export { mapGroupResponse, mapCombinedGroupResponse };
import type { 
  BackendGroup, 
  BackendCombinedGroup, 
  CombinedGroup as ImportedCombinedGroup, 
  Group as ImportedGroup 
} from "./group-helpers";
import {
  mapSingleRoomResponse,
  mapRoomResponse, // Used in exported function
  prepareRoomForBackend,
  mapRoomsResponse,
} from "./room-helpers";

// Export to prevent unused warning
export { mapRoomResponse };
import type { BackendRoom } from "./room-helpers";
import { handleAuthFailure } from "./auth-api";

// Helper function to safely handle errors
function handleApiError(error: unknown, context: string): Error {
  console.error(`${context}:`, error);
  return new Error(`${context}: ${error instanceof Error ? error.message : String(error)}`);
}

// Paginated response interface for API responses with pagination metadata
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

// Create an Axios instance
const api = axios.create({
  baseURL: env.NEXT_PUBLIC_API_URL, // Client-safe environment variable pointing to the backend server
  headers: {
    "Content-Type": "application/json",
  },
  // Important: Include credentials with every request to ensure cookies are sent
  withCredentials: true,
});

// Add a request interceptor to include the auth token
api.interceptors.request.use(
  async (config) => {
    // Get the session to retrieve the token
    const session = await getSession();

    // If there's a token, add it to the headers
    if (session?.user?.token) {
      config.headers.Authorization = `Bearer ${session.user.token}`;
    }

    return config;
  },
  (error: Error) => {
    return Promise.reject(error);
  },
);

// Add a response interceptor to handle common errors
api.interceptors.response.use(
  (response: AxiosResponse) => {
    return response;
  },
  async (error: AxiosError) => {
    const originalRequest = error.config as AxiosRequestConfig & {
      _retry?: boolean;
    };

    // If the error is a 401 (Unauthorized) and we haven't retried yet
    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true;

      console.log("Received 401 error, attempting to refresh token");

      // Try to refresh the token and retry the request
      const refreshSuccessful = await handleAuthFailure();

      if (refreshSuccessful && originalRequest.headers) {
        // Get the newest session with updated token
        const session = await getSession();

        if (session?.user?.token) {
          console.log("Using refreshed token for retry");
          originalRequest.headers.Authorization = `Bearer ${session.user.token}`;
          return api(originalRequest);
        }
      } else {
        console.log("Token refresh failed, unable to retry request");
        // Force redirect to login if we're in the browser
        if (typeof window !== "undefined") {
          window.location.href = "/";
        }
      }
    }

    return Promise.reject(error);
  },
);

// Re-export types for external usage
export type { Student } from "./student-helpers";
export type Group = ImportedGroup;
export type CombinedGroup = ImportedCombinedGroup;

// Room-related interfaces
export interface Room {
  id: string;
  name: string; 
  building?: string;
  floor: number;
  capacity: number;
  category: string;
  color: string;
  deviceId?: string;
  isOccupied: boolean;
  activityName?: string;
  groupName?: string;
  supervisorName?: string;
  studentCount?: number;
  createdAt?: string;
  updatedAt?: string;
}

// API services
export const studentService = {
  // Get all students
  getStudents: async (filters?: {
    search?: string;
    inHouse?: boolean;
    groupId?: string;
  }): Promise<Student[]> => {
    // Build query parameters
    const params = new URLSearchParams();
    if (filters?.search) params.append("search", filters.search);
    if (filters?.inHouse !== undefined)
      params.append("in_house", filters.inHouse.toString());
    if (filters?.groupId) params.append("group_id", filters.groupId);

    // Use the nextjs api route which handles auth token properly
    // Use relative URL in browser environment
    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi
      ? "/api/students"
      : `${env.NEXT_PUBLIC_API_URL}/students`;

    try {
      // Build query string for API route
      const queryString = params.toString();
      if (queryString) {
        url += `?${queryString}`;
      }

      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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

          // Try token refresh on 401 errors
          if (response.status === 401) {
            const refreshSuccessful = await handleAuthFailure();

            if (refreshSuccessful) {
              // Try the request again after token refresh
              const newSession = await getSession();
              const retryResponse = await fetch(url, {
                credentials: "include",
                headers: newSession?.user?.token
                  ? {
                      Authorization: `Bearer ${newSession.user.token}`,
                      "Content-Type": "application/json",
                    }
                  : undefined,
              });

              if (retryResponse.ok) {
                // Type assertion to avoid unsafe assignment
                const responseData = await retryResponse.json() as {
                  data?: Student[];
                  [key: string]: unknown;
                };
                
                // The Next.js API route uses route wrapper which may wrap the response
                if (responseData && typeof responseData === 'object' && 'data' in responseData && responseData.data) {
                  // If wrapped, extract the data
                  return responseData.data;
                }
                
                // Otherwise, treat as direct array
                return responseData as unknown as Student[];
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        const responseData = await response.json() as {
          data?: Student[];
          [key: string]: unknown;
        };
        
        // The Next.js API route uses route wrapper which may wrap the response
        if (responseData && typeof responseData === 'object' && 'data' in responseData && responseData.data) {
          // If wrapped, extract the data
          return responseData.data;
        }
        
        // Otherwise, treat as direct array
        return responseData as unknown as Student[];
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url, { params });
        return mapStudentsResponse((response as { data: unknown }).data as BackendStudent[]);
      }
    } catch (error) {
      throw handleApiError(error, "Error fetching students");
    }
  },

  // Get a specific student by ID
  getStudent: async (id: string): Promise<Student> => {
    // Use the nextjs api route which handles auth token properly
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/students/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/students/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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

          // Try token refresh on 401 errors
          if (response.status === 401) {
            const refreshSuccessful = await handleAuthFailure();

            if (refreshSuccessful) {
              // Try the request again after token refresh
              const newSession = await getSession();
              const retryResponse = await fetch(url, {
                credentials: "include",
                headers: newSession?.user?.token
                  ? {
                      Authorization: `Bearer ${newSession.user.token}`,
                      "Content-Type": "application/json",
                    }
                  : undefined,
              });

              if (retryResponse.ok) {
                // Type assertion to avoid unsafe assignment
                const data: unknown = await retryResponse.json();
                // Return as Student with additional fields - route handler already unwrapped it
                return data as Student;
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        const responseData = await response.json() as unknown;
        
        // Return as Student with additional fields - route handler already unwrapped it
        return responseData as Student;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        // Return as Student with additional fields
        return response.data as Student;
      }
    } catch (error) {
      throw handleApiError(error, `Error fetching student ${id}`);
    }
  },

  // Create a new student
  createStudent: async (student: Omit<Student, "id">): Promise<Student> => {
    // Transform from frontend model to backend model
    const backendStudent = prepareStudentForBackend(student);

    // Basic validation for student creation - match backend requirements
    if (!backendStudent.first_name) {
      throw new Error("First name is required");
    }
    if (!backendStudent.last_name) {
      throw new Error("Last name is required");
    }
    if (!backendStudent.school_class) {
      throw new Error("School class is required");
    }
    if (!backendStudent.guardian_name) {
      throw new Error("Guardian name is required");
    }
    if (!backendStudent.guardian_contact) {
      throw new Error("Guardian contact is required");
    }

    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/students`
      : `${env.NEXT_PUBLIC_API_URL}/students`;

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
          body: JSON.stringify(backendStudent),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          // Try to parse error for more detailed message
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(`API error: ${errorJson.error}`);
            }
          } catch {
            // If parsing fails, use status code
          }
          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        const data: unknown = await response.json();
        // Map response to our frontend model
        const mappedResponse = mapSingleStudentResponse({ data: data as BackendStudent });
        return mappedResponse;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendStudent);
        return mapSingleStudentResponse({ 
          data: response.data as unknown as BackendStudent 
        });
      }
    } catch (error) {
      throw handleApiError(error, "Error creating student");
    }
  },

  // Update a student
  updateStudent: async (
    id: string,
    student: Partial<Student>,
  ): Promise<Student> => {
    // First, capture the name fields so we can track them in the response later
    const firstName = student.first_name;
    const secondName = student.second_name;

    // Transform from frontend model to backend model updates
    const backendUpdates = prepareStudentForBackend(student);

    // Validation for required fields in updates
    // Note: For updates, we only validate fields that are provided
    // Backend will handle partial updates correctly

    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/students/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/students/${id}`;

    try {
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
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);

          // Try to parse error text as JSON for more detailed error
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(
                `API error ${response.status}: ${errorJson.error}`,
              );
            }
          } catch {
            // If parsing fails, use status code + error text
            throw new Error(
              `API error ${response.status}: ${errorText.substring(0, 100)}`,
            );
          }
        }

        // Type assertion to avoid unsafe assignment
        const data: unknown = await response.json();
        // Map response to our frontend model
        const mappedResponse = mapSingleStudentResponse({ data: data as BackendStudent });
        return mappedResponse;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        // Merge the returned data with our local name changes if provided
        const mappedResponse = mapSingleStudentResponse({
          data: response.data as unknown as BackendStudent
        });
        if (firstName || secondName) {
          if (firstName) mappedResponse.first_name = firstName;
          if (secondName) mappedResponse.second_name = secondName;
          // Update the display name as well
          if (firstName && secondName) {
            mappedResponse.name = `${firstName} ${secondName}`;
          } else if (firstName) {
            mappedResponse.name = firstName;
          } else if (secondName) {
            mappedResponse.name = secondName;
          }
        }
        return mappedResponse;
      }
    } catch (error) {
      throw handleApiError(error, `Error updating student ${id}`);
    }
  },

  // Delete a student
  deleteStudent: async (id: string): Promise<void> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/students/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/students/${id}`;

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
      throw handleApiError(error, `Error deleting student ${id}`);
    }
  },
};

// Group service for API operations
export const groupService = {
  // Get all groups
  getGroups: async (filters?: { search?: string }): Promise<Group[]> => {
    // Build query parameters
    const params = new URLSearchParams();
    if (filters?.search) params.append("search", filters.search);

    // Use the nextjs api route which handles auth token properly
    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi ? "/api/groups" : `${env.NEXT_PUBLIC_API_URL}/groups`;

    try {
      // Build query string for API route
      const queryString = params.toString();
      if (queryString) {
        url += `?${queryString}`;
      }

      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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
          // Don't log 403 errors as errors - they're expected for permission issues
          if (response.status === 403) {
            console.log(`Permission denied for groups endpoint (403)`);
          } else {
            console.error(`API error: ${response.status}`, errorText);
          }

          // Try token refresh on 401 errors
          if (response.status === 401) {
            const refreshSuccessful = await handleAuthFailure();

            if (refreshSuccessful) {
              // Try the request again after token refresh
              const newSession = await getSession();
              const retryResponse = await fetch(url, {
                credentials: "include",
                headers: newSession?.user?.token
                  ? {
                      Authorization: `Bearer ${newSession.user.token}`,
                      "Content-Type": "application/json",
                    }
                  : undefined,
              });

              if (retryResponse.ok) {
                // Type assertion to avoid unsafe assignment
                const responseData: unknown = await retryResponse.json();
                return mapGroupsResponse(responseData as BackendGroup[]);
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        const responseData: unknown = await response.json();
        console.log('Client-side groups response:', responseData);
        
        // Check if the response is wrapped in our ApiResponse format
        let groups: BackendGroup[] = [];
        if (typeof responseData === 'object' && responseData !== null && 'data' in responseData) {
          // It's wrapped in ApiResponse
          const apiResponse = responseData as { data?: unknown };
          groups = Array.isArray(apiResponse.data) ? apiResponse.data as BackendGroup[] : [];
        } else if (Array.isArray(responseData)) {
          // It's a direct array
          groups = responseData as BackendGroup[];
        }
        
        console.log('Groups before mapping:', groups);
        
        const mappedGroups = mapGroupsResponse(groups);
        console.log('Groups after mapping:', mappedGroups);
        
        return mappedGroups;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url, { params });
        const paginatedResponse = response.data as PaginatedResponse<BackendGroup>;
        return mapGroupsResponse(paginatedResponse.data);
      }
    } catch (error) {
      console.error("Error fetching groups:", error);
      throw error;
    }
  },

  // Get a specific group by ID
  getGroup: async (id: string): Promise<Group> => {
    // Use the nextjs api route which handles auth token properly
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/groups/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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

          // Try token refresh on 401 errors
          if (response.status === 401) {
            const refreshSuccessful = await handleAuthFailure();

            if (refreshSuccessful) {
              // Try the request again after token refresh
              const newSession = await getSession();
              const retryResponse = await fetch(url, {
                credentials: "include",
                headers: newSession?.user?.token
                  ? {
                      Authorization: `Bearer ${newSession.user.token}`,
                      "Content-Type": "application/json",
                    }
                  : undefined,
              });

              if (retryResponse.ok) {
                const responseData: unknown = await retryResponse.json();
                console.log('Group API retry response:', responseData);
                
                let groupData: BackendGroup;
                if (typeof responseData === 'object' && responseData !== null) {
                  if ('data' in responseData) {
                    groupData = (responseData as { data: BackendGroup }).data;
                  } else {
                    groupData = responseData as BackendGroup;
                  }
                } else {
                  throw new Error('Invalid response format from API');
                }
                
                if (!groupData) {
                  throw new Error('No group data in response');
                }
                
                return mapSingleGroupResponse({ data: groupData });
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        const responseData: unknown = await response.json();
        console.log('Group API response:', responseData);
        
        // Check if the response is wrapped in an ApiResponse format
        let groupData: BackendGroup;
        if (typeof responseData === 'object' && responseData !== null) {
          if ('success' in responseData && 'data' in responseData) {
            // Response is wrapped in ApiResponse format { success: true, message: "...", data: {...} }
            const apiResponse = responseData as ApiResponse<unknown>;
            
            // Check for double-wrapped response
            if (apiResponse.data && typeof apiResponse.data === 'object' && 'data' in apiResponse.data) {
              // Double-wrapped: extract the inner data
              const dataWrapper = apiResponse.data as { data: BackendGroup };
              groupData = dataWrapper.data;
            } else {
              // Single-wrapped
              groupData = apiResponse.data as BackendGroup;
            }
          } else if ('data' in responseData) {
            // Response is wrapped in { data: ... }
            const dataResponse = responseData as { data: BackendGroup };
            groupData = dataResponse.data;
          } else {
            // Response is direct group data
            groupData = responseData as BackendGroup;
          }
        } else {
          throw new Error('Invalid response format from API');
        }
        
        if (!groupData) {
          throw new Error('No group data in response');
        }
        
        console.log('Actual group data:', groupData);
        const mappedGroup = mapGroupResponse(groupData);
        console.log('Final mapped group:', mappedGroup);
        return mappedGroup;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapGroupResponse(response.data as BackendGroup);
      }
    } catch (error) {
      console.error(`Error fetching group ${id}:`, error);
      throw error;
    }
  },

  // Create a new group
  createGroup: async (group: Omit<Group, "id">): Promise<Group> => {
    // Transform from frontend model to backend model
    const backendGroup = prepareGroupForBackend(group);

    // Basic validation for group creation
    if (!backendGroup.name) {
      throw new Error("Missing required field: name");
    }

    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups`
      : `${env.NEXT_PUBLIC_API_URL}/groups`;

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
          body: JSON.stringify(backendGroup),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          // Try to parse error for more detailed message
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(`API error: ${errorJson.error}`);
            }
          } catch {
            // If parsing fails, use status code
          }
          throw new Error(`API error: ${response.status}`);
        }

        const data = (await response.json()) as BackendGroup;
        return mapSingleGroupResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendGroup);
        return mapSingleGroupResponse({ data: response.data as BackendGroup });
      }
    } catch (error) {
      console.error(`Error creating group:`, error);
      throw error;
    }
  },

  // Update a group
  updateGroup: async (id: string, group: Partial<Group>): Promise<Group> => {
    // Transform from frontend model to backend model updates
    const backendUpdates = prepareGroupForBackend(group);

    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/groups/${id}`;

    try {
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
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);

          // Try to parse error text as JSON for more detailed error
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(
                `API error ${response.status}: ${errorJson.error}`,
              );
            }
          } catch {
            // If parsing fails, use status code + error text
            throw new Error(
              `API error ${response.status}: ${errorText.substring(0, 100)}`,
            );
          }
        }

        const data = (await response.json()) as BackendGroup;
        return mapSingleGroupResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleGroupResponse({ data: response.data as BackendGroup });
      }
    } catch (error) {
      console.error(`Error updating group ${id}:`, error);
      throw error;
    }
  },

  // Delete a group
  deleteGroup: async (id: string): Promise<void> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/groups/${id}`;

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

          // Try to parse error text as JSON for more detailed error message
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              // Throw the actual error message from the backend
              throw new Error(errorJson.error);
            }
          } catch {
            // If JSON parsing fails, check if the error text contains the specific error message
            if (errorText.includes("cannot delete group with students")) {
              throw new Error("cannot delete group with students");
            }
            // Otherwise use status code
          }

          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        try {
          await api.delete(url);
          return;
        } catch (axiosError) {
          // Handle axios error format
          const axiosErr = axiosError as AxiosError;
          if (axiosErr.response?.data) {
            // Try to extract the error message from the response data
            const errorData = axiosErr.response.data as { error?: string };
            if (errorData.error) {
              throw new Error(errorData.error);
            }
          }
          throw axiosError;
        }
      }
    } catch (error) {
      console.error(`Error deleting group ${id}:`, error);
      throw error;
    }
  },

  // Get students in a group
  getGroupStudents: async (id: string): Promise<Student[]> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/${id}/students`
      : `${env.NEXT_PUBLIC_API_URL}/groups/${id}/students`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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

        // Type assertion to avoid unsafe assignment
        const responseData = await response.json() as {
          data?: Student[];
          [key: string]: unknown;
        };
        
        // The Next.js API route uses route wrapper which may wrap the response
        if (responseData && typeof responseData === 'object' && 'data' in responseData && responseData.data) {
          // If wrapped, extract the data
          return responseData.data;
        }
        
        // Otherwise, treat as direct array
        return responseData as unknown as Student[];
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapStudentsResponse((response as { data: unknown }).data as BackendStudent[]);
      }
    } catch (error) {
      throw handleApiError(error, `Error fetching students for group ${id}`);
    }
  },

  // Add a supervisor to a group
  addSupervisor: async (
    groupId: string,
    supervisorId: string,
  ): Promise<void> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/${groupId}/supervisors`
      : `${env.NEXT_PUBLIC_API_URL}/groups/${groupId}/supervisors`;

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
          body: JSON.stringify({ supervisor_id: parseInt(supervisorId, 10) }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.post(url, { supervisor_id: parseInt(supervisorId, 10) });
        return;
      }
    } catch (error) {
      console.error(
        `Error adding supervisor ${supervisorId} to group ${groupId}:`,
        error,
      );
      throw error;
    }
  },

  // Remove a supervisor from a group
  removeSupervisor: async (
    groupId: string,
    supervisorId: string,
  ): Promise<void> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/${groupId}/supervisors/${supervisorId}`
      : `${env.NEXT_PUBLIC_API_URL}/groups/${groupId}/supervisors/${supervisorId}`;

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
      console.error(
        `Error removing supervisor ${supervisorId} from group ${groupId}:`,
        error,
      );
      throw error;
    }
  },

  // Set the representative for a group
  setRepresentative: async (
    groupId: string,
    representativeId: string,
  ): Promise<void> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/${groupId}/representative`
      : `${env.NEXT_PUBLIC_API_URL}/groups/${groupId}/representative`;

    try {
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
          body: JSON.stringify({
            representative_id: parseInt(representativeId, 10),
          }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.put(url, {
          representative_id: parseInt(representativeId, 10),
        });
        return;
      }
    } catch (error) {
      console.error(
        `Error setting representative ${representativeId} for group ${groupId}:`,
        error,
      );
      throw error;
    }
  },
};

// Combined Group service for API operations
export const combinedGroupService = {
  // Get all combined groups
  getCombinedGroups: async (): Promise<CombinedGroup[]> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? "/api/groups/combined"
      : `${env.NEXT_PUBLIC_API_URL}/groups/combined`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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

        const responseData = (await response.json()) as BackendCombinedGroup[];
        return mapCombinedGroupsResponse(responseData);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapCombinedGroupsResponse(
          response.data as BackendCombinedGroup[],
        );
      }
    } catch (error) {
      console.error("Error fetching combined groups:", error);
      throw error;
    }
  },

  // Get a specific combined group by ID
  getCombinedGroup: async (id: string): Promise<CombinedGroup> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/combined/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/groups/combined/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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

        const responseData = (await response.json()) as BackendCombinedGroup;
        return mapSingleCombinedGroupResponse({ data: responseData });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapSingleCombinedGroupResponse({
          data: response.data as BackendCombinedGroup,
        });
      }
    } catch (error) {
      console.error(`Error fetching combined group ${id}:`, error);
      throw error;
    }
  },

  // Create a new combined group
  createCombinedGroup: async (
    combinedGroup: Omit<CombinedGroup, "id">,
  ): Promise<CombinedGroup> => {
    // Transform from frontend model to backend model
    const backendCombinedGroup = prepareCombinedGroupForBackend(combinedGroup);

    // Basic validation for combined group creation
    if (!backendCombinedGroup.name) {
      throw new Error("Missing required field: name");
    }
    if (!backendCombinedGroup.access_policy) {
      throw new Error("Missing required field: access_policy");
    }

    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/combined`
      : `${env.NEXT_PUBLIC_API_URL}/groups/combined`;

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
          body: JSON.stringify(backendCombinedGroup),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        const responseData = (await response.json()) as BackendCombinedGroup;
        return mapSingleCombinedGroupResponse({ data: responseData });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendCombinedGroup);
        return mapSingleCombinedGroupResponse({
          data: response.data as BackendCombinedGroup,
        });
      }
    } catch (error) {
      console.error(`Error creating combined group:`, error);
      throw error;
    }
  },

  // Update a combined group
  updateCombinedGroup: async (
    id: string,
    combinedGroup: Partial<CombinedGroup>,
  ): Promise<CombinedGroup> => {
    // Transform from frontend model to backend model updates
    const backendUpdates = prepareCombinedGroupForBackend(combinedGroup);

    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/combined/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/groups/combined/${id}`;

    try {
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
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        const responseData = (await response.json()) as BackendCombinedGroup;
        return mapSingleCombinedGroupResponse({ data: responseData });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleCombinedGroupResponse({
          data: response.data as BackendCombinedGroup,
        });
      }
    } catch (error) {
      console.error(`Error updating combined group ${id}:`, error);
      throw error;
    }
  },

  // Delete a combined group
  deleteCombinedGroup: async (id: string): Promise<void> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/combined/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/groups/combined/${id}`;

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
      console.error(`Error deleting combined group ${id}:`, error);
      throw error;
    }
  },

  // Add a group to a combined group
  addGroupToCombined: async (
    combinedGroupId: string,
    groupId: string,
  ): Promise<void> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/combined/${combinedGroupId}/groups`
      : `${env.NEXT_PUBLIC_API_URL}/groups/combined/${combinedGroupId}/groups`;

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
          body: JSON.stringify({ group_id: parseInt(groupId, 10) }),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }

        return;
      } else {
        // Server-side: use axios with the API URL directly
        await api.post(url, { group_id: parseInt(groupId, 10) });
        return;
      }
    } catch (error) {
      console.error(
        `Error adding group ${groupId} to combined group ${combinedGroupId}:`,
        error,
      );
      throw error;
    }
  },

  // Remove a group from a combined group
  removeGroupFromCombined: async (
    combinedGroupId: string,
    groupId: string,
  ): Promise<void> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/groups/combined/${combinedGroupId}/groups/${groupId}`
      : `${env.NEXT_PUBLIC_API_URL}/groups/combined/${combinedGroupId}/groups/${groupId}`;

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
      console.error(
        `Error removing group ${groupId} from combined group ${combinedGroupId}:`,
        error,
      );
      throw error;
    }
  },
};

// Room service for API operations
export const roomService = {
  // Get all rooms
  getRooms: async (filters?: {
    building?: string;
    floor?: number;
    category?: string;
    occupied?: boolean;
    search?: string;
  }): Promise<Room[]> => {
    // Build query parameters
    const params = new URLSearchParams();
    if (filters?.search) params.append("search", filters.search);
    if (filters?.building) params.append("building", filters.building);
    if (filters?.floor !== undefined) params.append("floor", filters.floor.toString());
    if (filters?.category) params.append("category", filters.category);
    if (filters?.occupied !== undefined) params.append("occupied", filters.occupied.toString());

    // Use the nextjs api route which handles auth token properly
    const useProxyApi = typeof window !== "undefined";
    let url = useProxyApi ? "/api/rooms" : `${env.NEXT_PUBLIC_API_URL}/rooms`;

    try {
      // Build query string for API route
      const queryString = params.toString();
      if (queryString) {
        url += `?${queryString}`;
      }

      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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

          // Try token refresh on 401 errors
          if (response.status === 401) {
            const refreshSuccessful = await handleAuthFailure();

            if (refreshSuccessful) {
              // Try the request again after token refresh
              const newSession = await getSession();
              const retryResponse = await fetch(url, {
                credentials: "include",
                headers: newSession?.user?.token
                  ? {
                      Authorization: `Bearer ${newSession.user.token}`,
                      "Content-Type": "application/json",
                    }
                  : undefined,
              });

              if (retryResponse.ok) {
                try {
                  // Type assertion to avoid unsafe assignment
                  const responseData: unknown = await retryResponse.json();
                  
                  // Handle null or non-array responses
                  if (!responseData || !Array.isArray(responseData)) {
                    console.warn("API retry returned invalid response format for rooms:", responseData);
                    return [];
                  }
                  
                  return mapRoomsResponse(responseData as BackendRoom[]);
                } catch (parseError) {
                  console.error("Error parsing API retry response:", parseError);
                  return [];
                }
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        try {
          const responseData: unknown = await response.json();
          
          // Handle null or non-array responses
          if (!responseData || !Array.isArray(responseData)) {
            console.warn("API returned invalid response format for rooms:", responseData);
            return [];
          }
          
          return mapRoomsResponse(responseData as BackendRoom[]);
        } catch (parseError) {
          console.error("Error parsing API response:", parseError);
          return [];
        }
      } else {
        // Server-side: use axios with the API URL directly
        try {
          const response = await api.get(url, { params });
          // Handle null or non-array responses
          if (!response.data || !Array.isArray(response.data)) {
            console.warn("API returned invalid response format for rooms:", response.data);
            return [];
          }
          return mapRoomsResponse(response.data as unknown as BackendRoom[]);
        } catch (error) {
          console.error("Error fetching rooms from API:", error);
          return [];
        }
      }
    } catch (error) {
      console.error("Error fetching rooms:", error);
      throw error;
    }
  },

  // Get a specific room by ID
  getRoom: async (id: string): Promise<Room> => {
    // Use the nextjs api route which handles auth token properly
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/rooms/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/rooms/${id}`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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

          // Try token refresh on 401 errors
          if (response.status === 401) {
            const refreshSuccessful = await handleAuthFailure();

            if (refreshSuccessful) {
              // Try the request again after token refresh
              const newSession = await getSession();
              const retryResponse = await fetch(url, {
                credentials: "include",
                headers: newSession?.user?.token
                  ? {
                      Authorization: `Bearer ${newSession.user.token}`,
                      "Content-Type": "application/json",
                    }
                  : undefined,
              });

              if (retryResponse.ok) {
                const data = (await retryResponse.json()) as BackendRoom;
                return mapSingleRoomResponse({ data });
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        interface RoomApiResponse {
          data?: BackendRoom;
          id?: number;
          [key: string]: unknown;
        }
        
        const responseData = await response.json() as RoomApiResponse;
        
        // Handle different response formats
        if (responseData && typeof responseData === 'object') {
          if ('data' in responseData && responseData.data) {
            // Wrapped response format with nested data property
            return mapSingleRoomResponse({ data: responseData.data });
          } else if ('id' in responseData) {
            // Direct room object without nesting
            // Convert to proper BackendRoom
            // Convert responseData to proper BackendRoom with safe type conversions
            const roomData: BackendRoom = {
              id: typeof responseData.id === 'number' ? responseData.id : 
                  typeof responseData.id === 'string' ? parseInt(responseData.id, 10) : 0,
              name: typeof responseData.name === 'string' ? responseData.name : "",
              building: typeof responseData.building === 'string' ? responseData.building : undefined,
              floor: typeof responseData.floor === 'number' ? responseData.floor :
                    typeof responseData.floor === 'string' ? parseInt(responseData.floor, 10) : 0,
              capacity: typeof responseData.capacity === 'number' ? responseData.capacity :
                        typeof responseData.capacity === 'string' ? parseInt(responseData.capacity, 10) : 0,
              category: typeof responseData.category === 'string' ? responseData.category : "",
              color: typeof responseData.color === 'string' ? responseData.color : "",
              device_id: typeof responseData.device_id === 'string' ? responseData.device_id : undefined,
              is_occupied: Boolean(responseData.is_occupied),
              activity_name: typeof responseData.activity_name === 'string' ? responseData.activity_name : undefined,
              group_name: typeof responseData.group_name === 'string' ? responseData.group_name : undefined,
              supervisor_name: typeof responseData.supervisor_name === 'string' ? responseData.supervisor_name : undefined,
              student_count: typeof responseData.student_count === 'number' ? responseData.student_count : undefined,
              created_at: typeof responseData.created_at === 'string' ? responseData.created_at : "",
              updated_at: typeof responseData.updated_at === 'string' ? responseData.updated_at : ""
            };
            return mapSingleRoomResponse({ data: roomData });
          }
        }
        
        // If nothing matched, log and return empty
        console.warn("Unexpected room response format:", responseData);
        throw new Error("Unexpected room response format");
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        
        // For axios, the response is always in response.data
        interface RoomApiResponse {
          data?: BackendRoom;
          id?: number;
          [key: string]: unknown;
        }
        
        const responseData = response.data as RoomApiResponse;
        if (responseData && typeof responseData === 'object') {
          if ('data' in responseData && responseData.data) {
            // Wrapped response format with nested data property
            return mapSingleRoomResponse({ data: responseData.data });
          } else if ('id' in responseData) {
            // Direct room object without nesting
            // Convert to proper BackendRoom
            // Convert responseData to proper BackendRoom with safe type conversions
            const roomData: BackendRoom = {
              id: typeof responseData.id === 'number' ? responseData.id : 
                  typeof responseData.id === 'string' ? parseInt(responseData.id, 10) : 0,
              name: typeof responseData.name === 'string' ? responseData.name : "",
              building: typeof responseData.building === 'string' ? responseData.building : undefined,
              floor: typeof responseData.floor === 'number' ? responseData.floor :
                    typeof responseData.floor === 'string' ? parseInt(responseData.floor, 10) : 0,
              capacity: typeof responseData.capacity === 'number' ? responseData.capacity :
                        typeof responseData.capacity === 'string' ? parseInt(responseData.capacity, 10) : 0,
              category: typeof responseData.category === 'string' ? responseData.category : "",
              color: typeof responseData.color === 'string' ? responseData.color : "",
              device_id: typeof responseData.device_id === 'string' ? responseData.device_id : undefined,
              is_occupied: Boolean(responseData.is_occupied),
              activity_name: typeof responseData.activity_name === 'string' ? responseData.activity_name : undefined,
              group_name: typeof responseData.group_name === 'string' ? responseData.group_name : undefined,
              supervisor_name: typeof responseData.supervisor_name === 'string' ? responseData.supervisor_name : undefined,
              student_count: typeof responseData.student_count === 'number' ? responseData.student_count : undefined,
              created_at: typeof responseData.created_at === 'string' ? responseData.created_at : "",
              updated_at: typeof responseData.updated_at === 'string' ? responseData.updated_at : ""
            };
            return mapSingleRoomResponse({ data: roomData });
          }
        }
        
        console.warn("Unexpected server room response format:", responseData);
        throw new Error("Unexpected room response format");
      }
    } catch (error) {
      console.error(`Error fetching room ${id}:`, error);
      throw error;
    }
  },

  // Create a new room
  createRoom: async (room: Omit<Room, "id" | "isOccupied">): Promise<Room> => {
    // Frontend validation before we transform the model
    if (!room.name) {
      throw new Error("Missing required field: name");
    }
    if (room.capacity === undefined || room.capacity <= 0) {
      throw new Error("Missing required field: capacity must be greater than 0");
    }
    if (!room.category) {
      throw new Error("Missing required field: category");
    }
    
    // Transform from frontend model to backend model
    const backendRoom = prepareRoomForBackend(room);

    // Backend model validation
    if (!backendRoom.name) {
      throw new Error("Missing required field: name");
    }

    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/rooms`
      : `${env.NEXT_PUBLIC_API_URL}/rooms`;

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
          body: JSON.stringify(backendRoom),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          // Try to parse error for more detailed message
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(`API error: ${errorJson.error}`);
            }
          } catch {
            // If parsing fails, use status code
          }
          throw new Error(`API error: ${response.status}`);
        }

        const data = (await response.json()) as BackendRoom;
        return mapSingleRoomResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendRoom);
        return mapSingleRoomResponse({ data: response.data as BackendRoom });
      }
    } catch (error) {
      console.error(`Error creating room:`, error);
      throw error;
    }
  },

  // Update a room
  updateRoom: async (id: string, room: Partial<Room>): Promise<Room> => {
    // Transform from frontend model to backend model updates
    const backendUpdates = prepareRoomForBackend(room);

    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/rooms/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/rooms/${id}`;

    try {
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
          body: JSON.stringify(backendUpdates),
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);

          // Try to parse error text as JSON for more detailed error
          try {
            const errorJson = JSON.parse(errorText) as { error?: string };
            if (errorJson.error) {
              throw new Error(
                `API error ${response.status}: ${errorJson.error}`,
              );
            }
          } catch {
            // If parsing fails, use status code + error text
            throw new Error(
              `API error ${response.status}: ${errorText.substring(0, 100)}`,
            );
          }
        }

        const data = (await response.json()) as BackendRoom;
        return mapSingleRoomResponse({ data });
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleRoomResponse({ data: response.data as BackendRoom });
      }
    } catch (error) {
      console.error(`Error updating room ${id}:`, error);
      throw error;
    }
  },

  // Delete a room
  deleteRoom: async (id: string): Promise<void> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? `/api/rooms/${id}`
      : `${env.NEXT_PUBLIC_API_URL}/rooms/${id}`;

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
      console.error(`Error deleting room ${id}:`, error);
      throw error;
    }
  },

  // Get rooms grouped by category
  getRoomsByCategory: async (): Promise<Record<string, Room[]>> => {
    const useProxyApi = typeof window !== "undefined";
    const url = useProxyApi
      ? "/api/rooms/by-category"
      : `${env.NEXT_PUBLIC_API_URL}/rooms/by-category`;

    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
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

        const data = await response.json() as Record<string, BackendRoom[]>;
        
        // Transform each category's room array
        const result: Record<string, Room[]> = {};
        for (const [category, rooms] of Object.entries(data)) {
          result[category] = mapRoomsResponse(rooms);
        }
        
        return result;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        const data = response.data as Record<string, BackendRoom[]>;
        
        // Transform each category's room array
        const result: Record<string, Room[]> = {};
        for (const [category, rooms] of Object.entries(data)) {
          result[category] = mapRoomsResponse(rooms);
        }
        
        return result;
      }
    } catch (error) {
      console.error("Error fetching rooms by category:", error);
      throw error;
    }
  },
};

export default api;
