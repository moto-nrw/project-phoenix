import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

export const GET = createParentGetHandler(async (_request, token, params) => {
  const appointmentId = params.appointmentId as string;
  return parentApiGet<unknown>(
    `/parent/me/calendar/appointments/${encodeURIComponent(appointmentId)}/overview`,
    token,
  );
});
