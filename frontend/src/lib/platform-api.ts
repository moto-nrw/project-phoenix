import { authFetch } from "./api-helpers";

export interface PlatformAnnouncement {
  id: string;
  title: string;
  content: string;
  type: "announcement" | "release" | "maintenance";
  severity: "info" | "warning" | "critical";
  version: string | null;
  publishedAt: string;
}

interface BackendPlatformAnnouncement {
  id: number;
  title: string;
  content: string;
  type: "announcement" | "release" | "maintenance";
  severity: "info" | "warning" | "critical";
  version?: string | null;
  published_at: string;
}

interface ProxyListResponse {
  success: boolean;
  data: BackendPlatformAnnouncement[];
}

function mapPlatformAnnouncement(
  data: BackendPlatformAnnouncement,
): PlatformAnnouncement {
  return {
    id: data.id.toString(),
    title: data.title,
    content: data.content,
    type: data.type,
    severity: data.severity,
    version: data.version ?? null,
    publishedAt: data.published_at,
  };
}

export async function fetchUnreadAnnouncements(): Promise<
  PlatformAnnouncement[]
> {
  const response = await authFetch<ProxyListResponse>(
    "/api/platform/announcements/unread",
  );
  return response.data.map(mapPlatformAnnouncement);
}

export async function markAnnouncementSeen(id: string): Promise<void> {
  await authFetch(`/api/platform/announcements/${id}/seen`, {
    method: "POST",
  });
}

export async function dismissAnnouncement(id: string): Promise<void> {
  await authFetch(`/api/platform/announcements/${id}/dismiss`, {
    method: "POST",
  });
}
