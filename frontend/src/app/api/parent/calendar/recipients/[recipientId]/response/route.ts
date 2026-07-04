import {
  createParentPostHandler,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";
import { isStringParam } from "~/lib/route-wrapper.server";

export const POST = createParentPostHandler(
  async (_request, body: unknown, token: string, params) => {
    const recipientId = isStringParam(params.recipientId)
      ? params.recipientId
      : "";
    return parentApiPost(
      `/parent/me/calendar/recipients/${encodeURIComponent(recipientId)}/response`,
      token,
      body ?? {},
    );
  },
);
