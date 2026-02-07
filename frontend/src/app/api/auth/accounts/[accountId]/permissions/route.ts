import { createGetHandler } from "@/lib/route-wrapper";
import { apiGet } from "@/lib/api-client";
import type { BackendPermission } from "@/lib/auth-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AccountPermissionsRoute" });

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
  logger.debug("fetching permissions for account", { account_id: accountId });

  // Make the API call with the validated account ID
  const response = await apiGet<PermissionsResponse>(
    `/auth/accounts/${accountId}/permissions`,
    token,
  );
  return response.data;
});
