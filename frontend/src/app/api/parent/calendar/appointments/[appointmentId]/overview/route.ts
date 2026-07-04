import { proxyGet } from "~/lib/parent/route-wrapper.server";

export const GET = proxyGet<unknown>((params) => {
  const appointmentId = String(params.appointmentId);
  return `/parent/me/calendar/appointments/${encodeURIComponent(appointmentId)}/overview`;
});
