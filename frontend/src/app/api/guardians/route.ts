import {
  createGetHandler,
  createPostHandler,
} from "@/lib/route-wrapper.server";
import { apiGet, apiPost } from "@/lib/api-helpers.server";

// GET /api/guardians - List guardians
export const GET = createGetHandler(async (request, token, _params) => {
  const searchParams = request.nextUrl.searchParams;
  const queryString = searchParams.toString();

  const endpoint = queryString
    ? `/api/guardians?${queryString}`
    : "/api/guardians";

  const response = await apiGet(endpoint, token);
  return response;
});

// POST /api/guardians - Create guardian
export const POST = createPostHandler(async (request, body, token, _params) => {
  const response = await apiPost("/api/guardians", token, body);
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});
