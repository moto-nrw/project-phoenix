import { createGetHandler } from "@/lib/route-wrapper";
import { apiGet } from "@/lib/api-client";
import type { BackendPermission } from "@/lib/auth-helpers";

interface PermissionsResponse {
  status: string;
  data: BackendPermission[];
  message?: string;
}

export const GET = createGetHandler(async (request, token, params) => {
  // Extract accountId from params, ensuring it's defined
  if (!params.accountId) {
    throw new Error("Account ID is required");
  }

  const accountId = params.accountId as string;
  // Fetching direct permissions for account

  // Make the API call to get only direct permissions (not role-based)
  const response = await apiGet<PermissionsResponse>(
    `/auth/accounts/${accountId}/permissions/direct`,
    token,
  );
  return response.data;
});
