// lib/student-api.ts
import { getCachedSession } from "./session-cache";
import { env } from "~/env";
import api from "./api";
import {
  handleDomainApiError,
  isBrowserContext,
  authFetch,
} from "./api-helpers";
import {
  mapStudentResponse,
  mapStudentsResponse,
  mapStudentDetailResponse,
  mapPrivacyConsentResponse,
  type Student,
  type BackendStudent,
  type BackendStudentDetail,
  type BackendPrivacyConsent,
  type BackendUpdateRequest,
  type PrivacyConsent,
} from "./student-helpers";
import {
  mapGroupResponse,
  type Group,
  type BackendGroup,
} from "./group-helpers";

// Filter interface for searching students
export interface StudentFilters {
  search?: string;
  school_class?: string;
  group_id?: string;
  location?: string;
  guardian_name?: string;
  first_name?: string;
  last_name?: string;
  page?: number;
  page_size?: number;
}

// Generic API response interface
interface ApiResponse<T> {
  data: T;
  message?: string;
  status?: string;
}

// Paginated response interface
interface PaginatedResponse<T> {
  status: string;
  data: T[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
  message?: string;
}

// Error handler using shared utility
function handleStudentApiError(error: unknown, context: string): never {
  handleDomainApiError(error, context, "STUDENT");
}

// Helper: Build URL with optional query params
// Note: Browser uses proxy path (/api/students), server uses direct backend path (/students)
function buildStudentUrl(
  proxyPath: string,
  backendPath: string,
  filters?: StudentFilters,
): { url: string; useProxy: boolean } {
  const useProxy = isBrowserContext();
  const baseUrl = useProxy
    ? proxyPath
    : `${env.NEXT_PUBLIC_API_URL}${backendPath}`;

  if (!filters) return { url: baseUrl, useProxy };

  const params = new URLSearchParams();
  if (filters.search) params.append("search", filters.search);
  if (filters.school_class) params.append("school_class", filters.school_class);
  if (filters.group_id) params.append("group_id", filters.group_id);
  if (filters.location) params.append("location", filters.location);
  if (filters.guardian_name)
    params.append("guardian_name", filters.guardian_name);
  if (filters.first_name) params.append("first_name", filters.first_name);
  if (filters.last_name) params.append("last_name", filters.last_name);
  if (filters.page) params.append("page", filters.page.toString());
  if (filters.page_size)
    params.append("page_size", filters.page_size.toString());

  const queryString = params.toString();
  return { url: queryString ? `${baseUrl}?${queryString}` : baseUrl, useProxy };
}

// Fetch students with filters and pagination
export async function fetchStudents(filters?: StudentFilters): Promise<{
  students: Student[];
  pagination?: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
}> {
  const { url, useProxy } = buildStudentUrl(
    "/api/students",
    "/students",
    filters,
  );

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<
        Student[] | PaginatedResponse<Student>
      >(url, { token: session?.user?.token });

      // Check if it's a paginated response
      if (
        responseData &&
        typeof responseData === "object" &&
        "data" in responseData &&
        "pagination" in responseData
      ) {
        return {
          students: responseData.data,
          pagination: responseData.pagination,
        };
      }

      // Fallback for non-paginated response
      return { students: Array.isArray(responseData) ? responseData : [] };
    }

    // Server-side: use axios with the API URL directly
    const response = await api.get<PaginatedResponse<BackendStudent>>(url);
    if (response.data?.data) {
      return {
        students: mapStudentsResponse(response.data.data),
        pagination: response.data.pagination,
      };
    }
    return { students: [] };
  } catch (error) {
    handleStudentApiError(error, "fetch students");
  }
}

// Type guard for wrapped API response
function isWrappedResponse<T>(
  responseData: unknown,
): responseData is ApiResponse<T> {
  return (
    responseData !== null &&
    typeof responseData === "object" &&
    "data" in responseData
  );
}

// Helper: Extract data from wrapped API response
function extractApiData<T>(responseData: ApiResponse<T> | T): T {
  if (isWrappedResponse<T>(responseData)) {
    return responseData.data;
  }
  return responseData;
}

// Fetch a single student by ID
export async function fetchStudent(id: string): Promise<Student> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${id}`
    : `${env.NEXT_PUBLIC_API_URL}/students/${id}`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<ApiResponse<Student> | Student>(
        url,
        {
          token: session?.user?.token,
        },
      );
      return extractApiData(responseData);
    }

    const response = await api.get<ApiResponse<BackendStudentDetail>>(url);
    return mapStudentDetailResponse(response.data.data);
  } catch (error) {
    handleStudentApiError(error, "fetch student");
  }
}

// Create a new student
export async function createStudent(studentData: {
  first_name: string;
  last_name: string;
  school_class: string;
  guardian_name: string;
  guardian_contact: string;
  group_id?: number;
  tag_id?: string;
  guardian_email?: string;
  guardian_phone?: string;
}): Promise<Student> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? "/api/students"
    : `${env.NEXT_PUBLIC_API_URL}/students`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<
        ApiResponse<BackendStudent> | BackendStudent
      >(url, {
        method: "POST",
        body: studentData,
        token: session?.user?.token,
      });
      const data = extractApiData<BackendStudent>(responseData);
      return mapStudentResponse(data);
    }

    const response = await api.post<ApiResponse<BackendStudent>>(
      url,
      studentData,
    );
    return mapStudentResponse(response.data.data);
  } catch (error) {
    handleStudentApiError(error, "create student");
  }
}

// Update a student
export async function updateStudent(
  id: string,
  studentData: BackendUpdateRequest,
): Promise<Student> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${id}`
    : `${env.NEXT_PUBLIC_API_URL}/students/${id}`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<
        ApiResponse<BackendStudent> | BackendStudent
      >(url, { method: "PUT", body: studentData, token: session?.user?.token });
      const data = extractApiData<BackendStudent>(responseData);
      return mapStudentResponse(data);
    }

    const response = await api.put<ApiResponse<BackendStudent>>(
      url,
      studentData,
    );
    return mapStudentResponse(response.data.data);
  } catch (error) {
    handleStudentApiError(error, "update student");
  }
}

// Delete a student
export async function deleteStudent(id: string): Promise<void> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${id}`
    : `${env.NEXT_PUBLIC_API_URL}/students/${id}`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      await authFetch<void>(url, {
        method: "DELETE",
        token: session?.user?.token,
      });
    } else {
      await api.delete(url);
    }
  } catch (error) {
    handleStudentApiError(error, "delete student");
  }
}

// Fetch all groups (for filter dropdown)
export async function fetchGroups(): Promise<Group[]> {
  const useProxy = isBrowserContext();
  const url = useProxy ? "/api/groups" : `${env.NEXT_PUBLIC_API_URL}/groups`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<BackendGroup[]>(url, {
        token: session?.user?.token,
      });
      const groups = Array.isArray(responseData) ? responseData : [];
      return groups.map(mapGroupResponse);
    }

    const response = await api.get<{ data: BackendGroup[] }>(url);
    const groups = response.data.data || [];
    return groups.map(mapGroupResponse);
  } catch (error) {
    console.error("Error fetching groups:", error);
    return [];
  }
}

// Fetch student's privacy consent
export async function fetchStudentPrivacyConsent(
  studentId: string,
): Promise<PrivacyConsent | null> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${studentId}/privacy-consent`
    : `${env.NEXT_PUBLIC_API_URL}/students/${studentId}/privacy-consent`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      // Cannot use authFetch here due to special 404 handling
      const response = await fetch(url, {
        method: "GET",
        credentials: "include",
        headers: session?.user?.token
          ? { Authorization: `Bearer ${session.user.token}` }
          : undefined,
      });

      if (!response.ok) {
        if (response.status === 404) return null;
        throw new Error(
          `API error (${response.status}): ${response.statusText}`,
        );
      }

      const responseData =
        (await response.json()) as ApiResponse<BackendPrivacyConsent>;
      return mapPrivacyConsentResponse(responseData.data);
    }

    const response = await api.get<ApiResponse<BackendPrivacyConsent>>(url);
    return mapPrivacyConsentResponse(response.data.data);
  } catch (error) {
    console.error("Error fetching privacy consent:", error);
    return null;
  }
}

// Update or create student's privacy consent
export async function updateStudentPrivacyConsent(
  studentId: string,
  consentData: {
    policy_version: string;
    accepted: boolean;
    duration_days?: number;
    data_retention_days: number;
    details?: Record<string, unknown>;
  },
): Promise<PrivacyConsent> {
  const useProxy = isBrowserContext();
  const url = useProxy
    ? `/api/students/${studentId}/privacy-consent`
    : `${env.NEXT_PUBLIC_API_URL}/students/${studentId}/privacy-consent`;

  try {
    if (useProxy) {
      const session = await getCachedSession();
      const responseData = await authFetch<ApiResponse<BackendPrivacyConsent>>(
        url,
        { method: "PUT", body: consentData, token: session?.user?.token },
      );
      return mapPrivacyConsentResponse(responseData.data);
    }

    const session = await getCachedSession();
    const response = await api.put<ApiResponse<BackendPrivacyConsent>>(
      url,
      consentData,
      { headers: { Authorization: `Bearer ${session?.user?.token}` } },
    );
    return mapPrivacyConsentResponse(response.data.data);
  } catch (error) {
    handleStudentApiError(error, "update privacy consent");
  }
}
