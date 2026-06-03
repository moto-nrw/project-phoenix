import { createPutHandler } from "@/lib/route-wrapper.server";
import { apiPut } from "~/lib/api-helpers.server";

export const PUT = createPutHandler(async (request, body, token, params) => {
  const accountId = params.accountId as string;
  return await apiPut<{ message: string }>(
    `/auth/accounts/${accountId}/deactivate`,
    token,
    null,
  );
});
