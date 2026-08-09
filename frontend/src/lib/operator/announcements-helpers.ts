export type AnnouncementType = "announcement" | "release" | "maintenance";
export type AnnouncementSeverity = "info" | "warning" | "critical";
type AnnouncementStatus = "draft" | "published" | "expired";
export type SystemRole = "admin" | "user" | "guardian";

export const SYSTEM_ROLE_LABELS: Record<SystemRole, string> = {
  admin: "Administratoren",
  user: "Betreuer",
  guardian: "Erziehungsberechtigte",
};

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
  target_roles: string[];
  target_org_ids: number[];
  target_tenant_ids: number[];
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
  targetRoles: SystemRole[];
  targetOrgIds: string[];
  targetTenantIds: string[];
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  status: AnnouncementStatus;
}

export interface AnnouncementStats {
  announcement_id: number;
  target_count: number;
  seen_count: number;
  dismissed_count: number;
}

export interface BackendAnnouncementViewDetail {
  user_id: number;
  user_name: string;
  seen_at: string;
  dismissed: boolean;
}

export interface AnnouncementViewDetail {
  userId: string;
  userName: string;
  seenAt: string;
  dismissed: boolean;
}

export function mapViewDetail(
  data: BackendAnnouncementViewDetail,
): AnnouncementViewDetail {
  return {
    userId: data.user_id.toString(),
    userName: data.user_name,
    seenAt: data.seen_at,
    dismissed: data.dismissed,
  };
}

export interface CreateAnnouncementRequest {
  title: string;
  content: string;
  type: AnnouncementType;
  severity: AnnouncementSeverity;
  version?: string;
  expires_at?: string;
  target_roles?: SystemRole[];
  target_org_ids?: number[];
  target_tenant_ids?: number[];
}

export interface UpdateAnnouncementRequest {
  title: string;
  content: string;
  type: AnnouncementType;
  severity: AnnouncementSeverity;
  version?: string | null;
  active?: boolean;
  expires_at?: string | null;
  target_roles?: SystemRole[];
  target_org_ids?: number[];
  target_tenant_ids?: number[];
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
    targetRoles: (data.target_roles ?? []) as SystemRole[],
    targetOrgIds: (data.target_org_ids ?? []).map(String),
    targetTenantIds: (data.target_tenant_ids ?? []).map(String),
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

export const TYPE_TEXT_COLORS: Record<AnnouncementType, string> = {
  announcement: "text-moto-blue", // moto blue
  release: "text-moto-green", // moto green
  maintenance: "text-moto-orange", // moto orange
};
