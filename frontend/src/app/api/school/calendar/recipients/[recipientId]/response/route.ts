import { proxyPost } from "~/lib/school/route-wrapper.server";

// Zu- oder Absage einer Lehrkraft auf eine Einladung der OGS (#4798).
export const POST = proxyPost(
  (params) =>
    `/school/calendar/recipients/${encodeURIComponent(params.recipientId as string)}/response`,
);
