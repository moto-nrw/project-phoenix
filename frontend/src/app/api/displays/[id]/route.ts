// app/api/displays/[id]/route.ts — update/delete one info-point display
import { apiPatch, apiDelete } from "~/lib/api-helpers.server";
import {
  createPatchHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper.server";
import type { BackendDisplay } from "~/lib/display-helpers";

interface DisplayUpdateRequest {
  name?: string;
  is_active?: boolean;
}

interface DisplayUpdateResponse {
  display: BackendDisplay;
}

function requireDisplayID(params: Record<string, unknown>): string {
  const id = params.id;
  if (typeof id !== "string" || !/^\d+$/.test(id)) {
    throw new Error("invalid display id");
  }
  return id;
}

export const PATCH = createPatchHandler<
  DisplayUpdateResponse,
  DisplayUpdateRequest
>(async (_request, body, token, params) => {
  const id = requireDisplayID(params);
  return apiPatch<DisplayUpdateResponse, DisplayUpdateRequest>(
    `/api/display/${id}`,
    token,
    body,
  );
});

export const DELETE = createDeleteHandler(async (_request, token, params) => {
  const id = requireDisplayID(params);
  await apiDelete(`/api/display/${id}`, token);
  return { deleted: true };
});
