// lib/group-transfer-api.ts
// API client for group transfer operations

import { sessionFetch } from "./session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GroupTransferAPI" });

// Staff member with role info for dropdown
export interface StaffWithRole {
  id: string;
  fullName: string;
}

export interface StaffGroupLeaderCandidate extends StaffWithRole {
  teacherId?: string;
}

// Transfer info
export interface GroupTransfer {
  substitutionId: string;
  groupId: string;
  targetStaffId: string;
  targetName: string;
  validUntil: string;
}

// Backend response for staff by role
interface BackendHandoverTarget {
  id: number;
  full_name: string;
}

interface BackendStaffGroupLeaderCandidate {
  id: number;
  teacher_id?: number;
  full_name: string;
}

// Map backend response to frontend type
function mapStaffWithRole(data: BackendHandoverTarget): StaffWithRole {
  return {
    id: data.id.toString(),
    fullName: data.full_name,
  };
}

export const groupTransferService = {
  // Get all staff members available for group transfer
  // Uses the canonical caregiver pool so admin-only staff never appear here.
  async getAllAvailableStaff(): Promise<StaffWithRole[]> {
    try {
      const response = await sessionFetch(`/api/substitutions`, {
        method: "GET",
      });

      if (!response.ok) {
        throw new Error("Fachkräfte konnten nicht geladen werden.");
      }

      const data = (await response.json()) as {
        targets?: BackendHandoverTarget[];
      };

      if (!Array.isArray(data.targets)) {
        throw new Error("Ungültige Antwort für verfügbare Fachkräfte.");
      }

      return data.targets.map(mapStaffWithRole);
    } catch (error) {
      logger.error("fetch_all_available_staff_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },

  // Group-leader replacement is a separate admin workflow. Keep only the
  // staff/teacher identifiers and display name from its broader endpoint.
  async getStaffByRole(role: string): Promise<StaffGroupLeaderCandidate[]> {
    const response = await sessionFetch(
      `/api/staff/by-role?role=${encodeURIComponent(role)}`,
      { method: "GET" },
    );
    if (!response.ok) {
      throw new Error("Fachkräfte konnten nicht geladen werden.");
    }
    const data = (await response.json()) as {
      data: BackendStaffGroupLeaderCandidate[] | null;
    };
    if (!Array.isArray(data.data)) {
      throw new Error("Ungültige Antwort für verfügbare Fachkräfte.");
    }
    return data.data.map((staff) => ({
      id: staff.id.toString(),
      teacherId: staff.teacher_id?.toString(),
      fullName: staff.full_name,
    }));
  },

  // Hand over responsibility for a group until the end of the Berlin school day.
  async transferGroup(groupId: string, targetStaffId: string): Promise<void> {
    try {
      const response = await sessionFetch(`/api/substitutions`, {
        method: "POST",
        body: JSON.stringify({
          type: "group_handover",
          group_handover: {
            group_id: Number.parseInt(groupId, 10),
            target_staff_id: Number.parseInt(targetStaffId, 10),
          },
        }),
      });

      if (!response.ok) {
        const errorData = (await response.json()) as {
          status?: string;
          error?: string;
        };
        // Extract clean error message from backend
        const errorMessage = errorData.error ?? `Transfer fehlgeschlagen`;
        // Create custom error with backend message (don't log, let caller handle it)
        const error = new Error(errorMessage);
        error.name = "TransferError";
        throw error;
      }
    } catch (error) {
      // Only log if it's NOT our custom error (unexpected errors only)
      if (error instanceof Error && error.name !== "TransferError") {
        logger.error("unexpected error transferring group", {
          group_id: groupId,
          error: String(error),
        });
      }
      throw error;
    }
  },

  // Get all active transfers for a group (from substitutions)
  async getActiveTransfersForGroup(groupId: string): Promise<GroupTransfer[]> {
    try {
      const response = await sessionFetch(
        `/api/substitutions?group_id=${encodeURIComponent(groupId)}`,
        { method: "GET" },
      );

      if (!response.ok) {
        const errorData = (await response.json()) as { error?: string };
        const error = new Error(
          errorData.error ?? "Gruppenübergaben konnten nicht geladen werden.",
        );
        error.name = "FetchGroupTransfersError";
        throw error;
      }

      const responseData = (await response.json()) as {
        group_handovers: Array<{
          id: number;
          group: { id: number };
          target: { id: number; full_name: string };
          period: { end_date: string };
        }>;
      };

      return responseData.group_handovers.map((handover) => ({
        substitutionId: handover.id.toString(),
        groupId: handover.group.id.toString(),
        targetStaffId: handover.target.id.toString(),
        targetName: handover.target.full_name,
        validUntil: handover.period.end_date,
      }));
    } catch (error) {
      if (
        !(error instanceof Error) ||
        error.name !== "FetchGroupTransfersError"
      ) {
        logger.error("unexpected error getting active transfers", {
          group_id: groupId,
          error: String(error),
        });
      }
      throw error;
    }
  },

  // Delete a specific transfer by substitution ID (with ownership check)
  async cancelTransferBySubstitutionId(substitutionId: string): Promise<void> {
    try {
      const response = await sessionFetch(`/api/substitutions/end`, {
        method: "POST",
        body: JSON.stringify({
          type: "group_handover",
          id: Number.parseInt(substitutionId, 10),
        }),
      });

      if (!response.ok) {
        const errorData = (await response.json()) as {
          status?: string;
          error?: string;
        };
        const errorMessage = errorData.error ?? `Löschen fehlgeschlagen`;
        const error = new Error(errorMessage);
        error.name = "CancelTransferError";
        throw error;
      }
    } catch (error) {
      // Only log unexpected errors
      if (error instanceof Error && error.name !== "CancelTransferError") {
        logger.error("unexpected error cancelling transfer", {
          substitution_id: substitutionId,
          error: String(error),
        });
      }
      throw error;
    }
  },
};
