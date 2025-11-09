import { NextRequest } from "next/server";
import { createGetHandler, createPostHandler } from "@/lib/route-wrapper";
import { apiGet, apiPost } from "@/lib/api-helpers";

// GET /api/guardians - List guardians
export const GET = createGetHandler(async (request, token, params) => {
  const searchParams = request.nextUrl.searchParams;
  const queryString = searchParams.toString();

  const endpoint = queryString
    ? `/api/guardians?${queryString}`
    : "/api/guardians";

  const response = await apiGet(endpoint, token);
  return response;
});

// POST /api/guardians - Create guardian
export const POST = createPostHandler(async (request, body, token, params) => {
  const response = await apiPost("/api/guardians", token, body);
  return response.data;
});
