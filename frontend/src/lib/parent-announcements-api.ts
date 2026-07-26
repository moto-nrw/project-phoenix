/**
 * Staff-side client for parent announcements (Elternmitteilungen, #1669):
 * tenant-authored broadcast news to guardians. Talks to the Next.js proxy
 * routes under /api/parent-announcements, which forward to the Go backend with
 * the staff JWT. Backend int64 ids arrive already stringified.
 */

export type AnnouncementPriority = "info" | "important";
export type AnnouncementStatus = "draft" | "published" | "expired";

export type AnnouncementTargetType =
  | "school_all"
  | "class"
  | "group"
  | "activity_group"
  | "student"
  | "pending_enrollment";

export interface AnnouncementTarget {
  target_type: AnnouncementTargetType;
  ref_id?: string;
  ref_text?: string;
}

export interface Announcement {
  id: string;
  title: string;
  body: string;
  priority: AnnouncementPriority;
  link_url?: string;
  requires_acknowledgement: boolean;
  send_email: boolean;
  status: AnnouncementStatus;
  published_at?: string;
  expires_at?: string;
  active: boolean;
  created_at: string;
  updated_at: string;
  targets: AnnouncementTarget[];
}

export interface AnnouncementStats {
  target_count: number;
  read_count: number;
  acknowledged_count: number;
}

type AnnouncementRecipientStatus = "pending" | "read" | "acknowledged";

/** One guardian account in the live audience with their read/ack state. */
export interface AnnouncementRecipient {
  account_id: string;
  first_name: string;
  last_name: string;
  status: AnnouncementRecipientStatus;
  read_at?: string;
  acknowledged_at?: string;
}

export interface AnnouncementInput {
  title: string;
  body: string;
  priority: AnnouncementPriority;
  link_url?: string | null;
  requires_acknowledgement: boolean;
  send_email: boolean;
  expires_at?: string | null;
  targets: AnnouncementTarget[];
}

interface ApiResponse<T> {
  status?: string;
  data?: T;
  error?: string;
}

async function throwApiError(
  response: Response,
  fallback: string,
): Promise<never> {
  let message = fallback;
  try {
    const body = (await response.json()) as { error?: string };
    if (body.error) message = body.error;
  } catch {
    // Body was not JSON — keep the German fallback.
  }
  throw new Error(message);
}

async function request<T>(
  url: string,
  init: RequestInit | undefined,
  fallback: string,
): Promise<T | undefined> {
  const response = await fetch(url, init);
  if (!response.ok) await throwApiError(response, fallback);
  const body = (await response.json()) as ApiResponse<T>;
  return body.data;
}

const BASE = "/api/parent-announcements";

export async function fetchAnnouncements(
  includeInactive = true,
): Promise<Announcement[]> {
  const url = includeInactive ? `${BASE}?include_inactive=true` : BASE;
  const data = await request<Announcement[]>(
    url,
    undefined,
    "Elternmitteilungen konnten nicht geladen werden",
  );
  return data ?? [];
}

function jsonBody(body: AnnouncementInput): RequestInit {
  return {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
}

export async function createAnnouncement(
  input: AnnouncementInput,
): Promise<Announcement> {
  const data = await request<Announcement>(
    BASE,
    jsonBody(input),
    "Elternmitteilung konnte nicht erstellt werden",
  );
  if (!data) throw new Error("Elternmitteilung konnte nicht erstellt werden");
  return data;
}

export async function updateAnnouncement(
  id: string,
  input: AnnouncementInput,
): Promise<Announcement> {
  const data = await request<Announcement>(
    `${BASE}/${encodeURIComponent(id)}`,
    { ...jsonBody(input), method: "PUT" },
    "Elternmitteilung konnte nicht gespeichert werden",
  );
  if (!data)
    throw new Error("Elternmitteilung konnte nicht gespeichert werden");
  return data;
}

export async function deleteAnnouncement(id: string): Promise<void> {
  await request<unknown>(
    `${BASE}/${encodeURIComponent(id)}`,
    { method: "DELETE" },
    "Elternmitteilung konnte nicht gelöscht werden",
  );
}

export async function publishAnnouncement(id: string): Promise<Announcement> {
  const data = await request<Announcement>(
    `${BASE}/${encodeURIComponent(id)}/publish`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    },
    "Elternmitteilung konnte nicht veröffentlicht werden",
  );
  if (!data)
    throw new Error("Elternmitteilung konnte nicht veröffentlicht werden");
  return data;
}

export async function unpublishAnnouncement(id: string): Promise<Announcement> {
  const data = await request<Announcement>(
    `${BASE}/${encodeURIComponent(id)}/unpublish`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    },
    "Elternmitteilung konnte nicht zurückgezogen werden",
  );
  if (!data)
    throw new Error("Elternmitteilung konnte nicht zurückgezogen werden");
  return data;
}

export async function fetchAnnouncementStats(
  id: string,
): Promise<AnnouncementStats> {
  const data = await request<AnnouncementStats>(
    `${BASE}/${encodeURIComponent(id)}/stats`,
    undefined,
    "Statistik konnte nicht geladen werden",
  );
  return data ?? { target_count: 0, read_count: 0, acknowledged_count: 0 };
}

export async function fetchAnnouncementRecipients(
  id: string,
): Promise<AnnouncementRecipient[]> {
  const data = await request<AnnouncementRecipient[]>(
    `${BASE}/${encodeURIComponent(id)}/recipients`,
    undefined,
    "Empfängerliste konnte nicht geladen werden",
  );
  return data ?? [];
}
