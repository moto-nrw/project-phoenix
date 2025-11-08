import { createGetHandler, createPutHandler } from "~/lib/route-wrapper";
import { apiGet, apiPut } from "~/lib/api-helpers";
import type { ApiResponse } from "~/lib/api-helpers";
import type {
  BackendProfile,
  ProfileUpdateRequest,
} from "~/lib/profile-helpers";

export const GET = createGetHandler(async (request, token, _params) => {
  const response = await apiGet<ApiResponse<BackendProfile>>(
    `/api/me/profile`,
    token,
  );
  return response.data;
});

export const PUT = createPutHandler<BackendProfile, ProfileUpdateRequest>(
  async (request, body, token, _params) => {
    const response = await apiPut<
      ApiResponse<BackendProfile>,
      ProfileUpdateRequest
    >("/api/me/profile", token, body);
    return response.data;
  },
);
