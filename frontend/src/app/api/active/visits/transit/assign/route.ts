import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface AssignTransitBody {
  student_ids: number[];
  active_group_id: number;
}

export const POST = createPostHandler<unknown, AssignTransitBody>(
  async (_request, body, token) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/active/visits/transit/assign",
      token,
      body,
    );
    return response.data;
  },
);
