import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler, isStringParam } from "~/lib/route-wrapper.server";

export const POST = createPostHandler(
  async (_request, _body: unknown, token: string, params) => {
    const appointmentId = isStringParam(params.appointmentId)
      ? params.appointmentId
      : "";
    const response = await apiPost<{ data: unknown }>(
      `/api/calendar/appointments/${encodeURIComponent(appointmentId)}/cancel`,
      token,
      {},
    );
    return response.data;
  },
);
