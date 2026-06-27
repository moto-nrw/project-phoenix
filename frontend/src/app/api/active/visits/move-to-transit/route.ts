import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface MoveToTransitBody {
  student_ids: number[];
}

export const POST = createPostHandler<unknown, MoveToTransitBody>(
  async (_request, body, token) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/active/visits/move-to-transit",
      token,
      body,
    );
    return response.data;
  },
);
