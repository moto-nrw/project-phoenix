// Staff API service for fetching all staff members and their supervision status

import { getSession } from "next-auth/react";

// Backend response types (already mapped by the API route handler)
export interface BackendStaffResponse {
  id: string;
  name: string;
  firstName: string;
  lastName: string;
  specialization: string;
  role: string | null;
  qualifications: string | null;
  tag_id: string | null;
  staff_notes: string | null;
  created_at: string;
  updated_at: string;
  staff_id?: string;
  teacher_id?: string;
}

export interface ActiveSupervisionResponse {
  id: number;
  staff_id: number;
  group_id: number;
  role: string;
  start_date: string;
  end_date?: string;
  is_active: boolean;
  active_group?: {
    id: number;
    name: string;
    room_id?: number;
    room?: {
      id: number;
      name: string;
    };
  };
}

// Frontend types
export interface Staff {
  id: string;
  name: string;
  firstName: string;
  lastName: string;
  email?: string;
  specialization?: string;
  qualifications?: string;
  staffNotes?: string;
  hasRfid: boolean;
  isTeacher: boolean;
  // Supervision status
  isSupervising: boolean;
  currentLocation?: string;
  currentRoomId?: string;
  supervisionRole?: string;
}

export interface StaffFilters {
  search?: string;
  status?: 'all' | 'supervising' | 'available';
  type?: 'all' | 'teachers' | 'staff';
}

// Staff service
class StaffService {
  // Get all staff members with their current supervision status
  async getAllStaff(filters?: StaffFilters): Promise<Staff[]> {
    try {
      const session = await getSession();
      const token = session?.user?.token;

      if (!token) {
        throw new Error("No authentication token available");
      }

      // Fetch all staff members
      let staffUrl = "/api/staff";
      const queryParams = new URLSearchParams();
      
      if (filters?.search) {
        queryParams.append("search", filters.search);
      }

      if (queryParams.toString()) {
        staffUrl += `?${queryParams.toString()}`;
      }

      const staffResponse = await fetch(staffUrl, {
        credentials: "include",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
      });

      if (!staffResponse.ok) {
        throw new Error(`Failed to fetch staff: ${staffResponse.statusText}`);
      }

      const staffData = await staffResponse.json() as BackendStaffResponse[] | { data: BackendStaffResponse[] };
      
      // Handle different response formats
      let staffList: BackendStaffResponse[] = [];
      if (Array.isArray(staffData)) {
        staffList = staffData;
      } else if (staffData && typeof staffData === 'object' && 'data' in staffData && Array.isArray(staffData.data)) {
        staffList = staffData.data;
      }

      // Fetch active groups once for all staff
      let activeGroups: Array<{
        supervisors?: Array<{
          staff_id?: number;
          role?: string;
        }>;
        room?: {
          id: number;
          name: string;
        };
      }> = [];

      try {
        const activeGroupsUrl = `/api/active/groups?active=true`;
        const activeGroupsResponse = await fetch(activeGroupsUrl, {
          credentials: "include",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
        });

        if (activeGroupsResponse.ok) {
          const activeGroupsData = await activeGroupsResponse.json() as unknown;
          
          // The frontend route handler wraps the backend response, so we need to unwrap it
          // Backend returns: { status: "success", data: [...], message: "..." }
          // Frontend wrapper makes it: { success: true, data: { status: "success", data: [...] } }
          if (activeGroupsData && typeof activeGroupsData === 'object' && 'data' in activeGroupsData) {
            const wrappedData = (activeGroupsData as { data?: unknown }).data;
            if (wrappedData && typeof wrappedData === 'object' && 'data' in wrappedData) {
              // Double wrapped - frontend wrapper around backend response
              const backendResponse = wrappedData as { data?: unknown };
              if (Array.isArray(backendResponse.data)) {
                activeGroups = backendResponse.data as typeof activeGroups;
              }
            } else if (Array.isArray(wrappedData)) {
              // Single wrapped - just frontend wrapper
              activeGroups = wrappedData as typeof activeGroups;
            }
          } else if (Array.isArray(activeGroupsData)) {
            // Direct array response (shouldn't happen with our setup)
            activeGroups = activeGroupsData as typeof activeGroups;
          }
        }
      } catch {
        // Continue with empty active groups if fetch fails
      }

      // Build a map of staff_id to their supervised groups for O(1) lookup
      const staffGroupsMap: Record<string, Array<{ group: typeof activeGroups[0]; role?: string }>> = {};
      
      for (const group of activeGroups) {
        const supervisors = group.supervisors ?? [];
        for (const supervisor of supervisors) {
          if (supervisor.staff_id !== undefined) {
            const staffIdStr = supervisor.staff_id.toString();
            staffGroupsMap[staffIdStr] ??= [];
            staffGroupsMap[staffIdStr].push({ group, role: supervisor.role });
          }
        }
      }

      // Map staff and check their supervision status
      const mappedStaff = staffList.map((staff): Staff => {
          let currentLocation: string | undefined = "Zuhause"; // Default to "Zuhause" (at home)
          let isSupervising = false;
          let supervisionRole: string | undefined;

          // Check if this staff member is supervising any active group
          const supervisedRooms: string[] = [];
          if (staff.staff_id) {
            const supervisedGroups = staffGroupsMap[staff.staff_id];
            if (supervisedGroups) {
              isSupervising = true;
              
              for (const { group, role } of supervisedGroups) {
                if (group.room) {
                  supervisedRooms.push(group.room.name);
                }
                supervisionRole ??= role;
              }
              
              // Set location based on supervised rooms
              if (supervisedRooms.length > 1) {
                currentLocation = `${supervisedRooms.length} Räume`;
              } else if (supervisedRooms.length === 1) {
                currentLocation = supervisedRooms[0];
              } else {
                currentLocation = "Unterwegs";
              }
            }
          }

          return {
            id: staff.id,
            name: staff.name,
            firstName: staff.firstName,
            lastName: staff.lastName,
            email: undefined, // Not provided by API route handler
            specialization: staff.specialization,
            qualifications: staff.qualifications ?? undefined,
            staffNotes: staff.staff_notes ?? undefined,
            hasRfid: !!staff.tag_id,
            isTeacher: !!staff.teacher_id, // Has teacher_id means is teacher
            isSupervising,
            currentLocation,
            currentRoomId: undefined,
            supervisionRole,
          };
        });

      // Apply client-side filters
      let filteredStaff = mappedStaff;

      if (filters?.status && filters.status !== 'all') {
        filteredStaff = filteredStaff.filter(staff => {
          if (filters.status === 'supervising') {
            return staff.isSupervising;
          } else if (filters.status === 'available') {
            return !staff.isSupervising;
          }
          return true;
        });
      }

      if (filters?.type && filters.type !== 'all') {
        filteredStaff = filteredStaff.filter(staff => {
          if (filters.type === 'teachers') {
            return staff.isTeacher;
          } else if (filters.type === 'staff') {
            return !staff.isTeacher;
          }
          return true;
        });
      }

      return filteredStaff;
    } catch (error) {
      console.error("Error fetching staff:", error);
      throw error;
    }
  }

  // Get active supervisions for a specific staff member
  async getStaffSupervisions(staffId: string): Promise<ActiveSupervisionResponse[]> {
    try {
      const session = await getSession();
      const token = session?.user?.token;

      if (!token) {
        throw new Error("No authentication token available");
      }

      const response = await fetch(`/api/active/supervisors/staff/${staffId}/active`, {
        credentials: "include",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
      });

      if (!response.ok) {
        throw new Error(`Failed to fetch staff supervisions: ${response.statusText}`);
      }

      const data = await response.json() as ActiveSupervisionResponse[] | { data: ActiveSupervisionResponse[] };
      
      if (Array.isArray(data)) {
        return data;
      } else if (data && typeof data === 'object' && 'data' in data && Array.isArray(data.data)) {
        return data.data;
      }

      return [];
    } catch (error) {
      console.error(`Error fetching supervisions for staff ${staffId}:`, error);
      return [];
    }
  }
}

export const staffService = new StaffService();