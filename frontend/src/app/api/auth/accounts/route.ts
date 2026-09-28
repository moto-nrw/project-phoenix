import { createGetHandler } from "@/lib/route-wrapper.server";
import { apiGet, apiPut } from "~/lib/api-helpers.server";
import { createTenantApiAdapter } from "~/lib/backend-proxy-route.server";

export const GET = createGetHandler(async (request, token, _params) => {
  // Extract query parameters
  const searchParams = request.nextUrl.searchParams;
  const email = searchParams.get("email");
  const active = searchParams.get("active");

  // Build query parameters for backend
  const queryParams = new URLSearchParams();
  if (email) queryParams.set("email", email);
  if (active) queryParams.set("active", active);

  const url = queryParams.toString()
    ? `/auth/accounts?${queryParams.toString()}`
    : "/auth/accounts";

  return await apiGet<{ data: unknown }>(url, token);
});

// POST handler for updating accounts
export const POST = createTenantApiAdapter(
  async (request, token) => {
    let body: {
      id: string;
      [key: string]: unknown;
    };
    try {
      body = (await request.json()) as typeof body;
    } catch {
      return Response.json({ error: "Invalid JSON" }, { status: 400 });
    }
    const { id, ...updateData } = body;
    const response = await apiPut(`/auth/accounts/${id}`, token, updateData);
    return Response.json(response);
  },
  () => Response.json({ error: "Failed to update account" }, { status: 500 }),
);
