/**
 * Staff-side client for the Gruppennachrichten (#4784): the OGS writes to
 * every family of a group, class or AG. Parents answer only the OGS, in the
 * conversation of their child; their answers never appear here.
 *
 * Talks to the Next.js proxy routes under /api/messages/group-threads.
 * Backend int64 ids arrive already stringified.
 */

import { ApiError, apiErrorFromText, transportFetch } from "~/lib/api-error";

export type GroupTargetType = "group" | "class" | "activity_group";

interface GroupThread {
  id: string;
  target_type: GroupTargetType;
  /** Group or AG id, or the class name for a class. */
  target_id: string;
  target_label: string;
  title: string;
  closed_at?: string;
  last_message_at?: string;
  created_at: string;
}

/**
 * Who a group thread reached (#5200): the group, class or AG and how many
 * families received it. Parents' answers go to each child's team
 * conversation.
 */
export interface GroupReaders {
  target_type: GroupTargetType;
  target_label: string;
  recipient_count: number;
}

export interface GroupThreadSummary extends GroupThread {
  last_message_body: string;
  message_count: number;
  family_count: number;
  read_count: number;
  readers?: GroupReaders;
}

interface GroupMessage {
  client_message_id?: string;
  id: string;
  sender_name: string;
  body: string;
  created_at: string;
}

export interface GroupChild {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class?: string;
  reachable: boolean;
  read: boolean;
}

export interface GroupThreadDetail {
  thread: GroupThread;
  messages: GroupMessage[];
  children: GroupChild[];
  family_count: number;
  read_count: number;
  readers?: GroupReaders;
}

export interface GroupTarget {
  type: GroupTargetType;
  id: string;
  label: string;
}

interface ApiResponse<T> {
  data?: T;
}

async function request<T>(
  url: string,
  fallback: string,
  init?: RequestInit,
): Promise<T> {
  const response = await transportFetch(url, init);
  if (!response.ok) {
    let text = "";
    try {
      text = await response.text();
    } catch {
      // Body unreadable: the status still classifies the error.
    }
    throw apiErrorFromText(fallback, response.status, text);
  }
  const result = (await response.json()) as ApiResponse<T>;
  if (result.data === undefined) throw new ApiError(fallback, 500);
  return result.data;
}

function post<T>(url: string, body: unknown, fallback: string): Promise<T> {
  return request<T>(url, fallback, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

const BASE = "/api/messages/group-threads";

export function fetchGroupThreads(): Promise<GroupThreadSummary[]> {
  return request<GroupThreadSummary[]>(
    BASE,
    "Gruppennachrichten konnten nicht geladen werden",
  );
}

export function fetchGroupAudience(
  type: GroupTargetType,
  id: string,
): Promise<GroupChild[]> {
  const params = new URLSearchParams({ target_type: type, target_id: id });
  return request<GroupChild[]>(
    `${BASE}/audience?${params.toString()}`,
    "Empfänger konnten nicht geladen werden",
  );
}

export function createGroupThread(input: {
  client_message_id?: string;
  target: GroupTarget;
  title: string;
  body: string;
}): Promise<GroupThreadDetail> {
  return post<GroupThreadDetail>(
    BASE,
    {
      target_type: input.target.type,
      target_id: input.target.id,
      target_label: input.target.label,
      title: input.title,
      body: input.body,
      client_message_id: input.client_message_id,
    },
    "Gruppennachricht konnte nicht gesendet werden",
  );
}

export function fetchGroupThread(id: string): Promise<GroupThreadDetail> {
  return request<GroupThreadDetail>(
    `${BASE}/${encodeURIComponent(id)}`,
    "Gruppennachricht konnte nicht geladen werden",
  );
}

export function postGroupMessage(
  id: string,
  body: string,
  clientMessageId?: string,
): Promise<GroupThreadDetail> {
  return post<GroupThreadDetail>(
    `${BASE}/${encodeURIComponent(id)}/messages`,
    { body, client_message_id: clientMessageId },
    "Nachricht konnte nicht gesendet werden",
  );
}

export function closeGroupThread(id: string): Promise<GroupThreadDetail> {
  return post<GroupThreadDetail>(
    `${BASE}/${encodeURIComponent(id)}/close`,
    {},
    "Gruppennachricht konnte nicht geschlossen werden",
  );
}

/** "3 von 5 Familien gelesen". */
export function familyReadLabel(read: number, families: number): string {
  return `${read} von ${families} ${families === 1 ? "Familie" : "Familien"} gelesen`;
}

/**
 * „Ging an: Gruppe Bären (12 Familien).“ — where a group thread went (#5200). The
 * target label already names its kind („Gruppe“, „Klasse“, „AG“).
 */
export function groupRecipientsLabel(readers: GroupReaders): string {
  const families = `${readers.recipient_count} ${readers.recipient_count === 1 ? "Familie" : "Familien"}`;
  return `Ging an: ${readers.target_label} (${families}).`;
}
