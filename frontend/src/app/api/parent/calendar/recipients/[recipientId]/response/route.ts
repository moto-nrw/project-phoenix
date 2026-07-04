import { createParentCalendarPostHandler } from "~/lib/parent/calendar-route-proxy.server";
import { isStringParam } from "~/lib/route-wrapper.server";

export const POST = createParentCalendarPostHandler<unknown>(
  (_request, params) => {
    const recipientId = isStringParam(params.recipientId)
      ? params.recipientId
      : "";
    return `/parent/me/calendar/recipients/${encodeURIComponent(recipientId)}/response`;
  },
);
