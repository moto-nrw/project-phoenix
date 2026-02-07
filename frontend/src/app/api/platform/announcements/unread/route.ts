import { createProxyGetDataHandler } from "~/lib/route-wrapper";

interface UnreadAnnouncement {
  id: number;
  title: string;
  content: string;
  type: string;
  severity: string;
  version?: string;
  published_at: string;
}

export const GET = createProxyGetDataHandler<UnreadAnnouncement[]>(
  "/api/platform/announcements/unread",
);
