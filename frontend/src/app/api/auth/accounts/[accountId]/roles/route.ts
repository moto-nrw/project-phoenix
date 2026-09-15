import { createGetHandler, createPutHandler } from "@/lib/route-wrapper.server";
import { apiGet, apiPut } from "~/lib/api-helpers.server";
import type { BackendRole } from "@/lib/auth-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AccountRolesRoute" });

interface RolesResponse {
  status: string;
  data: BackendRole[];
  message?: string;
}

export const GET = createGetHandler(async (request, token, params) => {
  // Extract accountId from params, ensuring it's defined
  if (!params.accountId) {
    throw new Error("Account ID is required");
  }

  const accountId = params.accountId as string;
  logger.debug("fetching roles for account", { account_id: accountId });

  // Make the API call with the validated account ID
  return await apiGet<RolesResponse>(
    `/auth/accounts/${accountId}/roles`,
    token,
  );
});

interface ReplaceAccountRoleBody {
  role_id: string;
}

export const PUT = createPutHandler<unknown, ReplaceAccountRoleBody>(
  async (_request, body, token, params) => {
    const accountId = params.accountId as string;
    if (!accountId) {
      throw new Error("Account ID is required");
    }
    return await apiPut<unknown>(
      `/auth/accounts/${accountId}/roles`,
      token,
      body,
    );
  },
);
