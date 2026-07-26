import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface MoveToGroupBody {
  student_ids: number[];
  target_active_group_id: number;
}

export const POST = createPostHandler<unknown, MoveToGroupBody>(
  async (_request, body, token) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/active/visits/move-to-group",
      token,
      body,
    );
    return response.data;
  },
);
