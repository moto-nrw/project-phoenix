import { createParentCalendarGetHandler } from "~/lib/parent/calendar-route-proxy.server";

export const GET = createParentCalendarGetHandler<unknown>(
  (_request, params) => {
    const appointmentId = params.appointmentId as string;
    return `/parent/me/calendar/appointments/${encodeURIComponent(appointmentId)}/overview`;
  },
);
