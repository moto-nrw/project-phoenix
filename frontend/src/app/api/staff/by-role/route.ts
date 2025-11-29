import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-helpers";

export const GET = createGetHandler(async (request, token, _params) => {
  const { searchParams } = new URL(request.url);
  const role = searchParams.get("role");

  if (!role) {
    throw new Error("Role parameter is required");
  }

  const response = await apiGet<{ data: unknown }>(
    `/api/staff/by-role?role=${role}`,
    token,
  );
  return response.data;
});
