// app/api/displays/route.ts — admin CRUD proxy for info-point displays
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";
import type {
  BackendDisplay,
  BackendDisplayCreateResponse,
} from "~/lib/display-helpers";

interface DisplayListResponse {
  displays: BackendDisplay[];
}

interface DisplayCreateRequest {
  name: string;
}

export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<DisplayListResponse>("/api/display", token);
  return response.displays ?? [];
});

export const POST = createPostHandler<
  BackendDisplayCreateResponse,
  DisplayCreateRequest
>(async (_request, body, token) => {
  return apiPost<BackendDisplayCreateResponse, DisplayCreateRequest>(
    "/api/display",
    token,
    { name: body.name },
  );
});
