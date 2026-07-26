import type { NextRequest } from "next/server";
import { createGetHandler } from "@/lib/route-wrapper.server";
import { apiGet, apiPut } from "~/lib/api-helpers.server";
import { auth } from "@/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";

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
async function POSTHandler(request: NextRequest) {
  const session = await auth();
  if (!session?.user?.token) {
    return Response.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const body = (await request.json()) as {
      id: string;
      [key: string]: unknown;
    };
    const { id, ...updateData } = body;
    const response = await apiPut(
      `/auth/accounts/${id}`,
      session.user.token,
      updateData,
    );
    return Response.json(response);
  } catch {
    return Response.json(
      { error: "Failed to update account" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
