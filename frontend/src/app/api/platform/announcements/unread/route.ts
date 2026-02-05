import { createProxyGetDataHandler } from "~/lib/route-wrapper";

export const GET = createProxyGetDataHandler(
  "/api/platform/announcements/unread",
);
