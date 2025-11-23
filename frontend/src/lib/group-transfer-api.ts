// lib/group-transfer-api.ts
// API client for group transfer operations

import { getSession } from "next-auth/react";

// Staff member with role info for dropdown
export interface StaffWithRole {
  id: string;
  personId: string;
  firstName: string;
  lastName: string;
  fullName: string;
  accountId: string;
  email: string;
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
interface BackendStaffWithRole {
  id: number;
  person_id: number;
  first_name: string;
  last_name: string;
  full_name: string;
  account_id: number;
  email: string;
}

// Map backend response to frontend type
function mapStaffWithRole(data: BackendStaffWithRole): StaffWithRole {
  return {
    id: data.id.toString(),
    personId: data.person_id.toString(),
    firstName: data.first_name,
    lastName: data.last_name,
    fullName: data.full_name,
    accountId: data.account_id.toString(),
    email: data.email,
  };
}

export const groupTransferService = {
  // Get staff members with a specific role (for dropdown)
  async getStaffByRole(role: string): Promise<StaffWithRole[]> {
    try {
      const session = await getSession();
      const response = await fetch(`/api/staff/by-role?role=${role}`, {
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        throw new Error(
          `Failed to fetch staff by role: ${response.statusText}`,
        );
      }

      const data = (await response.json()) as {
        data: BackendStaffWithRole[] | null;
      };

      if (!data.data || !Array.isArray(data.data)) {
        return [];
      }

      return data.data.map(mapStaffWithRole);
    } catch (error) {
      console.error("Error fetching staff by role:", error);
      throw error;
    }
  },

  // Transfer group to another user (until end of day)
  async transferGroup(groupId: string, targetPersonId: string): Promise<void> {
    try {
      const session = await getSession();
      const response = await fetch(`/api/groups/${groupId}/transfer`, {
        method: "POST",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
        body: JSON.stringify({ target_user_id: parseInt(targetPersonId, 10) }),
      });

      if (!response.ok) {
        const errorData = (await response.json()) as { error?: string };
        throw new Error(
          errorData.error ?? `Transfer failed: ${response.statusText}`,
        );
      }
    } catch (error) {
      console.error("Error transferring group:", error);
      throw error;
    }
  },

  // Cancel group transfer
  async cancelTransfer(groupId: string): Promise<void> {
    try {
      const session = await getSession();
      const response = await fetch(`/api/groups/${groupId}/transfer`, {
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
        const errorData = (await response.json()) as { error?: string };
        throw new Error(
          errorData.error ?? `Cancel transfer failed: ${response.statusText}`,
        );
      }
    } catch (error) {
      console.error("Error cancelling transfer:", error);
      throw error;
    }
  },

  // Get all active transfers for a group (from substitutions)
  async getActiveTransfersForGroup(groupId: string): Promise<GroupTransfer[]> {
    try {
      const session = await getSession();
      const response = await fetch(`/api/groups/${groupId}/substitutions`, {
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });

      if (!response.ok) {
        // Return empty array instead of throwing if not found
        return [];
      }

      const data = (await response.json()) as {
        data: Array<{
          id: number;
          group_id: number;
          regular_staff_id: number | null;
          substitute_staff_id: number;
          substitute_staff?: {
            person?: {
              first_name: string;
              last_name: string;
            };
          };
          start_date: string;
          end_date: string;
        }> | null;
      };

      if (!data.data || !Array.isArray(data.data)) {
        return [];
      }

      // Find ALL transfers (regular_staff_id IS NULL)
      const transfers = data.data.filter(
        (sub) => sub.regular_staff_id === null,
      );

      return transfers.map((transfer) => {
        const targetName = transfer.substitute_staff?.person
          ? `${transfer.substitute_staff.person.first_name} ${transfer.substitute_staff.person.last_name}`
          : "Unbekannt";

        return {
          substitutionId: transfer.id.toString(),
          groupId: transfer.group_id.toString(),
          targetStaffId: transfer.substitute_staff_id.toString(),
          targetName,
          validUntil: transfer.end_date,
        };
      });
    } catch (error) {
      console.error("Error getting active transfers:", error);
      return [];
    }
  },

  // Delete a specific transfer by substitution ID
  async deleteTransferById(substitutionId: string): Promise<void> {
    try {
      const session = await getSession();
      const response = await fetch(`/api/substitutions/${substitutionId}`, {
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
        const errorData = (await response.json()) as { error?: string };
        throw new Error(
          errorData.error ?? `Delete transfer failed: ${response.statusText}`,
        );
      }
    } catch (error) {
      console.error("Error deleting transfer:", error);
      throw error;
    }
  },
};
