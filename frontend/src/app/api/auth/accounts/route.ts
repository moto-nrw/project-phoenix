import { apiGet, apiPut, bffError } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

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

export const POST = createPostHandler<unknown, { id?: unknown }>(
  async (_request, body, token) => {
    const { id, ...updateData } = body;
    if (typeof id !== "string" && typeof id !== "number") {
      throw bffError(400, "id is required");
    }
    return Response.json(
      await apiPut(
        `/auth/accounts/${encodeURIComponent(String(id))}`,
        token,
        updateData,
      ),
    );
  },
);
