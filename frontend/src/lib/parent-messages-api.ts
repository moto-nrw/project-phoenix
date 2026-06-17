/**
 * Staff-side client for the parent-OGS messaging feature. Messaging is
 * organized as email-like threads: a thread is one conversation between the
 * OGS and ONE guardian about ONE child, identified by a subject. A guardian
 * can have several threads about the same child.
 *
 * Talks to the Next.js proxy routes under /api/messages, which forward to the
 * Go backend with the staff JWT. Backend int64 ids arrive already stringified.
 */

export interface InboxThread {
  thread_id: string;
  subject: string;
  student_id: string;
  student_name: string;
  school_class?: string;
  group_name?: string;
  guardian_name: string;
  relationship_type?: "parent" | "guardian" | "relative" | "other";
  last_message_at?: string;
  last_sender_kind?: "guardian" | "staff";
  last_message_body?: string;
  unread_count: number;
}

export interface Message {
  id: string;
  sender_kind: "guardian" | "staff";
  sender_name: string;
  body: string;
  created_at: string;
}

export interface ThreadDetail {
  thread_id: string;
  subject: string;
  student_id: string;
  student_name: string;
  guardian_name: string;
  relationship_type?: string;
  messages: Message[];
}

export interface Guardian {
  account_id: string;
  name: string;
  relationship_type?: string;
  is_primary: boolean;
}

interface ApiResponse<T> {
  status?: string;
  success?: boolean;
  data?: T;
  error?: string;
}

/**
 * German label for a guardian relationship type. No gender colon/Doppelpunkt;
 * unknown or empty values fall back to the neutral "Bezugsperson".
 */
export function relationshipLabel(rel?: string): string {
  switch (rel) {
    case "parent":
      return "Elternteil";
    case "guardian":
      return "Erziehungsberechtigt";
    case "relative":
      return "Verwandt";
    case "other":
      return "Bezugsperson";
    default:
      return "Bezugsperson";
  }
}

/**
 * Fetch the staff inbox of message threads. When `onlyUnread` is true only
 * threads with unread guardian messages are returned.
 */
export async function fetchInbox(onlyUnread = false): Promise<InboxThread[]> {
  const response = await fetch(
    `/api/messages${onlyUnread ? "?unread=true" : ""}`,
  );
  if (!response.ok) {
    throw new Error("Nachrichten konnten nicht geladen werden");
  }
  const result = (await response.json()) as ApiResponse<InboxThread[]>;
  return result.data ?? [];
}

/**
 * Fetch the number of unread message threads for the unread badge.
 */
export async function fetchUnreadCount(): Promise<number> {
  const response = await fetch("/api/messages/unread-count");
  if (!response.ok) {
    throw new Error("Ungelesene Nachrichten konnten nicht geladen werden");
  }
  const result = (await response.json()) as ApiResponse<{
    unread_count: number;
  }>;
  return result.data?.unread_count ?? 0;
}

/**
 * Fetch the full thread (subject, child/guardian metadata, messages
 * oldest-first) for the chat window.
 */
export async function fetchThread(threadId: string): Promise<ThreadDetail> {
  const response = await fetch(`/api/messages/threads/${threadId}`);
  if (!response.ok) {
    throw new Error("Nachrichtenverlauf konnte nicht geladen werden");
  }
  const result = (await response.json()) as ApiResponse<ThreadDetail>;
  if (!result.data) {
    throw new Error("Nachrichtenverlauf konnte nicht geladen werden");
  }
  return result.data;
}

/**
 * Send a staff reply to a thread. Returns the full updated message list.
 */
export async function postMessage(
  threadId: string,
  body: string,
): Promise<Message[]> {
  const response = await fetch(`/api/messages/threads/${threadId}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ body }),
  });
  if (!response.ok) {
    throw new Error("Nachricht konnte nicht gesendet werden");
  }
  const result = (await response.json()) as ApiResponse<Message[]>;
  return result.data ?? [];
}

/**
 * Start a new thread between the OGS and one guardian about one child.
 * Returns the created ThreadDetail (including its first message).
 */
export async function startThread(input: {
  studentId: string;
  guardianAccountId: string;
  subject: string;
  body: string;
}): Promise<ThreadDetail> {
  const response = await fetch("/api/messages/threads", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      student_id: input.studentId,
      guardian_account_id: input.guardianAccountId,
      subject: input.subject,
      body: input.body,
    }),
  });
  if (!response.ok) {
    throw new Error("Nachricht konnte nicht gesendet werden");
  }
  const result = (await response.json()) as ApiResponse<ThreadDetail>;
  if (!result.data) {
    throw new Error("Nachricht konnte nicht gesendet werden");
  }
  return result.data;
}

/**
 * Fetch the guardians of a child who have a parents-portal account and can
 * receive a new message thread.
 */
export async function fetchGuardians(studentId: string): Promise<Guardian[]> {
  const response = await fetch(`/api/messages/students/${studentId}/guardians`);
  if (!response.ok) {
    throw new Error("Bezugspersonen konnten nicht geladen werden");
  }
  const result = (await response.json()) as ApiResponse<Guardian[]>;
  return result.data ?? [];
}
