import type { Guardian, StudentGuardian } from "./guardian-helpers";
import {
  mapGuardianResponse,
  mapStudentGuardianResponse,
  mapGuardianRequest,
  mapStudentGuardianRequest,
} from "./guardian-helpers";

export interface GuardianFilters {
  active?: boolean;
  email?: string;
  phone?: string;
  country?: string;
  page?: number;
  per_page?: number;
  sort?: string;
  order?: "asc" | "desc";
}

export interface GuardianListResponse {
  data: Guardian[];
  total: number;
  page: number;
  per_page: number;
}

export const guardianService = {
  // Guardian CRUD
  async listGuardians(filters?: GuardianFilters): Promise<GuardianListResponse> {
    const params = new URLSearchParams();
    if (filters?.active !== undefined) params.append("active", String(filters.active));
    if (filters?.email) params.append("email", filters.email);
    if (filters?.phone) params.append("phone", filters.phone);
    if (filters?.country) params.append("country", filters.country);
    if (filters?.page) params.append("page", String(filters.page));
    if (filters?.per_page) params.append("per_page", String(filters.per_page));
    if (filters?.sort) params.append("sort", filters.sort);
    if (filters?.order) params.append("order", filters.order);

    const url = `/api/guardians${params.toString() ? `?${params.toString()}` : ""}`;
    const response = await fetch(url);

    if (!response.ok) {
      throw new Error(`Failed to fetch guardians: ${response.statusText}`);
    }

    // Unwrap the Next.js route wrapper: { success, message, data }
    const wrapper = (await response.json()) as {
      data: {
        data: unknown[];
        total: number;
        page: number;
        per_page: number;
      };
    };

    return {
      data: wrapper.data.data.map((item) => mapGuardianResponse(item as never)),
      total: wrapper.data.total,
      page: wrapper.data.page,
      per_page: wrapper.data.per_page,
    };
  },

  async getGuardian(id: string): Promise<Guardian> {
    const response = await fetch(`/api/guardians/${id}`);

    if (!response.ok) {
      throw new Error(`Failed to fetch guardian: ${response.statusText}`);
    }

    // Unwrap the Next.js route wrapper: { success, message, data }
    const wrapper = (await response.json()) as { data: unknown };
    return mapGuardianResponse(wrapper.data as never);
  },

  async createGuardian(data: Omit<Guardian, "id" | "createdAt" | "updatedAt">): Promise<Guardian> {
    const requestData = mapGuardianRequest(data);

    const response = await fetch("/api/guardians", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestData),
    });

    if (!response.ok) {
      throw new Error(`Failed to create guardian: ${response.statusText}`);
    }

    // Unwrap the Next.js route wrapper: { success, message, data }
    const wrapper = (await response.json()) as { data: unknown };
    return mapGuardianResponse(wrapper.data as never);
  },

  async updateGuardian(id: string, data: Partial<Guardian>): Promise<Guardian> {
    const requestData = mapGuardianRequest(data);

    const response = await fetch(`/api/guardians/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestData),
    });

    if (!response.ok) {
      throw new Error(`Failed to update guardian: ${response.statusText}`);
    }

    // Unwrap the Next.js route wrapper: { success, message, data }
    const wrapper = (await response.json()) as { data: unknown };
    return mapGuardianResponse(wrapper.data as never);
  },

  async deleteGuardian(id: string): Promise<void> {
    const response = await fetch(`/api/guardians/${id}`, {
      method: "DELETE",
    });

    if (!response.ok) {
      throw new Error(`Failed to delete guardian: ${response.statusText}`);
    }
  },

  async searchGuardians(query: string, limit = 20): Promise<Guardian[]> {
    const params = new URLSearchParams({ q: query, limit: String(limit) });
    const response = await fetch(`/api/guardians/search?${params.toString()}`);

    if (!response.ok) {
      throw new Error(`Failed to search guardians: ${response.statusText}`);
    }

    // Unwrap the Next.js route wrapper: { success, message, data }
    const wrapper = (await response.json()) as { data: unknown[] };
    return wrapper.data.map((item) => mapGuardianResponse(item as never));
  },

  // Student-Guardian relationships
  async getStudentGuardians(studentId: string): Promise<StudentGuardian[]> {
    const response = await fetch(`/api/students/${studentId}/guardians`);

    if (!response.ok) {
      throw new Error(`Failed to fetch student guardians: ${response.statusText}`);
    }

    // Unwrap the Next.js route wrapper: { success, message, data }
    const wrapper = (await response.json()) as { data: unknown[] };
    return wrapper.data.map((item) => mapStudentGuardianResponse(item as never));
  },

  async addStudentGuardian(
    studentId: string,
    data: {
      guardianId: string;
      relationshipType: string;
      isPrimary?: boolean;
      isEmergencyContact?: boolean;
      canPickup?: boolean;
    }
  ): Promise<StudentGuardian> {
    const requestData = {
      guardian_id: parseInt(data.guardianId),
      relationship_type: data.relationshipType,
      is_primary: data.isPrimary ?? false,
      is_emergency_contact: data.isEmergencyContact ?? false,
      can_pickup: data.canPickup ?? true,
    };

    const response = await fetch(`/api/students/${studentId}/guardians`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestData),
    });

    if (!response.ok) {
      throw new Error(`Failed to add guardian to student: ${response.statusText}`);
    }

    // Unwrap the Next.js route wrapper: { success, message, data }
    const wrapper = (await response.json()) as { data: unknown };
    return mapStudentGuardianResponse(wrapper.data as never);
  },

  async removeStudentGuardian(studentId: string, guardianId: string): Promise<void> {
    const response = await fetch(`/api/students/${studentId}/guardians/${guardianId}`, {
      method: "DELETE",
    });

    if (!response.ok) {
      throw new Error(`Failed to remove guardian from student: ${response.statusText}`);
    }
  },

  async updateStudentGuardian(
    studentId: string,
    guardianId: string,
    data: Partial<StudentGuardian>
  ): Promise<StudentGuardian> {
    const requestData = mapStudentGuardianRequest(data);

    const response = await fetch(`/api/students/${studentId}/guardians/${guardianId}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(requestData),
    });

    if (!response.ok) {
      throw new Error(`Failed to update student guardian relationship: ${response.statusText}`);
    }

    // Unwrap the Next.js route wrapper: { success, message, data }
    const wrapper = (await response.json()) as { data: unknown };
    return mapStudentGuardianResponse(wrapper.data as never);
  },

  async getGuardianStudents(guardianId: string): Promise<unknown[]> {
    const response = await fetch(`/api/guardians/${guardianId}/students`);

    if (!response.ok) {
      throw new Error(`Failed to fetch guardian students: ${response.statusText}`);
    }

    // Unwrap the Next.js route wrapper: { success, message, data }
    const wrapper = (await response.json()) as { data: unknown[] };
    return wrapper.data;
  },
};
