import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendAnnouncement {
  id: string;
  title: string;
  body: string;
  priority: "info" | "important";
  requires_acknowledgement: boolean;
  school_name: string;
  published_at?: string;
  expires_at?: string;
  read: boolean;
  acknowledged: boolean;
}

/**
 * Proxy GET /api/parent/me/news → backend. Returns the guardian's parent-news
 * feed across all their (news-enabled) children's schools, newest-published
 * first, each with the guardian's read/ack state. Audience + visibility are
 * enforced server-side from the JWT account.
 */
export const GET = proxyGet<BackendAnnouncement[]>("/parent/me/news");
