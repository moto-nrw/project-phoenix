import { createGetHandler } from "@/lib/route-wrapper.server";
import { apiGet } from "~/lib/api-helpers.server";

export const GET = createGetHandler(async (request, token, params) => {
  const roleId = params.roleId as string;
  return await apiGet<{ data: unknown }>(
    `/auth/roles/${roleId}/permissions`,
    token,
  );
});
