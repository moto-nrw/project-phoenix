// lib/usercontext-api.ts
// BetterAuth: Authentication handled via cookies, no manual token management needed
import { env } from "~/env";
import api from "./api";
import { fetchWithAuth } from "./fetch-with-auth";
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
  // BetterAuth: authentication handled via cookies
  getCurrentUser: async (): Promise<UserProfile> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi ? "/api/me" : `${env.NEXT_PUBLIC_API_URL}/me`;

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: {
            "Content-Type": "application/json",
          },
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(
            `Get current user error: ${response.status}`,
            errorText,
          );
          throw new Error(`Get current user failed: ${response.status}`);
        }

        const responseData =
          (await response.json()) as ApiResponse<BackendUserProfile>;
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
  // BetterAuth: authentication handled via cookies
  getCurrentStaff: async (): Promise<Staff> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/me/staff"
      : `${env.NEXT_PUBLIC_API_URL}/me/staff`;

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: {
            "Content-Type": "application/json",
          },
        });

        if (!response.ok) {
          const status = response.status;
          const errorText = await response.text();
          // Do not spam console for the common "not linked" case
          if (status !== 404) {
            console.error(`Get current staff error: ${status}`, errorText);
          }
          throw new Error(`Get current staff failed: ${status}`);
        }

        const responseData =
          (await response.json()) as ApiResponse<BackendStaff>;
        return mapStaffResponse(responseData.data);
      } else {
        const response = await api.get<ApiResponse<BackendStaff>>(url);
        return mapStaffResponse(response.data.data);
      }
    } catch (error) {
      // Suppress 404 logs (account not linked to a person) to avoid noisy console
      if (
        !(
          error instanceof Error &&
          error.message.includes("Get current staff failed: 404")
        )
      ) {
        console.error("Get current staff error:", error);
      }
      throw error;
    }
  },

  // Get current user's teacher profile
  // BetterAuth: authentication handled via cookies
  getCurrentTeacher: async (): Promise<Teacher> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/me/teacher"
      : `${env.NEXT_PUBLIC_API_URL}/me/teacher`;

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: {
            "Content-Type": "application/json",
          },
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(
            `Get current teacher error: ${response.status}`,
            errorText,
          );
          throw new Error(`Get current teacher failed: ${response.status}`);
        }

        const responseData =
          (await response.json()) as ApiResponse<BackendTeacher>;
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
  // BetterAuth: authentication handled via cookies
  getMyEducationalGroups: async (): Promise<EducationalGroup[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/me/groups"
      : `${env.NEXT_PUBLIC_API_URL}/me/groups`;

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: {
            "Content-Type": "application/json",
          },
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(
            `Get educational groups error: ${response.status}`,
            errorText,
          );
          throw new Error(`Get educational groups failed: ${response.status}`);
        }

        const responseData = (await response.json()) as ApiResponse<
          BackendEducationalGroup[]
        >;
        // Handle empty or missing data
        if (!responseData.data || !Array.isArray(responseData.data)) {
          return [];
        }
        return responseData.data.map(mapEducationalGroupResponse);
      } else {
        const response =
          await api.get<ApiResponse<BackendEducationalGroup[]>>(url);
        // Handle empty or missing data
        if (!response.data.data || !Array.isArray(response.data.data)) {
          return [];
        }
        return response.data.data.map(mapEducationalGroupResponse);
      }
    } catch (error) {
      console.error("Get educational groups error:", error);
      throw error;
    }
  },

  // Get activity groups for current user
  // BetterAuth: authentication handled via cookies
  getMyActivityGroups: async (): Promise<ActivityGroup[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/me/groups/activity"
      : `${env.NEXT_PUBLIC_API_URL}/me/groups/activity`;

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: {
            "Content-Type": "application/json",
          },
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(
            `Get activity groups error: ${response.status}`,
            errorText,
          );
          throw new Error(`Get activity groups failed: ${response.status}`);
        }

        const responseData = (await response.json()) as ApiResponse<
          BackendActivityGroup[]
        >;
        return responseData.data.map(mapActivityGroupResponse);
      } else {
        const response =
          await api.get<ApiResponse<BackendActivityGroup[]>>(url);
        return response.data.data.map(mapActivityGroupResponse);
      }
    } catch (error) {
      console.error("Get activity groups error:", error);
      throw error;
    }
  },

  // Get active groups for current user
  // BetterAuth: authentication handled via cookies
  getMyActiveGroups: async (): Promise<ActiveGroup[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/me/groups/active"
      : `${env.NEXT_PUBLIC_API_URL}/me/groups/active`;

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: {
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

        const responseData = (await response.json()) as ApiResponse<
          BackendActiveGroup[] | null
        >;
        const groups = Array.isArray(responseData.data)
          ? responseData.data
          : [];
        return groups.map(mapActiveGroupResponse);
      } else {
        const response =
          await api.get<ApiResponse<BackendActiveGroup[] | null>>(url);
        const groups = Array.isArray(response.data.data)
          ? response.data.data
          : [];
        return groups.map(mapActiveGroupResponse);
      }
    } catch (error) {
      console.error("Get active groups error:", error);
      throw error;
    }
  },

  // Get supervised groups for current user
  // BetterAuth: authentication handled via cookies
  getMySupervisedGroups: async (): Promise<ActiveGroup[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = useProxyApi
      ? "/api/me/groups/supervised"
      : `${env.NEXT_PUBLIC_API_URL}/me/groups/supervised`;

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: {
            "Content-Type": "application/json",
          },
        });

        if (!response.ok) {
          const errorText = await response.text();
          console.error(
            `Get supervised groups error: ${response.status}`,
            errorText,
          );
          throw new Error(`Get supervised groups failed: ${response.status}`);
        }

        const responseData = (await response.json()) as ApiResponse<
          BackendActiveGroup[] | null
        >;
        const groups = Array.isArray(responseData.data)
          ? responseData.data
          : [];
        return groups.map(mapActiveGroupResponse);
      } else {
        const response =
          await api.get<ApiResponse<BackendActiveGroup[] | null>>(url);
        const groups = Array.isArray(response.data.data)
          ? response.data.data
          : [];
        return groups.map(mapActiveGroupResponse);
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
