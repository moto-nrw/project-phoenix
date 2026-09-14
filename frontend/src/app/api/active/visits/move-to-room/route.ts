import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface MoveToRoomBody {
  student_ids: number[];
  target_room_id: number;
}

// Independent move into a released room (#3066). The backend decides the
// release, the caller's rights and the room session; this route only forwards.
export const POST = createPostHandler<unknown, MoveToRoomBody>(
  async (_request, body, token) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/active/visits/move-to-room",
      token,
      body,
    );
    return response.data;
  },
);
