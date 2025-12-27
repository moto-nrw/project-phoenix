// This file contains the Teacher API service and related types

import { getSession } from "next-auth/react";
import type { Activity } from "./activity-helpers";

/**
 * Builds fetch headers with optional auth token
 */
function buildHeaders(token: string | undefined): HeadersInit {
  return token
    ? { Authorization: `Bearer ${token}`, "Content-Type": "application/json" }
    : { "Content-Type": "application/json" };
}

/**
 * Extracts error message from API error response
 */
function extractErrorMessage(
  errorData: { error?: string; message?: string },
  fallback: string,
): string {
  return errorData.error ?? errorData.message ?? fallback;
}

/**
 * Extracts ID from potentially wrapped API response
 */
function extractIdFromResponse(data: {
  id?: string | number;
  data?: { id?: string | number };
}): string | number | undefined {
  return data.id ?? data.data?.id;
}

/**
 * Extracts person ID from double-wrapped person response
 */
function extractPersonId(responseData: unknown): number | undefined {
  if (!responseData || typeof responseData !== "object") {
    return undefined;
  }

  const response = responseData as {
    data?: { data?: { id?: number }; id?: number };
    id?: number;
  };

  // Try route wrapper → backend wrapper → id
  if ("data" in response && response.data) {
    const backendResponse = response.data;
    if ("data" in backendResponse && backendResponse.data) {
      return backendResponse.data.id;
    }
    if ("id" in backendResponse) {
      return backendResponse.id;
    }
  }

  // Direct PersonResponse format
  if ("id" in response) {
    return response.id;
  }

  return undefined;
}

/**
 * Extracts Teacher data from potentially double-wrapped response
 */
function extractTeacherData(responseData: unknown): Teacher {
  if (!responseData || typeof responseData !== "object") {
    return responseData as Teacher;
  }

  const response = responseData as {
    data?: { data?: Teacher } | Teacher;
  };

  if ("data" in response && response.data) {
    const backendResponse = response.data;
    if (
      backendResponse &&
      typeof backendResponse === "object" &&
      "data" in backendResponse
    ) {
      return (backendResponse as { data: Teacher }).data;
    }
    return backendResponse as Teacher;
  }

  return responseData as Teacher;
}

// Define Teacher interface aligned with staff API response structure
export interface Teacher {
  id: string;
  name: string;
  first_name: string;
  last_name: string;
  email?: string; // Email address for authentication
  specialization?: string | null;
  role?: string | null;
  qualifications?: string | null;
  tag_id?: string | null;
  staff_notes?: string | null;
  created_at?: string;
  updated_at?: string;
  activities?: Activity[];
  // Optional fields from staff API for consistency
  person_id?: number;
  account_id?: number;
  is_teacher?: boolean;
  person?: unknown; // For nested person object
  // ID fields for proper mapping
  staff_id?: string;
  teacher_id?: string;
}

export interface TeacherWithCredentials extends Teacher {
  temporaryCredentials?: {
    email: string;
    password: string;
  };
}

// Teacher service with API methods
class TeacherService {
  // Get all teachers with optional filters
  async getTeachers(filters?: { search?: string }): Promise<Teacher[]> {
    try {
      let url = "/api/staff?teachers_only=true";

      // Add query parameters if filters are provided
      if (filters) {
        const params = new URLSearchParams();
        if (filters.search) {
          params.append("search", filters.search);
        }

        if (params.toString()) {
          url += `&${params.toString()}`;
        }
      }

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
        throw new Error(`Failed to fetch teachers: ${response.statusText}`);
      }

      const data = (await response.json()) as Teacher[] | { data: Teacher[] };

      // Handle different response formats
      if (Array.isArray(data)) {
        return data;
      } else if (
        data &&
        typeof data === "object" &&
        "data" in data &&
        Array.isArray(data.data)
      ) {
        return data.data;
      } else {
        console.error("Unexpected response format:", data);
        return [];
      }
    } catch (error) {
      console.error("Error fetching teachers:", error);
      throw error;
    }
  }

  // Get a single teacher by ID
  async getTeacher(id: string): Promise<Teacher> {
    try {
      const session = await getSession();
      const response = await fetch(`/api/staff/${id}`, {
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : undefined,
      });
      if (!response.ok) {
        throw new Error(`Failed to fetch teacher: ${response.statusText}`);
      }

      const data = (await response.json()) as Teacher | { data: Teacher };
      // Processing teacher API response

      // Handle wrapped response from route handler
      if (data && typeof data === "object" && "data" in data) {
        // Response is wrapped (from route handler)
        return data.data;
      }

      // Direct teacher object
      return data;
    } catch (error) {
      console.error(`Error fetching teacher with ID ${id}:`, error);
      throw error;
    }
  }

  // Create a new teacher
  async createTeacher(
    teacherData: Omit<Teacher, "id" | "name" | "created_at" | "updated_at"> & {
      password?: string;
      role_id?: number;
    },
  ): Promise<TeacherWithCredentials> {
    const password = teacherData.password;
    if (!password) {
      throw new Error("Password is required for creating a teacher");
    }

    const roleId = teacherData.role_id;
    if (!roleId) {
      throw new Error("Role ID is required for creating a teacher");
    }

    const email =
      teacherData.email ??
      `${teacherData.first_name.toLowerCase()}.${teacherData.last_name.toLowerCase()}@school.local`;
    const username = `${teacherData.first_name.toLowerCase()}_${teacherData.last_name.toLowerCase()}`;
    const fullName = `${teacherData.first_name} ${teacherData.last_name}`;

    const session = await getSession();
    const token = session?.user?.token;
    const headers = buildHeaders(token);

    // Step 1: Create account
    const accountId = await this.createAccount(
      headers,
      email,
      username,
      fullName,
      password,
      roleId,
    );

    // Step 2: Create person linked to account
    const personId = await this.createPerson(headers, teacherData, accountId);

    // Step 3: Create staff with is_teacher flag
    const staffData = await this.createStaff(headers, teacherData, personId);

    // Return with credentials and name data
    return {
      ...staffData,
      first_name: teacherData.first_name,
      last_name: teacherData.last_name,
      name: fullName,
      email: email,
      temporaryCredentials: { email, password },
    } as TeacherWithCredentials;
  }

  /** Creates account for teacher registration */
  private async createAccount(
    headers: HeadersInit,
    email: string,
    username: string,
    name: string,
    password: string,
    roleId: number,
  ): Promise<string | number> {
    const response = await fetch("/api/auth/register", {
      method: "POST",
      credentials: "include",
      headers,
      body: JSON.stringify({
        email,
        username,
        name,
        password,
        confirm_password: password,
        role_id: roleId,
      }),
    });

    if (!response.ok) {
      const errorData = (await response.json()) as {
        error?: string;
        message?: string;
      };
      throw new Error(
        `Failed to create account: ${extractErrorMessage(errorData, response.statusText)}`,
      );
    }

    const data = (await response.json()) as {
      id?: string | number;
      data?: { id?: string | number };
    };
    const accountId = extractIdFromResponse(data);

    if (!accountId) {
      console.error("Failed to get account ID from response:", data);
      throw new Error("Failed to get account ID from response");
    }

    return accountId;
  }

  /** Creates person linked to account */
  private async createPerson(
    headers: HeadersInit,
    teacherData: {
      first_name: string;
      last_name: string;
      tag_id?: string | null;
    },
    accountId: string | number,
  ): Promise<number> {
    const response = await fetch("/api/users", {
      method: "POST",
      credentials: "include",
      headers,
      body: JSON.stringify({
        first_name: teacherData.first_name,
        last_name: teacherData.last_name,
        tag_id: teacherData.tag_id ?? null,
        account_id: accountId,
      }),
    });

    if (!response.ok) {
      const errorData = (await response.json()) as {
        error?: string;
        message?: string;
      };
      throw new Error(
        `Failed to create person: ${extractErrorMessage(errorData, response.statusText)}`,
      );
    }

    const data: unknown = await response.json();
    const personId = extractPersonId(data);

    if (!personId) {
      console.error("Unexpected person response format:", data);
      throw new Error("Failed to get person ID from response");
    }

    return personId;
  }

  /** Creates staff record with is_teacher flag */
  private async createStaff(
    headers: HeadersInit,
    teacherData: {
      staff_notes?: string | null;
      specialization?: string | null;
      role?: string | null;
      qualifications?: string | null;
    },
    personId: number,
  ): Promise<Teacher> {
    const response = await fetch("/api/staff", {
      method: "POST",
      credentials: "include",
      headers,
      body: JSON.stringify({
        person_id: personId,
        is_teacher: true,
        staff_notes: teacherData.staff_notes?.trim() ?? "",
        specialization: teacherData.specialization?.trim() ?? "",
        role: teacherData.role?.trim() ?? "",
        qualifications: teacherData.qualifications?.trim() ?? "",
      }),
    });

    if (!response.ok) {
      const errorText = await response.text().catch(() => "");
      const suffix = errorText ? ` - ${errorText}` : "";
      throw new Error(
        `Failed to create teacher: ${response.statusText}${suffix}`,
      );
    }

    const data: unknown = await response.json();
    return extractTeacherData(data);
  }

  // Update an existing teacher
  async updateTeacher(
    id: string,
    teacherData: Partial<Teacher>,
  ): Promise<Teacher> {
    try {
      const session = await getSession();

      // First, get the current teacher data to get person_id
      const currentTeacher = await this.getTeacher(id);
      console.log("Current teacher data:", currentTeacher);
      console.log("Update data:", teacherData);

      // If name fields are included, update the person record first
      if (
        teacherData.first_name ||
        teacherData.last_name ||
        teacherData.tag_id !== undefined
      ) {
        if (!currentTeacher.person_id) {
          throw new Error("Cannot update person fields - person_id not found");
        }

        // First, we need to get the person data to find the account_id
        const personResponse = await fetch(
          `/api/users/${currentTeacher.person_id}`,
          {
            method: "GET",
            credentials: "include",
            headers: session?.user?.token
              ? {
                  Authorization: `Bearer ${session.user.token}`,
                  "Content-Type": "application/json",
                }
              : undefined,
          },
        );

        if (!personResponse.ok) {
          throw new Error("Failed to fetch person data");
        }

        const personInfo = (await personResponse.json()) as {
          data?: { account_id?: number };
          account_id?: number;
        };

        const personData: {
          first_name?: string;
          last_name?: string;
          tag_id?: string | null;
          account_id?: number;
        } = {};
        if (teacherData.first_name !== undefined)
          personData.first_name = teacherData.first_name;
        if (teacherData.last_name !== undefined)
          personData.last_name = teacherData.last_name;
        if (teacherData.tag_id !== undefined) {
          // Convert empty string to null for backend
          personData.tag_id = teacherData.tag_id ?? null;
        }
        // Always include account_id from the fetched person data
        const accountId = personInfo.data?.account_id ?? personInfo.account_id;
        if (accountId) {
          personData.account_id = accountId;
        }

        console.log("Person update data:", personData);
        console.log("Person ID:", currentTeacher.person_id);
        console.log("Person info fetched:", personInfo);

        const personUpdateResponse = await fetch(
          `/api/users/${currentTeacher.person_id}`,
          {
            method: "PUT",
            credentials: "include",
            headers: session?.user?.token
              ? {
                  Authorization: `Bearer ${session.user.token}`,
                  "Content-Type": "application/json",
                }
              : {
                  "Content-Type": "application/json",
                },
            body: JSON.stringify(personData),
          },
        );

        if (!personUpdateResponse.ok) {
          const errorText = await personUpdateResponse.text();
          throw new Error(`Failed to update person: ${errorText}`);
        }
      }

      // Then update the staff record with staff-specific fields
      // Note: Backend does FULL model updates, so we send all fields.
      // Merge partial update data with current teacher data, then send everything.
      // Empty strings are converted to NULL via BUN's nullzero tag.
      const staffData = {
        person_id: currentTeacher.person_id, // Required by backend
        is_teacher: true,
        staff_notes:
          (teacherData.staff_notes !== undefined
            ? teacherData.staff_notes?.trim()
            : currentTeacher.staff_notes?.trim()) ?? "",
        specialization:
          (teacherData.specialization !== undefined
            ? teacherData.specialization?.trim()
            : currentTeacher.specialization?.trim()) ?? "",
        role:
          (teacherData.role !== undefined
            ? teacherData.role?.trim()
            : currentTeacher.role?.trim()) ?? "",
        qualifications:
          (teacherData.qualifications !== undefined
            ? teacherData.qualifications?.trim()
            : currentTeacher.qualifications?.trim()) ?? "",
      };

      const response = await fetch(`/api/staff/${id}`, {
        method: "PUT",
        credentials: "include",
        headers: session?.user?.token
          ? {
              Authorization: `Bearer ${session.user.token}`,
              "Content-Type": "application/json",
            }
          : {
              "Content-Type": "application/json",
            },
        body: JSON.stringify(staffData),
      });

      if (!response.ok) {
        const errorText = await response.text().catch(() => "");
        throw new Error(
          `Failed to update teacher: ${response.statusText}${errorText ? ` - ${errorText}` : ""}`,
        );
      }

      const data = (await response.json()) as Teacher;
      return data;
    } catch (error) {
      console.error(`Error updating teacher with ID ${id}:`, error);
      throw error;
    }
  }

  // Delete a teacher
  async deleteTeacher(id: string): Promise<void> {
    try {
      const session = await getSession();
      const response = await fetch(`/api/staff/${id}`, {
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
        throw new Error(`Failed to delete teacher: ${response.statusText}`);
      }
    } catch (error) {
      console.error(`Error deleting teacher with ID ${id}:`, error);
      throw error;
    }
  }

  // Get activities for a teacher
  async getTeacherActivities(id: string): Promise<Activity[]> {
    try {
      // For now, activities endpoint is not implemented for staff
      // Return empty array until implemented on the backend
      console.warn(`Activities endpoint not implemented for staff/teachers`);
      return [];
    } catch (error) {
      console.error(
        `Error fetching activities for teacher with ID ${id}:`,
        error,
      );
      throw error;
    }
  }
}

export const teacherService = new TeacherService();
