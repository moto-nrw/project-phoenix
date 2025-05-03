import axios from "axios";
import type { AxiosError, AxiosRequestConfig, AxiosResponse } from "axios";
import { getSession } from "next-auth/react";
import { env } from "~/env";
import {
  mapSingleStudentResponse,
  mapStudentResponse,
  prepareStudentForBackend,
} from "./student-helpers";
import type { BackendStudent } from "./student-helpers";
import {
  mapSingleGroupResponse,
  mapGroupResponse,
  prepareGroupForBackend,
  mapSingleCombinedGroupResponse,
  mapCombinedGroupResponse,
  prepareCombinedGroupForBackend,
} from "./group-helpers";
import type { BackendGroup, BackendCombinedGroup } from "./group-helpers";
import {
  mapSingleRoomResponse,
  mapRoomResponse,
  prepareRoomForBackend,
} from "./room-helpers";
import type { BackendRoom } from "./room-helpers";
import { handleAuthFailure } from "./auth-api";

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
          window.location.href = "/login";
        }
      }
    }

    return Promise.reject(error);
  },
);

// API interfaces
export interface Student {
  id: string;
  name: string; // Derived from CustomUser's FirstName + SecondName
  first_name?: string; // FirstName in CustomUser record
  second_name?: string; // SecondName in CustomUser record
  school_class: string; // From backend's SchoolClass field
  grade?: string; // For frontend display/form use
  studentId?: string; // For frontend display/form use
  group_name?: string; // From related Group
  group_id?: string; // ID of the related Group
  in_house: boolean; // Current location status
  wc?: boolean; // Bathroom status
  school_yard?: boolean; // School yard status
  bus?: boolean; // Bus status
  name_lg?: string; // Legal Guardian name
  contact_lg?: string; // Legal Guardian contact
  custom_users_id?: string; // ID of the related CustomUser
}

// Group-related interfaces
export interface Group {
  id: string;
  name: string;
  room_id?: string;
  room_name?: string;
  representative_id?: string;
  representative_name?: string;
  student_count?: number;
  supervisor_count?: number;
  created_at?: string;
  updated_at?: string;
  students?: Student[];
  supervisors?: Array<{ id: string; name: string }>;
}

// CombinedGroup interface for temporary group combinations
export interface CombinedGroup {
  id: string;
  name: string;
  is_active: boolean;
  created_at?: string;
  valid_until?: string;
  access_policy: "all" | "first" | "specific" | "manual";
  specific_group_id?: string;
  specific_group?: Group;
  groups?: Group[];
  access_specialists?: Array<{ id: string; name: string }>;
  is_expired?: boolean;
  group_count?: number;
  specialist_count?: number;
  time_until_expiration?: string;
}

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
                const responseData: unknown = await retryResponse.json();
                return mapStudentResponse(responseData as BackendStudent[]);
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        const responseData: unknown = await response.json();
        return mapStudentResponse(responseData as BackendStudent[]);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url, { params });
        return mapStudentResponse(response.data as unknown as BackendStudent[]);
      }
    } catch (error) {
      console.error("Error fetching students:", error);
      throw error;
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
                return mapSingleStudentResponse(data as BackendStudent);
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        const data: unknown = await response.json();
        // Map response to our frontend model
        const mappedResponse = mapSingleStudentResponse(data as BackendStudent);
        return mappedResponse;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapSingleStudentResponse(
          response.data as unknown as BackendStudent,
        );
      }
    } catch (error) {
      console.error(`Error fetching student ${id}:`, error);
      throw error;
    }
  },

  // Create a new student
  createStudent: async (student: Omit<Student, "id">): Promise<Student> => {
    // Transform from frontend model to backend model
    const backendStudent = prepareStudentForBackend(student);

    // Basic validation for student creation
    if (!backendStudent.school_class) {
      throw new Error("Missing required field: school_class");
    }
    if (!backendStudent.first_name) {
      throw new Error("Missing required field: first_name");
    }
    if (!backendStudent.second_name) {
      throw new Error("Missing required field: second_name");
    }
    // Ensure group_id is set (defaults to 1 if not provided)
    backendStudent.group_id ??= 1;

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
        const mappedResponse = mapSingleStudentResponse(data as BackendStudent);
        return mappedResponse;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendStudent);
        return mapSingleStudentResponse(
          response.data as unknown as BackendStudent,
        );
      }
    } catch (error) {
      console.error(`Error creating student:`, error);
      throw error;
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

    // Additional validation before sending to API for updates
    if (!backendUpdates.custom_users_id) {
      throw new Error("Missing required field: custom_users_id");
    }
    // Other validations that apply to both updates and creates
    if (!backendUpdates.group_id) {
      throw new Error("Missing required field: group_id");
    }
    if (!backendUpdates.school_class) {
      throw new Error("Missing required field: school_class");
    }

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
        const mappedResponse = mapSingleStudentResponse(data as BackendStudent);
        return mappedResponse;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        // Merge the returned data with our local name changes if provided
        const mappedResponse = mapSingleStudentResponse(
          response.data as unknown as BackendStudent,
        );
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
      console.error(`Error updating student ${id}:`, error);
      throw error;
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
      console.error(`Error deleting student ${id}:`, error);
      throw error;
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
                const responseData: unknown = await retryResponse.json();
                return mapGroupResponse(responseData as BackendGroup[]);
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        const responseData: unknown = await response.json();
        return mapGroupResponse(responseData as BackendGroup[]);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url, { params });
        return mapGroupResponse(response.data as unknown as BackendGroup[]);
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
                const data = (await retryResponse.json()) as BackendGroup;
                return mapSingleGroupResponse(data);
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        const data = (await response.json()) as BackendGroup;
        return mapSingleGroupResponse(data);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapSingleGroupResponse(response.data as BackendGroup);
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
        return mapSingleGroupResponse(data);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendGroup);
        return mapSingleGroupResponse(response.data as BackendGroup);
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
        return mapSingleGroupResponse(data);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleGroupResponse(response.data as BackendGroup);
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
        const responseData: unknown = await response.json();
        return mapStudentResponse(responseData as BackendStudent[]);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapStudentResponse(response.data as BackendStudent[]);
      }
    } catch (error) {
      console.error(`Error fetching students for group ${id}:`, error);
      throw error;
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
        return mapCombinedGroupResponse(responseData);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapCombinedGroupResponse(
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
        return mapSingleCombinedGroupResponse(responseData);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapSingleCombinedGroupResponse(
          response.data as BackendCombinedGroup,
        );
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
        return mapSingleCombinedGroupResponse(responseData);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendCombinedGroup);
        return mapSingleCombinedGroupResponse(
          response.data as BackendCombinedGroup,
        );
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
        return mapSingleCombinedGroupResponse(responseData);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleCombinedGroupResponse(
          response.data as BackendCombinedGroup,
        );
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
                // Type assertion to avoid unsafe assignment
                const responseData: unknown = await retryResponse.json();
                return mapRoomResponse(responseData as BackendRoom[]);
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        // Type assertion to avoid unsafe assignment
        const responseData: unknown = await response.json();
        return mapRoomResponse(responseData as BackendRoom[]);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url, { params });
        return mapRoomResponse(response.data as unknown as BackendRoom[]);
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
                return mapSingleRoomResponse(data);
              }
            }
          }

          throw new Error(`API error: ${response.status}`);
        }

        const data = (await response.json()) as BackendRoom;
        return mapSingleRoomResponse(data);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        return mapSingleRoomResponse(response.data as BackendRoom);
      }
    } catch (error) {
      console.error(`Error fetching room ${id}:`, error);
      throw error;
    }
  },

  // Create a new room
  createRoom: async (room: Omit<Room, "id" | "isOccupied">): Promise<Room> => {
    // Transform from frontend model to backend model
    const backendRoom = prepareRoomForBackend(room);

    // Basic validation for room creation
    if (!backendRoom.room_name) {
      throw new Error("Missing required field: room_name");
    }
    if (backendRoom.capacity === undefined || backendRoom.capacity <= 0) {
      throw new Error("Missing required field: capacity must be greater than 0");
    }
    if (!backendRoom.category) {
      throw new Error("Missing required field: category");
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
        return mapSingleRoomResponse(data);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.post(url, backendRoom);
        return mapSingleRoomResponse(response.data as BackendRoom);
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
        return mapSingleRoomResponse(data);
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.put(url, backendUpdates);
        return mapSingleRoomResponse(response.data as BackendRoom);
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
          result[category] = mapRoomResponse(rooms);
        }
        
        return result;
      } else {
        // Server-side: use axios with the API URL directly
        const response = await api.get(url);
        const data = response.data as Record<string, BackendRoom[]>;
        
        // Transform each category's room array
        const result: Record<string, Room[]> = {};
        for (const [category, rooms] of Object.entries(data)) {
          result[category] = mapRoomResponse(rooms);
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
