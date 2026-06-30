import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (_request, body: unknown, token: string, params) => {
    const recipientId = isStringParam(params.recipientId)
      ? params.recipientId
      : "";
    const response = await apiPost<{ data: unknown }>(
      `/api/calendar/recipients/${encodeURIComponent(recipientId)}/response`,
      token,
      body ?? {},
    );
    return response.data;
  },
);
