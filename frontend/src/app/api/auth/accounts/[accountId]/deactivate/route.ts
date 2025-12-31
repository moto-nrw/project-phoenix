import { createPutHandler } from "@/lib/route-wrapper";
import { apiPut } from "@/lib/api-client";

export const PUT = createPutHandler(async (request, body, token, params) => {
  const accountId = params.accountId as string;
  const response = await apiPut<{ message: string }>(
    `/auth/accounts/${accountId}/deactivate`,
    null,
    token,
  );
  return response.data;
});
