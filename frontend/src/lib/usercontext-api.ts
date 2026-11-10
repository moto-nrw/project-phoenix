// lib/usercontext-api.ts
import { ApiError, apiErrorFromText } from "./api-error";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "UserContextAPI" });
import api from "./api";
import { fetchWithAuth } from "./fetch-with-auth";
import {
  mapEducationalGroupResponse,
  mapActiveGroupResponse,
  mapStaffResponse,
  type EducationalGroup,
  type ActiveGroup,
  type Staff,
  type BackendEducationalGroup,
  type BackendActiveGroup,
  type BackendStaff,
} from "./usercontext-helpers";

// The BFF routes read the session cookie; the browser holds no token (#5005).
const JSON_HEADERS = { "Content-Type": "application/json" };

// Generic API response interface
interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

export const userContextService = {
  // Get current user's staff profile
  getCurrentStaff: async (): Promise<Staff> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = "/api/me/staff";

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: JSON_HEADERS,
        });

        if (!response.ok) {
          const status = response.status;
          const errorText = await response.text();
          // Do not spam console for the common "not linked" case
          if (status !== 404) {
            logger.error("get current staff failed", {
              status,
              error: errorText,
            });
          }
          throw apiErrorFromText(
            `Get current staff failed: ${status}`,
            status,
            errorText,
          );
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
      if (!(error instanceof ApiError && error.status === 404)) {
        logger.error("get current staff error", { error: String(error) });
      }
      throw error;
    }
  },

  // Get educational groups for current user
  getMyEducationalGroups: async (): Promise<EducationalGroup[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = "/api/me/groups";

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: JSON_HEADERS,
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("get educational groups failed", {
            status: response.status,
            error: errorText,
          });
          throw apiErrorFromText(
            `Get educational groups failed: ${response.status}`,
            response.status,
            errorText,
          );
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
      logger.error("get educational groups error", { error: String(error) });
      throw error;
    }
  },

  // Get supervised groups for current user
  getMySupervisedGroups: async (): Promise<ActiveGroup[]> => {
    const useProxyApi = globalThis.window !== undefined;
    const url = "/api/me/groups/supervised";

    try {
      if (useProxyApi) {
        const response = await fetchWithAuth(url, {
          headers: JSON_HEADERS,
        });

        if (!response.ok) {
          const errorText = await response.text();
          logger.error("get supervised groups failed", {
            status: response.status,
            error: errorText,
          });
          throw apiErrorFromText(
            `Get supervised groups failed: ${response.status}`,
            response.status,
            errorText,
          );
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
      logger.error("get supervised groups error", { error: String(error) });
      throw error;
    }
  },
};
