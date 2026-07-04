import { createParentCalendarGetHandler } from "~/lib/parent/calendar-route-proxy.server";

export const GET = createParentCalendarGetHandler<unknown>((request) => {
  const search = request.nextUrl.searchParams.toString();
  return `/parent/me/calendar${search ? `?${search}` : ""}`;
});
