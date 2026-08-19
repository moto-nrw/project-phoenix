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

/**
 * "none" is a plain Mitteilung; the two choice types make it an Umfrage that
 * parents answer per child (#1371).
 */
export type AnnouncementResponseType =
  "none" | "single_choice" | "multi_choice";

interface AnnouncementOption {
  id: string;
  label: string;
}

/**
 * How the announcement is delivered. "letter" is the Elternbrief (#2384):
 * e-mail and confirmation are mandatory and the mail carries the full text.
 */
export type AnnouncementDeliveryMode = "standard" | "letter";

/**
 * Who receives the e-mail — a separate axis from who sees the announcement in
 * the portal. "portal_only" is the default and the safe one; "all_contacts"
 * additionally reaches guardians that have an address but no portal access.
 */
export type AnnouncementEmailAudience = "portal_only" | "all_contacts";

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
  response_type: AnnouncementResponseType;
  response_deadline?: string;
  options: AnnouncementOption[];
  delivery_mode: AnnouncementDeliveryMode;
  email_audience: AnnouncementEmailAudience;
}

/** True when the announcement is a binding Elternbrief. */
export function isLetter(announcement: Announcement): boolean {
  return announcement.delivery_mode === "letter";
}

/** True when the announcement asks the parents a question. */
export function isPoll(announcement: Announcement): boolean {
  return announcement.response_type !== "none";
}

/** One option's tally: how many reached children picked it. */
interface PollOptionResult {
  option_id: string;
  label: string;
  count: number;
}

export interface PollResults {
  child_count: number;
  target_child_count: number;
  answered_count: number;
  options: PollOptionResult[];
}

/** One reached child with the answer given for them (empty = still open). */
export interface PollChild {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  answer_labels: string[];
  responded_at?: string;
  can_answer: boolean;
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
  /** Omit or "none" for a plain Mitteilung; a choice type requires options. */
  response_type?: AnnouncementResponseType;
  response_deadline?: string | null;
  /** Answer labels in display order — the backend assigns the option ids. */
  options?: string[];
  /**
   * "letter" makes this an Elternbrief. The backend then forces send_email and
   * requires_acknowledgement on, so the client need not repeat them.
   */
  delivery_mode?: AnnouncementDeliveryMode;
  email_audience?: AnnouncementEmailAudience;
}

/**
 * One addressed person in the recipient matrix. The two channels are reported
 * separately on purpose (#2384): "die E-Mail ist raus" and "jemand hat in moto
 * bestätigt" are different facts.
 *
 * email_status "sent" means handed to the mail server — label it "Versendet",
 * never "Zugestellt", until provider delivery events exist (#1937).
 */
export interface LetterRecipient {
  first_name: string;
  last_name: string;
  email?: string;
  email_status: "pending" | "sent" | "failed" | "no_email" | "excluded";
  reachability: "ok" | "no_email" | "no_portal" | "excluded";
  last_error?: string;
  sent_at?: string;
}

/** One reached child with the derived fulfilment of the letter. */
export interface LetterChild {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  fulfilled: boolean;
  acknowledged_at?: string;
  acknowledged_by?: string;
}

export interface LetterSummary {
  children_total: number;
  children_fulfilled: number;
  children_open: number;
  recipients_total: number;
  emails_sent: number;
  emails_pending: number;
  emails_failed: number;
  without_email: number;
  without_portal: number;
}

export interface LetterStatus {
  recipients: LetterRecipient[];
  children: LetterChild[];
  summary: LetterSummary;
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

export async function fetchPollResults(id: string): Promise<PollResults> {
  const data = await request<PollResults>(
    `${BASE}/${encodeURIComponent(id)}/poll-results`,
    undefined,
    "Umfrageergebnis konnte nicht geladen werden",
  );
  return (
    data ?? {
      child_count: 0,
      target_child_count: 0,
      answered_count: 0,
      options: [],
    }
  );
}

export async function fetchPollChildren(id: string): Promise<PollChild[]> {
  const data = await request<PollChild[]>(
    `${BASE}/${encodeURIComponent(id)}/poll-children`,
    undefined,
    "Antwortliste konnte nicht geladen werden",
  );
  return data ?? [];
}

/**
 * Notifies the guardians of children without an answer. Returns how many people
 * were reached so the UI can say "8 Eltern erinnert" instead of a silent OK.
 */
export async function remindUnanswered(id: string): Promise<number> {
  const data = await request<{ reminded_count: number }>(
    `${BASE}/${encodeURIComponent(id)}/remind`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    },
    "Erinnerung konnte nicht gesendet werden",
  );
  return data?.reminded_count ?? 0;
}

/** The Elternbrief recipient matrix: per person, per child, plus the counts. */
export async function fetchLetterStatus(id: string): Promise<LetterStatus> {
  const data = await request<LetterStatus>(
    `${BASE}/${encodeURIComponent(id)}/letter-status`,
    undefined,
    "Status konnte nicht geladen werden",
  );
  return (
    data ?? {
      recipients: [],
      children: [],
      summary: {
        children_total: 0,
        children_fulfilled: 0,
        children_open: 0,
        recipients_total: 0,
        emails_sent: 0,
        emails_pending: 0,
        emails_failed: 0,
        without_email: 0,
        without_portal: 0,
      },
    }
  );
}

/** Re-queue only the mails that failed. Separate from remindUnanswered. */
export async function resendFailedEmails(id: string): Promise<number> {
  const data = await request<{ resent_count: number }>(
    `${BASE}/${encodeURIComponent(id)}/resend-failed`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    },
    "Erneuter Versand fehlgeschlagen",
  );
  return data?.resent_count ?? 0;
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
