import { proxyGet } from "~/lib/route-proxy.server";

interface UnreadAnnouncement {
  id: number;
  title: string;
  content: string;
  type: string;
  severity: string;
  version?: string;
  published_at: string;
}

export const GET = proxyGet<UnreadAnnouncement[]>(
  "/api/platform/announcements/unread",
);
