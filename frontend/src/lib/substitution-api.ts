// lib/substitution-api.ts
// API client for substitution-related operations

import { sessionFetch } from "./session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SubstitutionAPI" });
import {
  type Substitution,
  type TeacherAvailability,
  type BackendSubstitution,
  type BackendStaffWithSubstitutionStatus,
  mapSubstitutionResponse,
  mapSubstitutionsResponse,
  mapTeacherAvailabilityResponses,
  prepareSubstitutionForBackend,
  formatDateForBackend,
} from "./substitution-helpers";

// Substitution service with API methods
class SubstitutionService {
  // Get all substitutions with optional filters
  async fetchSubstitutions(filters?: {
    page?: number;
    pageSize?: number;
  }): Promise<Substitution[]> {
    try {
      let url = "/api/substitutions";

      if (filters) {
        const params = new URLSearchParams();
        if (filters.page) params.append("page", filters.page.toString());
        if (filters.pageSize)
          params.append("page_size", filters.pageSize.toString());

        if (params.toString()) {
          url += `?${params.toString()}`;
        }
      }

      const response = await sessionFetch(url, {
        credentials: "include",
      });

      if (!response.ok) {
        throw new Error(
          `Failed to fetch substitutions: ${response.statusText}`,
        );
      }

      const data = (await response.json()) as { data: unknown } | unknown[];

      // Handle response format - could be wrapped or direct array
      if (Array.isArray(data)) {
        return mapSubstitutionsResponse(data as BackendSubstitution[]);
      } else if (data && typeof data === "object" && "data" in data) {
        return mapSubstitutionsResponse(data.data as BackendSubstitution[]);
      } else {
        logger.error("unexpected response format for substitutions");
        return [];
      }
    } catch (error) {
      logger.error("error fetching substitutions", { error: String(error) });
      throw error;
    }
  }

  // Get active substitutions for a specific date
  async fetchActiveSubstitutions(date?: Date): Promise<Substitution[]> {
    try {
      let url = "/api/substitutions/active";

      if (date) {
        const params = new URLSearchParams();
        params.append("date", formatDateForBackend(date));
        url += `?${params.toString()}`;
      }

      const response = await sessionFetch(url, {
        credentials: "include",
      });

      if (!response.ok) {
        throw new Error(
          `Failed to fetch active substitutions: ${response.statusText}`,
        );
      }

      const data = (await response.json()) as { data: unknown } | unknown[];

      if (Array.isArray(data)) {
        return mapSubstitutionsResponse(data as BackendSubstitution[]);
      } else if (data && typeof data === "object" && "data" in data) {
        return mapSubstitutionsResponse(data.data as BackendSubstitution[]);
      } else {
        logger.error("unexpected response format for active substitutions");
        return [];
      }
    } catch (error) {
      logger.error("error fetching active substitutions", {
        error: String(error),
      });
      throw error;
    }
  }

  // Get available teachers with their substitution status
  async fetchAvailableTeachers(
    date?: Date,
    search?: string,
  ): Promise<TeacherAvailability[]> {
    try {
      let url = "/api/staff/available-for-substitution";
      const params = new URLSearchParams();

      if (date) {
        params.append("date", formatDateForBackend(date));
      }
      if (search) {
        params.append("search", search);
      }

      if (params.toString()) {
        url += `?${params.toString()}`;
      }

      const response = await sessionFetch(url, {
        credentials: "include",
      });

      if (!response.ok) {
        throw new Error(
          `Failed to fetch available teachers: ${response.statusText}`,
        );
      }

      const data = (await response.json()) as { data: unknown } | unknown[];

      if (Array.isArray(data)) {
        return mapTeacherAvailabilityResponses(
          data as BackendStaffWithSubstitutionStatus[],
        );
      } else if (data && typeof data === "object" && "data" in data) {
        return mapTeacherAvailabilityResponses(
          data.data as BackendStaffWithSubstitutionStatus[],
        );
      } else {
        logger.error("unexpected response format for available teachers");
        return [];
      }
    } catch (error) {
      logger.error("error fetching available teachers", {
        error: String(error),
      });
      throw error;
    }
  }

  // Create a new substitution
  async createSubstitution(
    groupId: string,
    regularStaffId: string | null, // Now optional - null for general coverage
    substituteStaffId: string,
    startDate: Date,
    endDate: Date,
    reason?: string,
    notes?: string,
  ): Promise<Substitution> {
    try {
      const requestData = prepareSubstitutionForBackend(
        groupId,
        regularStaffId,
        substituteStaffId,
        startDate,
        endDate,
        reason,
        notes,
      );

      const response = await sessionFetch("/api/substitutions", {
        method: "POST",
        credentials: "include",
        body: JSON.stringify(requestData),
      });

      if (!response.ok) {
        const errorData = (await response.json()) as {
          error?: string;
          message?: string;
        };
        const errorMessage =
          errorData.error ?? errorData.message ?? response.statusText;
        throw new Error(`Failed to create substitution: ${errorMessage}`);
      }

      const data = (await response.json()) as unknown;

      if (data && typeof data === "object" && "data" in data) {
        return mapSubstitutionResponse(
          (data as { data: BackendSubstitution }).data,
        );
      } else {
        return mapSubstitutionResponse(data as BackendSubstitution);
      }
    } catch (error) {
      logger.error("error creating substitution", {
        group_id: groupId,
        error: String(error),
      });
      throw error;
    }
  }

  // Delete/end a substitution
  async deleteSubstitution(id: string): Promise<void> {
    try {
      const response = await sessionFetch(`/api/substitutions/${id}`, {
        method: "DELETE",
        credentials: "include",
      });

      if (!response.ok) {
        throw new Error(
          `Failed to delete substitution: ${response.statusText}`,
        );
      }
    } catch (error) {
      logger.error("error deleting substitution", {
        substitution_id: id,
        error: String(error),
      });
      throw error;
    }
  }

  // Get a single substitution by ID
  async getSubstitution(id: string): Promise<Substitution> {
    try {
      const response = await sessionFetch(`/api/substitutions/${id}`, {
        credentials: "include",
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch substitution: ${response.statusText}`);
      }

      const data = (await response.json()) as unknown;

      if (data && typeof data === "object" && "data" in data) {
        return mapSubstitutionResponse(
          (data as { data: BackendSubstitution }).data,
        );
      } else {
        return mapSubstitutionResponse(data as BackendSubstitution);
      }
    } catch (error) {
      logger.error("error fetching substitution", {
        substitution_id: id,
        error: String(error),
      });
      throw error;
    }
  }
}

export const substitutionService = new SubstitutionService();
