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
 * The school portal (#2208) reaches the SAME conversations through its own
 * proxy routes (/api/school/staff-messages, school session): the functions are
 * built by createStaffMessagesApi(basePath), and the named exports below are
 * the tenant-portal instance.
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

/**
 * Which side of the school a colleague sits on (#2208). Coarse on purpose:
 * with Lehrkräfte in the same conversations as the OGS team, the name alone
 * no longer says who someone is.
 */
export type StaffRoleKind = "lehrkraft" | "admin" | "staff";

export interface StaffInboxThread {
  thread_id: string;
  counterpart_account_id: string;
  counterpart_name: string;
  counterpart_role_kind: StaffRoleKind;
  last_message_at?: string;
  last_message_body?: string;
  last_message_mine: boolean;
  unread_count: number;
}

export interface StaffThreadDetail {
  thread_id: string;
  counterpart_account_id: string;
  counterpart_name: string;
  counterpart_role_kind: StaffRoleKind;
  messages: StaffMessage[];
}

export interface MessageableStaff {
  account_id: string;
  name: string;
  role_kind: StaffRoleKind;
}

/**
 * The label a reader sees next to a colleague's name, or null when it would
 * only repeat what the reader already knows.
 *
 * In the OGS portal every colleague is OGS by default, so only "Lehrkraft"
 * carries information. In the school portal a Lehrkraft writes INTO the OGS
 * and needs to tell the leadership from the team - and sees fellow Lehrkräfte
 * marked as such.
 */
export function staffRoleKindLabel(
  kind: StaffRoleKind | undefined,
  portal: "tenant" | "school",
): string | null {
  switch (kind) {
    case "lehrkraft":
      return "Lehrkraft";
    case "admin":
      return portal === "school" ? "OGS-Leitung" : null;
    case "staff":
      return portal === "school" ? "OGS-Team" : null;
    default:
      return null;
  }
}

interface ApiResponse<T> {
  status?: string;
  success?: boolean;
  data?: T;
  error?: string;
  code?: string;
}

/**
 * Wire contract with modules/communication/http/staffmessages: the school has the Team-Chat switched
 * off. Carried as a stable code rather than matched on the German sentence, so
 * polishing the wording cannot turn the read-only state back into a red
 * "loading failed" error with a dead-end compose button.
 */
const STAFF_MESSAGING_DISABLED = "staff_messaging_disabled";

/** An error carrying the backend's stable code, when there was one. */
class StaffMessagesError extends Error {
  readonly code?: string;
  constructor(message: string, code?: string) {
    super(message);
    this.name = "StaffMessagesError";
    this.code = code;
  }
}

/**
 * Wire contract with modules/communication/http/staffmessages: the other side of this conversation is
 * no longer a reachable colleague. Same reasoning as STAFF_MESSAGING_DISABLED -
 * a code, not a sentence, so the read-only branch survives a rewording.
 */
const COUNTERPART_UNAVAILABLE = "staff_counterpart_unavailable";

/** Whether an unknown thrown value is the "counterpart has left" case. */
export function isCounterpartUnavailable(err: unknown): boolean {
  return (
    err instanceof StaffMessagesError && err.code === COUNTERPART_UNAVAILABLE
  );
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

/** The six calls a Team-Chat surface needs, bound to one proxy base path. */
export interface StaffMessagesApi {
  /** The caller's conversations, newest activity first. */
  fetchInbox(filters: { onlyUnread?: boolean }): Promise<StaffInboxThread[]>;
  /** Unread total for the navigation badge. */
  fetchUnreadCount(): Promise<number>;
  /** Colleagues the caller may write to. */
  fetchRecipients(): Promise<MessageableStaff[]>;
  /** The full conversation (messages oldest-first) for the chat window. */
  fetchThread(threadId: string): Promise<StaffThreadDetail>;
  /** Open (or create) the conversation with one colleague. */
  openThread(accountId: string): Promise<StaffThreadDetail>;
  /** Send one message into a conversation. */
  postMessage(threadId: string, body: string): Promise<StaffMessage>;
}

/**
 * Builds the client for one portal. `basePath` is the Next.js proxy prefix
 * ("/api/staff-messages" for the OGS portal, "/api/school/staff-messages" for
 * the school portal); both forward to the same backend surface.
 */
export function createStaffMessagesApi(basePath: string): StaffMessagesApi {
  return {
    async fetchInbox(filters) {
      const params = new URLSearchParams();
      if (filters.onlyUnread) params.set("only_unread", "true");
      const result = await getEnvelope<StaffInboxThread[]>(
        `${basePath}${params.size > 0 ? `?${params.toString()}` : ""}`,
        "Nachrichten konnten nicht geladen werden",
      );
      return result.data ?? [];
    },

    async fetchUnreadCount() {
      const result = await getEnvelope<{ unread_count: number }>(
        `${basePath}/unread-count`,
        "Ungelesene Nachrichten konnten nicht geladen werden",
      );
      return result.data?.unread_count ?? 0;
    },

    async fetchRecipients() {
      const result = await getEnvelope<MessageableStaff[]>(
        `${basePath}/recipients`,
        "Die Liste konnte nicht geladen werden",
      );
      return result.data ?? [];
    },

    async fetchThread(threadId) {
      const fallback = "Der Verlauf konnte nicht geladen werden";
      const result = await getEnvelope<StaffThreadDetail>(
        `${basePath}/threads/${encodeURIComponent(threadId)}`,
        fallback,
      );
      if (!result.data) {
        throw new Error(fallback);
      }
      return result.data;
    },

    async openThread(accountId) {
      const fallback = "Die Unterhaltung konnte nicht geöffnet werden";
      const result = await postEnvelope<StaffThreadDetail>(
        `${basePath}/threads/open`,
        { account_id: accountId },
        fallback,
      );
      if (!result.data) {
        throw new Error(fallback);
      }
      return result.data;
    },

    async postMessage(threadId, body) {
      const fallback = "Die Nachricht konnte nicht gesendet werden";
      const result = await postEnvelope<StaffMessage>(
        `${basePath}/threads/${encodeURIComponent(threadId)}`,
        { body },
        fallback,
      );
      if (!result.data) {
        throw new Error(fallback);
      }
      return result.data;
    },
  };
}

/** The OGS (tenant) portal instance. */
export const tenantStaffMessagesApi = createStaffMessagesApi(
  "/api/staff-messages",
);

/** Unread total for the sidebar badge (tenant portal). */
export const fetchStaffUnreadCount = tenantStaffMessagesApi.fetchUnreadCount;
