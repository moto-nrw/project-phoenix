import { createPutHandler } from "@/lib/route-wrapper";
import { apiPut } from "~/lib/api-helpers";

export const PUT = createPutHandler(async (request, body, token, params) => {
  const accountId = params.accountId as string;
  return await apiPut<{ message: string }>(
    `/auth/accounts/${accountId}/activate`,
    token,
    null,
  );
});
