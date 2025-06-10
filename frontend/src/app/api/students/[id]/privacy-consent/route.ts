import type { AxiosError } from "axios";
import { createGetHandler, createPutHandler } from "~/lib/route-wrapper";
import { apiGet, apiPut } from "~/lib/api-client";

interface PrivacyConsentBody {
  policy_version?: string;
  accepted?: boolean;
  duration_days?: number;
  data_retention_days?: number;
  details?: Record<string, unknown>;
}

interface PrivacyConsentResponse {
  data: {
    id: number;
    student_id: number;
    policy_version: string;
    accepted: boolean;
    accepted_at?: string;
    expires_at?: string;
    duration_days?: number;
    renewal_required: boolean;
    data_retention_days: number;
    details?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
  };
}

// GET handler - Fetch student's privacy consent
export const GET = createGetHandler(async (request, token, params) => {
  const id = params.id as string;
  
  if (!id) {
    throw new Error("Student ID is required");
  }

  try {
    const response = await apiGet<PrivacyConsentResponse>(`/api/students/${id}/privacy-consent`, token);
    return response.data.data;
  } catch (error) {
    const axiosError = error as AxiosError;
    if (axiosError.response?.status === 404) {
      // No consent found, return null or a default response
      return {
        student_id: parseInt(id, 10),
        policy_version: "",
        accepted: false,
        data_retention_days: 30,
        renewal_required: false,
      };
    }
    throw error;
  }
});

// PUT handler - Update or create privacy consent  
export const PUT = createPutHandler<unknown, PrivacyConsentBody>(
  async (request, body, token, params) => {
    const id = params.id as string;
    
    if (!id) {
      throw new Error("Student ID is required");
    }

    // Validate required fields
    if (!body.policy_version || body.data_retention_days === undefined) {
      throw new Error("policy_version and data_retention_days are required");
    }

    // Validate data_retention_days range
    if (body.data_retention_days < 1 || body.data_retention_days > 31) {
      throw new Error("data_retention_days must be between 1 and 31");
    }

    const response = await apiPut<PrivacyConsentResponse>(`/api/students/${id}/privacy-consent`, body, token);
    return response.data.data;
  }
);