/**
 * Staff-side client for the parent-OGS messaging feature. Chat model: one
 * continuous conversation between the OGS and ONE guardian about ONE child (no
 * subject). Sending is get-or-create — there is at most one conversation per
 * (child, guardian).
 *
 * Talks to the Next.js proxy routes under /api/messages, which forward to the
 * Go backend with the staff JWT. Backend int64 ids arrive already stringified.
 */

import type { ChatMessage } from "~/lib/messaging-status";
import { getRelationshipTypeLabel } from "~/lib/guardian-helpers";

// The wire message shape is shared with the parent client; see ChatMessage.
export type Message = ChatMessage;

export interface InboxThread {
  thread_id: string;
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

export interface ThreadDetail {
  thread_id: string;
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
 * German label for a guardian relationship type. Delegates to the shared
 * getRelationshipTypeLabel (lib/guardian-helpers) — the SAME label set the
 * guardian admin UI uses — so a guardian never reads differently in the
 * messages inbox than in the guardian manager. Empty/unset falls back to the
 * neutral "Bezugsperson" (getRelationshipTypeLabel would echo the empty value).
 */
export function relationshipLabel(rel?: string): string {
  if (!rel) return "Bezugsperson";
  return getRelationshipTypeLabel(rel);
}

export async function fetchInboxWithFilters(filters: {
  onlyUnread?: boolean;
}): Promise<InboxThread[]> {
  const params = new URLSearchParams();
  if (filters.onlyUnread) params.set("unread", "true");
  const response = await fetch(
    `/api/messages${params.size > 0 ? `?${params.toString()}` : ""}`,
  );
  if (!response.ok) {
    throw new Error("Nachrichten konnten nicht geladen werden");
  }
  const result = (await response.json()) as ApiResponse<InboxThread[]>;
  return result.data ?? [];
}

/**
 * Fetch one child's threads (staff view) for the student-detail card, scoped
 * server-side so the card no longer pulls the whole tenant inbox and filters by
 * student_id client-side.
 */
export async function fetchStudentThreads(
  studentId: string,
): Promise<InboxThread[]> {
  const response = await fetch(
    `/api/messages/students/${encodeURIComponent(studentId)}/threads`,
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
 * Fetch the full conversation (child/guardian metadata, messages oldest-first)
 * for the chat window.
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
 * Send the OGS's first message to one guardian about one child. The backend
 * get-or-creates the conversation and returns the full ThreadDetail (including
 * the message).
 */
export async function startThread(input: {
  studentId: string;
  guardianAccountId: string;
  body: string;
}): Promise<ThreadDetail> {
  const response = await fetch("/api/messages/threads", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      student_id: input.studentId,
      guardian_account_id: input.guardianAccountId,
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
 * Get-or-create the conversation for a (child, guardian) pair and return it
 * (with history if any) WITHOUT sending a message — opens the chat window
 * directly from the recipient picker, WhatsApp-style. The empty thread stays
 * hidden from the inbox until the first message is sent.
 */
export async function openThread(input: {
  studentId: string;
  guardianAccountId: string;
}): Promise<ThreadDetail> {
  const response = await fetch("/api/messages/threads/open", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      student_id: input.studentId,
      guardian_account_id: input.guardianAccountId,
    }),
  });
  if (!response.ok) {
    throw new Error("Unterhaltung konnte nicht geöffnet werden");
  }
  const result = (await response.json()) as ApiResponse<ThreadDetail>;
  if (!result.data) {
    throw new Error("Unterhaltung konnte nicht geöffnet werden");
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
