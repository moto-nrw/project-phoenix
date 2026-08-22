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
 * Reads the backend error message off a failed /api/messages response and throws
 * it, falling back to the supplied German default only when the body carries no
 * `error`. Without this every helper discarded the envelope's `error`, so a 403
 * "messaging disabled" surfaced as a generic "... konnte nicht geladen werden"
 * and the real reason was lost. Mirrors parent-api.ts's shared error path.
 */
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

/** GET a /api/messages route, surfacing the backend error on failure. */
async function getEnvelope<T>(
  url: string,
  fallback: string,
): Promise<ApiResponse<T>> {
  const response = await fetch(url);
  if (!response.ok) await throwApiError(response, fallback);
  return (await response.json()) as ApiResponse<T>;
}

/** POST to a /api/messages route, surfacing the backend error on failure. */
async function postEnvelope<T>(
  url: string,
  body: unknown,
  fallback: string,
): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) await throwApiError(response, fallback);
  return (await response.json()) as ApiResponse<T>;
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
  const result = await getEnvelope<InboxThread[]>(
    `/api/messages${params.size > 0 ? `?${params.toString()}` : ""}`,
    "Nachrichten konnten nicht geladen werden",
  );
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
  const result = await getEnvelope<InboxThread[]>(
    `/api/messages/students/${encodeURIComponent(studentId)}/threads`,
    "Nachrichten konnten nicht geladen werden",
  );
  return result.data ?? [];
}

/**
 * Fetch the number of unread message threads for the unread badge.
 */
export async function fetchUnreadCount(): Promise<number> {
  const result = await getEnvelope<{ unread_count: number }>(
    "/api/messages/unread-count",
    "Ungelesene Nachrichten konnten nicht geladen werden",
  );
  return result.data?.unread_count ?? 0;
}

/**
 * Fetch the full conversation (child/guardian metadata, messages oldest-first)
 * for the chat window.
 */
export async function fetchThread(threadId: string): Promise<ThreadDetail> {
  const fallback = "Nachrichtenverlauf konnte nicht geladen werden";
  const result = await getEnvelope<ThreadDetail>(
    `/api/messages/threads/${threadId}`,
    fallback,
  );
  if (!result.data) {
    throw new Error(fallback);
  }
  return result.data;
}

/**
 * Send a staff reply to a thread. Returns the full updated message list.
 */
export async function postMessage(
  threadId: string,
  body: string,
  handledUpToMessageId?: string,
): Promise<Message[]> {
  const result = await postEnvelope<Message[]>(
    `/api/messages/threads/${threadId}`,
    {
      body,
      ...(handledUpToMessageId
        ? { handled_up_to_message_id: handledUpToMessageId }
        : {}),
    },
    "Nachricht konnte nicht gesendet werden",
  );
  return result.data ?? [];
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
  const fallback = "Unterhaltung konnte nicht geöffnet werden";
  const result = await postEnvelope<ThreadDetail>(
    "/api/messages/threads/open",
    {
      student_id: input.studentId,
      guardian_account_id: input.guardianAccountId,
    },
    fallback,
  );
  if (!result.data) {
    throw new Error(fallback);
  }
  return result.data;
}

/**
 * Fetch the guardians of a child who have a parents-portal account and can
 * receive a new message thread.
 */
export async function fetchGuardians(studentId: string): Promise<Guardian[]> {
  const result = await getEnvelope<Guardian[]>(
    `/api/messages/students/${studentId}/guardians`,
    "Bezugspersonen konnten nicht geladen werden",
  );
  return result.data ?? [];
}
