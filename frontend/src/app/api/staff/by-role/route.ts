import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-helpers";

export const GET = createGetHandler(async (request, token, _params) => {
  const { searchParams } = new URL(request.url);
  const role = searchParams.get("role");
  const roles = searchParams.get("roles");

  if (!role && !roles) {
    throw new Error("Role or roles parameter is required");
  }

  // Build query string - forward whichever param is present
  const params = new URLSearchParams();
  if (roles) {
    params.set("roles", roles);
  } else if (role) {
    params.set("role", role);
  }

  const response = await apiGet<{ data: unknown }>(
    `/api/staff/by-role?${params.toString()}`,
    token,
  );
  return response.data;
});
