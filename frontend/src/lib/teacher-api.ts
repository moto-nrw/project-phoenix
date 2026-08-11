// This file contains the Teacher API service and related types

import { sessionFetch } from "./session-cache";
import { createLogger } from "~/lib/logger";
import type { Activity } from "./activity-helpers";

const logger = createLogger({ component: "TeacherAPI" });

/** Result of a createTeacher call — either success or a prompt to link an existing account. */
type CreateTeacherResult =
  | { status: "created"; data: TeacherWithCredentials }
  | { status: "account_exists"; email: string };

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

/** The person fields the backend needs to provision a staff identity. */
interface PersonIdentityFields {
  first_name: string;
  last_name: string;
  tag_id?: string | null;
}

/** What the backend provisioned alongside the account (#2222). */
interface SchoolIdentity {
  person_id: number;
  staff_id: number;
  teacher_id?: number;
}

interface AccountWithIdentityResponse {
  id?: string | number;
  school_identity?: SchoolIdentity | null;
  data?: { id?: string | number; school_identity?: SchoolIdentity | null };
}

/**
 * Extracts the provisioned person/staff ids from an account response.
 *
 * Present whenever the request carried first_name and last_name: since #2222
 * the backend creates the person and staff record in the same transaction as
 * the account, so the client never has to stitch that chain together across
 * requests — a failure between them used to leave an account that held a role
 * and was not staff.
 */
function extractSchoolIdentity(
  data: AccountWithIdentityResponse,
): SchoolIdentity | undefined {
  return data.school_identity ?? data.data?.school_identity ?? undefined;
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
  avatar?: string | null; // Avatar path from account
  specialization?: string | null;
  role?: string | null;
  account_role?: string | null;
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

      const response = await sessionFetch(url, {
        credentials: "include",
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
        logger.error("unexpected response format for teachers");
        return [];
      }
    } catch (error) {
      logger.error("error fetching teachers", { error: String(error) });
      throw error;
    }
  }

  // Get a single teacher by ID
  async getTeacher(id: string): Promise<Teacher> {
    try {
      const response = await sessionFetch(`/api/staff/${id}`, {
        credentials: "include",
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
      logger.error("error fetching teacher", {
        teacher_id: id,
        error: String(error),
      });
      throw error;
    }
  }

  // Create a new teacher
  async createTeacher(
    teacherData: Omit<Teacher, "id" | "name" | "created_at" | "updated_at"> & {
      password?: string;
      role_id?: number;
      linkExisting?: boolean;
    },
  ): Promise<CreateTeacherResult> {
    const roleId = teacherData.role_id;
    if (!roleId) {
      throw new Error("Role ID is required for creating a teacher");
    }

    const email =
      teacherData.email ??
      `${teacherData.first_name.toLowerCase()}.${teacherData.last_name.toLowerCase()}@school.local`;
    const suffix = Math.random().toString(36).slice(2, 8);
    const username = `${teacherData.first_name.toLowerCase()}_${teacherData.last_name.toLowerCase()}_${suffix}`;
    const fullName = `${teacherData.first_name} ${teacherData.last_name}`;

    // Step 1: Create account or link existing. The identity fields go with it,
    // so the backend creates the person and staff record in the same
    // transaction (#2222) instead of us stitching the chain together across
    // three requests that can fail in the middle.
    let personId: number;
    if (teacherData.linkExisting) {
      // Link existing account — password is NOT changed
      personId = await this.linkAccountToTenant(email, roleId, teacherData);
    } else {
      const password = teacherData.password;
      if (!password) {
        throw new Error("Password is required for creating a teacher");
      }
      // Try to create — if email exists, return account_exists result (not an error)
      const createResult = await this.tryCreateAccount(
        email,
        username,
        fullName,
        password,
        roleId,
        teacherData,
      );
      if (createResult.status === "account_exists") {
        return { status: "account_exists", email };
      }
      personId = createResult.personId;
    }

    // Step 2: apply the staff details to the record that already exists. The
    // backend adopts it instead of creating a second one.
    const staffData = await this.createStaff(teacherData, personId);

    // Return with credentials and name data
    return {
      status: "created",
      data: {
        ...staffData,
        first_name: teacherData.first_name,
        last_name: teacherData.last_name,
        name: fullName,
        email: email,
        temporaryCredentials: teacherData.linkExisting
          ? undefined
          : { email, password: teacherData.password ?? "" },
      } as TeacherWithCredentials,
    };
  }

  /** Tries to create an account — returns account_exists status instead of throwing. */
  private async tryCreateAccount(
    email: string,
    username: string,
    name: string,
    password: string,
    roleId: number,
    identity: PersonIdentityFields,
  ): Promise<
    { status: "created"; personId: number } | { status: "account_exists" }
  > {
    const response = await sessionFetch("/api/auth/register", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        email,
        username,
        name,
        password,
        confirm_password: password,
        role_id: roleId,
        first_name: identity.first_name,
        last_name: identity.last_name,
        tag_id: identity.tag_id ?? null,
      }),
    });

    if (!response.ok) {
      const errorData = (await response.json()) as {
        error?: string;
        message?: string;
      };
      const msg = extractErrorMessage(errorData, response.statusText);
      if (msg.includes("bereits registriert")) {
        return { status: "account_exists" as const };
      }
      if (msg.includes("Benutzername ist bereits vergeben")) {
        throw new Error(
          "Ein Konto mit diesem Benutzernamen existiert bereits.",
        );
      }
      throw new Error(`Konto konnte nicht erstellt werden: ${msg}`);
    }

    const data = (await response.json()) as AccountWithIdentityResponse;
    if (!extractIdFromResponse(data)) {
      logger.error("failed to get account ID from response");
      throw new Error("Failed to get account ID from response");
    }

    const identityIds = extractSchoolIdentity(data);
    if (!identityIds) {
      logger.error("account created without school identity");
      throw new Error(
        "Das Konto wurde angelegt, aber kein Mitarbeiter-Datensatz. Bitte erneut versuchen.",
      );
    }

    return { status: "created" as const, personId: identityIds.person_id };
  }

  /** Links an existing account to the current tenant. */
  private async linkAccountToTenant(
    email: string,
    roleId: number,
    identity: PersonIdentityFields,
  ): Promise<number> {
    const response = await sessionFetch("/api/auth/link-to-tenant", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        email,
        role_id: roleId,
        first_name: identity.first_name,
        last_name: identity.last_name,
        tag_id: identity.tag_id ?? null,
      }),
    });

    if (!response.ok) {
      const errorData = (await response.json()) as {
        error?: string;
        message?: string;
      };
      throw new Error(
        `Konto konnte nicht verknüpft werden: ${extractErrorMessage(errorData, response.statusText)}`,
      );
    }

    const data = (await response.json()) as AccountWithIdentityResponse;
    const identityIds = extractSchoolIdentity(data);

    if (!identityIds) {
      throw new Error(
        "Der Mitarbeiter-Datensatz konnte nicht aus der Verknüpfungs-Antwort gelesen werden.",
      );
    }

    return identityIds.person_id;
  }

  /** Creates staff record with is_teacher flag */
  private async createStaff(
    teacherData: {
      staff_notes?: string | null;
      specialization?: string | null;
      role?: string | null;
      qualifications?: string | null;
      is_teacher?: boolean;
    },
    personId: number,
  ): Promise<Teacher> {
    const response = await sessionFetch("/api/staff", {
      method: "POST",
      credentials: "include",
      body: JSON.stringify({
        person_id: personId,
        // Lehrkraft (#1772) bekommt kein Betreuungsprofil (users.teachers):
        // das Formular setzt is_teacher=false, alle anderen Rollen behalten
        // den bisherigen Default.
        is_teacher: teacherData.is_teacher ?? true,
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
    const currentTeacher = await this.getTeacher(id);

    // Update person record if name/tag fields changed
    const needsPersonUpdate =
      teacherData.first_name !== undefined ||
      teacherData.last_name !== undefined ||
      teacherData.tag_id !== undefined;

    if (needsPersonUpdate) {
      await this.updatePersonRecord(currentTeacher, teacherData);
    }

    // Update staff record
    const staffData = this.buildStaffUpdateData(currentTeacher, teacherData);
    return this.updateStaffRecord(id, staffData);
  }

  /** Updates person record for teacher */
  private async updatePersonRecord(
    currentTeacher: Teacher,
    teacherData: Partial<Teacher>,
  ): Promise<void> {
    if (!currentTeacher.person_id) {
      throw new Error("Cannot update person fields - person_id not found");
    }

    const personId = currentTeacher.person_id;

    // Fetch current person to get account_id
    const personResponse = await sessionFetch(`/api/users/${personId}`, {
      method: "GET",
      credentials: "include",
    });

    if (!personResponse.ok) {
      throw new Error("Failed to fetch person data");
    }

    const personInfo = (await personResponse.json()) as {
      data?: { account_id?: number };
      account_id?: number;
    };

    // Build person update payload
    const personData = this.buildPersonUpdateData(teacherData, personInfo);

    const updateResponse = await sessionFetch(`/api/users/${personId}`, {
      method: "PUT",
      credentials: "include",
      body: JSON.stringify(personData),
    });

    if (!updateResponse.ok) {
      const errorText = await updateResponse.text();
      throw new Error(`Failed to update person: ${errorText}`);
    }
  }

  /** Builds person update payload from teacher data */
  private buildPersonUpdateData(
    teacherData: Partial<Teacher>,
    personInfo: { data?: { account_id?: number }; account_id?: number },
  ): {
    first_name?: string;
    last_name?: string;
    tag_id?: string | null;
    account_id?: number;
  } {
    const personData: {
      first_name?: string;
      last_name?: string;
      tag_id?: string | null;
      account_id?: number;
    } = {};

    if (teacherData.first_name !== undefined) {
      personData.first_name = teacherData.first_name;
    }
    if (teacherData.last_name !== undefined) {
      personData.last_name = teacherData.last_name;
    }
    if (teacherData.tag_id !== undefined) {
      personData.tag_id = teacherData.tag_id ?? null;
    }

    const accountId = personInfo.data?.account_id ?? personInfo.account_id;
    if (accountId) {
      personData.account_id = accountId;
    }

    return personData;
  }

  /** Builds staff update data by merging partial update with current values */
  private buildStaffUpdateData(
    currentTeacher: Teacher,
    teacherData: Partial<Teacher>,
  ): {
    person_id: number | undefined;
    is_teacher: boolean;
    staff_notes: string;
    specialization: string;
    role: string;
    qualifications: string;
  } {
    return {
      person_id: currentTeacher.person_id,
      // Den bestehenden Profil-Zustand erhalten, NIE hart true senden:
      // PUT /api/staff/{id} legt bei is_teacher=true eine fehlende
      // users.teachers-Zeile an — eine Lehrkraft (#1772, ohne
      // Betreuungsprofil) bekäme durch eine bloße Namenskorrektur sonst
      // stillschweigend eines.
      is_teacher:
        currentTeacher.is_teacher ?? Boolean(currentTeacher.teacher_id),
      staff_notes: this.mergeField(
        teacherData.staff_notes,
        currentTeacher.staff_notes,
      ),
      specialization: this.mergeField(
        teacherData.specialization,
        currentTeacher.specialization,
      ),
      role: this.mergeField(teacherData.role, currentTeacher.role),
      qualifications: this.mergeField(
        teacherData.qualifications,
        currentTeacher.qualifications,
      ),
    };
  }

  /** Merges update value with current value, trimming and defaulting to empty string */
  private mergeField(
    updateValue: string | null | undefined,
    currentValue: string | null | undefined,
  ): string {
    if (updateValue !== undefined) {
      return updateValue?.trim() ?? "";
    }
    return currentValue?.trim() ?? "";
  }

  /** Updates staff record via API */
  private async updateStaffRecord(
    id: string,
    staffData: object,
  ): Promise<Teacher> {
    const response = await sessionFetch(`/api/staff/${id}`, {
      method: "PUT",
      credentials: "include",
      body: JSON.stringify(staffData),
    });

    if (!response.ok) {
      const errorText = await response.text().catch(() => "");
      const suffix = errorText ? ` - ${errorText}` : "";
      throw new Error(
        `Failed to update teacher: ${response.statusText}${suffix}`,
      );
    }

    return (await response.json()) as Teacher;
  }

  // Delete a teacher
  async deleteTeacher(id: string): Promise<void> {
    try {
      const response = await sessionFetch(`/api/staff/${id}`, {
        method: "DELETE",
        credentials: "include",
      });

      if (!response.ok) {
        throw new Error(`Failed to delete teacher: ${response.statusText}`);
      }
    } catch (error) {
      logger.error("error deleting teacher", {
        teacher_id: id,
        error: String(error),
      });
      throw error;
    }
  }

  // Get activities for a teacher
  async getTeacherActivities(id: string): Promise<Activity[]> {
    try {
      // For now, activities endpoint is not implemented for staff
      // Return empty array until implemented on the backend
      logger.warn("activities endpoint not implemented for staff/teachers");
      return [];
    } catch (error) {
      logger.error("error fetching activities for teacher", {
        teacher_id: id,
        error: String(error),
      });
      throw error;
    }
  }
}

export const teacherService = new TeacherService();
