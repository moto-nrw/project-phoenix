import { createGetHandler, createPutHandler } from "@/lib/route-wrapper.server";
import { apiGet, apiPut } from "~/lib/api-helpers.server";

export const GET = createGetHandler(async (request, token, params) => {
  const roleId = params.roleId as string;
  return await apiGet<{ data: unknown }>(
    `/auth/roles/${roleId}/permissions`,
    token,
  );
});

interface ReplaceRolePermissionsBody {
  permission_ids: readonly string[];
}

export const PUT = createPutHandler<unknown, ReplaceRolePermissionsBody>(
  async (_request, body, token, params) => {
    const roleId = params.roleId as string;
    return await apiPut<unknown>(
      `/auth/roles/${roleId}/permissions`,
      token,
      body,
    );
  },
);
