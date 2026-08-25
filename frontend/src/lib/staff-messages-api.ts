/**
 * Client for the OGS-internal colleague chat (#2598). Chat model: one
 * continuous conversation between two staff accounts of the same school (no
 * subject). Opening is get-or-create — there is at most one conversation per
 * pair.
 *
 * Talks to the Next.js proxy routes under /api/staff-messages, which forward to
 * the Go backend with the staff JWT. Backend int64 ids arrive already
 * stringified.
 *
 * Deliberately separate from parent-messages-api.ts: the two look alike on the
 * wire but are different surfaces with different audiences, and merging them
 * would make it possible to send an internal note into a parent conversation by
 * passing the wrong id.
 */

export interface StaffMessage {
  id: string;
  sender_account_id: string;
  sender_name: string;
  body: string;
  created_at: string;
}

export interface StaffInboxThread {
  thread_id: string;
  counterpart_account_id: string;
  counterpart_name: string;
  last_message_at?: string;
  last_message_body?: string;
  last_message_mine: boolean;
  unread_count: number;
}

export interface StaffThreadDetail {
  thread_id: string;
  counterpart_account_id: string;
  counterpart_name: string;
  messages: StaffMessage[];
}

export interface MessageableStaff {
  account_id: string;
  name: string;
}

interface ApiResponse<T> {
  status?: string;
  success?: boolean;
  data?: T;
  error?: string;
  code?: string;
}

/**
 * Wire contract with api/staffmessaging: the school has the Team-Chat switched
 * off. Carried as a stable code rather than matched on the German sentence, so
 * polishing the wording cannot turn the read-only state back into a red
 * "loading failed" error with a dead-end compose button.
 */
export const STAFF_MESSAGING_DISABLED = "staff_messaging_disabled";

/** An error carrying the backend's stable code, when there was one. */
export class StaffMessagesError extends Error {
  readonly code?: string;
  constructor(message: string, code?: string) {
    super(message);
    this.name = "StaffMessagesError";
    this.code = code;
  }
}

/** Whether an unknown thrown value is the "school switched it off" case. */
export function isStaffMessagingDisabled(err: unknown): boolean {
  return (
    err instanceof StaffMessagesError && err.code === STAFF_MESSAGING_DISABLED
  );
}

/**
 * Reads the backend error message off a failed response and throws it, falling
 * back to the supplied German default only when the body carries no `error`.
 * Without this a 403 "Team-Chat nicht aktiviert" would surface as a generic
 * "konnte nicht geladen werden" and the real reason would be lost.
 */
async function unwrap<T>(
  response: Response,
  fallbackMessage: string,
): Promise<ApiResponse<T>> {
  const body = (await response.json().catch(() => ({}))) as ApiResponse<T>;
  if (!response.ok) {
    throw new StaffMessagesError(body.error ?? fallbackMessage, body.code);
  }
  return body;
}

async function getEnvelope<T>(
  url: string,
  fallbackMessage: string,
): Promise<ApiResponse<T>> {
  return unwrap<T>(await fetch(url), fallbackMessage);
}

async function postEnvelope<T>(
  url: string,
  payload: unknown,
  fallbackMessage: string,
): Promise<ApiResponse<T>> {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  return unwrap<T>(response, fallbackMessage);
}

/** The caller's conversations, newest activity first. */
export async function fetchStaffInbox(filters: {
  onlyUnread?: boolean;
}): Promise<StaffInboxThread[]> {
  const params = new URLSearchParams();
  if (filters.onlyUnread) params.set("only_unread", "true");
  const result = await getEnvelope<StaffInboxThread[]>(
    `/api/staff-messages${params.size > 0 ? `?${params.toString()}` : ""}`,
    "Nachrichten konnten nicht geladen werden",
  );
  return result.data ?? [];
}

/** Unread total for the sidebar badge. */
export async function fetchStaffUnreadCount(): Promise<number> {
  const result = await getEnvelope<{ unread_count: number }>(
    "/api/staff-messages/unread-count",
    "Ungelesene Nachrichten konnten nicht geladen werden",
  );
  return result.data?.unread_count ?? 0;
}

/** Colleagues the caller may write to. */
export async function fetchMessageableStaff(): Promise<MessageableStaff[]> {
  const result = await getEnvelope<MessageableStaff[]>(
    "/api/staff-messages/recipients",
    "Die Liste konnte nicht geladen werden",
  );
  return result.data ?? [];
}

/** The full conversation (messages oldest-first) for the chat window. */
export async function fetchStaffThread(
  threadId: string,
): Promise<StaffThreadDetail> {
  const fallback = "Der Verlauf konnte nicht geladen werden";
  const result = await getEnvelope<StaffThreadDetail>(
    `/api/staff-messages/threads/${encodeURIComponent(threadId)}`,
    fallback,
  );
  if (!result.data) {
    throw new Error(fallback);
  }
  return result.data;
}

/** Open (or create) the conversation with one colleague. */
export async function openStaffThread(
  accountId: string,
): Promise<StaffThreadDetail> {
  const fallback = "Die Unterhaltung konnte nicht geöffnet werden";
  const result = await postEnvelope<StaffThreadDetail>(
    "/api/staff-messages/threads/open",
    { account_id: accountId },
    fallback,
  );
  if (!result.data) {
    throw new Error(fallback);
  }
  return result.data;
}

/** Send one message into a conversation. */
export async function postStaffMessage(
  threadId: string,
  body: string,
): Promise<StaffMessage> {
  const fallback = "Die Nachricht konnte nicht gesendet werden";
  const result = await postEnvelope<StaffMessage>(
    `/api/staff-messages/threads/${encodeURIComponent(threadId)}`,
    { body },
    fallback,
  );
  if (!result.data) {
    throw new Error(fallback);
  }
  return result.data;
}
