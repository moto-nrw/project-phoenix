import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

export const GET = createGetHandler(async (_request, token, params) => {
  const appointmentId = params.appointmentId as string;
  const response = await apiGet<{ data: unknown }>(
    `/api/calendar/appointments/${encodeURIComponent(appointmentId)}/overview`,
    token,
  );
  return response.data;
});
