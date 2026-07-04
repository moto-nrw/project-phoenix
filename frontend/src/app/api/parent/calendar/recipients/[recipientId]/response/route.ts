import { proxyPost } from "~/lib/parent/route-wrapper.server";

export const POST = proxyPost<unknown>((params) => {
  const recipientId = String(params.recipientId);
  return `/parent/me/calendar/recipients/${encodeURIComponent(recipientId)}/response`;
});
