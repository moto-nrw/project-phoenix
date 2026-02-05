export type AnnouncementType = "announcement" | "release" | "maintenance";
export type AnnouncementSeverity = "info" | "warning" | "critical";
export type AnnouncementStatus = "draft" | "published" | "expired";

export interface BackendAnnouncement {
  id: number;
  title: string;
  content: string;
  type: AnnouncementType;
  severity: AnnouncementSeverity;
  version?: string | null;
  active: boolean;
  published_at?: string | null;
  expires_at?: string | null;
  created_by: number;
  created_at: string;
  updated_at: string;
  status: AnnouncementStatus;
}

export interface Announcement {
  id: string;
  title: string;
  content: string;
  type: AnnouncementType;
  severity: AnnouncementSeverity;
  version: string | null;
  active: boolean;
  publishedAt: string | null;
  expiresAt: string | null;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  status: AnnouncementStatus;
}

export interface CreateAnnouncementRequest {
  title: string;
  content: string;
  type: AnnouncementType;
  severity: AnnouncementSeverity;
  version?: string;
  expires_at?: string;
}

export interface UpdateAnnouncementRequest {
  title: string;
  content: string;
  type: AnnouncementType;
  severity: AnnouncementSeverity;
  version?: string | null;
  active?: boolean;
  expires_at?: string | null;
}

export function mapAnnouncement(data: BackendAnnouncement): Announcement {
  return {
    id: data.id.toString(),
    title: data.title,
    content: data.content,
    type: data.type,
    severity: data.severity,
    version: data.version ?? null,
    active: data.active,
    publishedAt: data.published_at ?? null,
    expiresAt: data.expires_at ?? null,
    createdBy: data.created_by.toString(),
    createdAt: data.created_at,
    updatedAt: data.updated_at,
    status: data.status,
  };
}

export const TYPE_LABELS: Record<AnnouncementType, string> = {
  announcement: "Ankündigung",
  release: "Release",
  maintenance: "Wartung",
};

export const SEVERITY_LABELS: Record<AnnouncementSeverity, string> = {
  info: "Info",
  warning: "Warnung",
  critical: "Kritisch",
};

export const ANNOUNCEMENT_STATUS_LABELS: Record<AnnouncementStatus, string> = {
  draft: "Entwurf",
  published: "Veröffentlicht",
  expired: "Abgelaufen",
};

export const TYPE_STYLES: Record<AnnouncementType, string> = {
  announcement: "bg-blue-100 text-blue-700",
  release: "bg-green-100 text-green-700",
  maintenance: "bg-orange-100 text-orange-700",
};

export const SEVERITY_STYLES: Record<AnnouncementSeverity, string> = {
  info: "bg-gray-100 text-gray-700",
  warning: "bg-yellow-100 text-yellow-800",
  critical: "bg-red-100 text-red-700",
};

export const ANNOUNCEMENT_STATUS_STYLES: Record<AnnouncementStatus, string> = {
  draft: "bg-gray-100 text-gray-700",
  published: "bg-green-100 text-green-700",
  expired: "bg-red-100 text-red-700",
};
