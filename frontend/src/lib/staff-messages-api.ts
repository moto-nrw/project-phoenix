import type { ErrorCode } from "~/lib/error-codes.generated";
import {
  ApiError,
  apiErrorFromBody,
  transportFetch,
  wireErrorCode,
} from "./api-error";
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
  client_message_id?: string;
  id: string;
  sender_account_id: string;
  sender_name: string;
  body: string;
  created_at: string;
  /**
   * Group chats only (#4776): who of the members read the message, and how
   * many members could have (everyone but the sender who had joined by then).
   */
  read_by_account_ids?: string[];
  /** Group chats: members who were eligible to read this message. */
  eligible_reader_account_ids?: string[];
  reader_count?: number;
}

/** A direct chat between two people, or a named group chat (#4776). */
export type StaffThreadKind = "direct" | "group";

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
  /** "group": `group_name` and `member_count` instead of a counterpart. */
  kind?: StaffThreadKind;
  group_name?: string;
  member_count?: number;
  /** Group chats: who wrote the last message. */
  last_sender_name?: string;
}

export interface StaffThreadDetail {
  thread_id: string;
  counterpart_account_id: string;
  counterpart_name: string;
  counterpart_role_kind: StaffRoleKind;
  messages: StaffMessage[];
  kind?: StaffThreadKind;
  group_name?: string;
  /** The viewer started the group or leads the school (#4776). */
  can_manage?: boolean;
  members?: MessageableStaff[];
}

/** Whether a thread is a group chat (#4776). */
export function isGroupThread(thread: { kind?: StaffThreadKind }): boolean {
  return thread.kind === "group";
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
  code?: string;
}

/**
 * Wire contract with modules/communication/http/staffmessages: the school has the Team-Chat switched
 * off. Carried as a stable code rather than matched on the German sentence, so
 * polishing the wording cannot turn the read-only state back into a red
 * "loading failed" error with a dead-end compose button.
 */
const STAFF_MESSAGING_DISABLED: ErrorCode =
  "communication.staff_messaging_disabled";

/**
 * Wire contract with modules/communication/http/staffmessages: the other side of this conversation is
 * no longer a reachable colleague. Same reasoning as STAFF_MESSAGING_DISABLED -
 * a code, not a sentence, so the read-only branch survives a rewording.
 */
const COUNTERPART_UNAVAILABLE: ErrorCode =
  "communication.staff_counterpart_unavailable";

/** Whether an unknown thrown value is the "counterpart has left" case. */
export function isCounterpartUnavailable(err: unknown): boolean {
  return (
    err instanceof ApiError &&
    wireErrorCode(err.code) === COUNTERPART_UNAVAILABLE
  );
}

/** Whether an unknown thrown value is the "school switched it off" case. */
export function isStaffMessagingDisabled(err: unknown): boolean {
  return (
    err instanceof ApiError &&
    wireErrorCode(err.code) === STAFF_MESSAGING_DISABLED
  );
}

/**
 * Throws a failed response as an ApiError carrying the envelope's
 * code, status, field errors and request ID (#2517). The message is a
 * developer diagnostic; the shared display path shows the catalog text.
 */
async function unwrap<T>(
  response: Response,
  fallbackMessage: string,
): Promise<ApiResponse<T>> {
  const body = (await response.json().catch(() => ({}))) as ApiResponse<T>;
  if (!response.ok) {
    throw apiErrorFromBody(fallbackMessage, response.status, body);
  }
  return body;
}

/** A success response without the expected payload. */
function missingData(fallback: string): ApiError {
  return new ApiError(fallback, 500);
}

async function getEnvelope<T>(
  url: string,
  fallbackMessage: string,
): Promise<ApiResponse<T>> {
  return unwrap<T>(await transportFetch(url), fallbackMessage);
}

async function postEnvelope<T>(
  url: string,
  payload: unknown,
  fallbackMessage: string,
  method: "POST" | "PUT" | "DELETE" = "POST",
): Promise<ApiResponse<T>> {
  const response = await transportFetch(url, {
    method,
    headers: { "Content-Type": "application/json" },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
  return unwrap<T>(response, fallbackMessage);
}

/** The chat window from a response that must carry one. */
function requireThread(
  result: ApiResponse<StaffThreadDetail>,
  fallback: string,
): StaffThreadDetail {
  if (!result.data) {
    throw missingData(fallback);
  }
  return result.data;
}

/** The calls a Team-Chat surface needs, bound to one proxy base path. */
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
  postMessage(
    threadId: string,
    body: string,
    clientMessageId?: string,
  ): Promise<StaffMessage>;
  /** Start a group chat (#4776). OGS portal only. */
  createGroup(name: string, accountIds: string[]): Promise<StaffThreadDetail>;
  /** Rename a group chat. OGS portal only. */
  renameGroup(threadId: string, name: string): Promise<StaffThreadDetail>;
  /** Add members to a group chat. OGS portal only. */
  addGroupMembers(
    threadId: string,
    accountIds: string[],
  ): Promise<StaffThreadDetail>;
  /** Remove one member from a group chat. OGS portal only. */
  removeGroupMember(
    threadId: string,
    accountId: string,
  ): Promise<StaffThreadDetail>;
  /** Leave a group chat. Both portals. */
  leaveGroup(threadId: string): Promise<void>;
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
        throw missingData(fallback);
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
        throw missingData(fallback);
      }
      return result.data;
    },

    async postMessage(threadId, body, clientMessageId) {
      const fallback = "Die Nachricht konnte nicht gesendet werden";
      const result = await postEnvelope<StaffMessage>(
        `${basePath}/threads/${encodeURIComponent(threadId)}`,
        { body, client_message_id: clientMessageId },
        fallback,
      );
      if (!result.data) {
        throw missingData(fallback);
      }
      return result.data;
    },

    async createGroup(name, accountIds) {
      const fallback = "Die Gruppe konnte nicht angelegt werden";
      const result = await postEnvelope<StaffThreadDetail>(
        `${basePath}/groups`,
        { name, account_ids: accountIds },
        fallback,
      );
      return requireThread(result, fallback);
    },

    async renameGroup(threadId, name) {
      const fallback = "Der Name konnte nicht gespeichert werden";
      const result = await postEnvelope<StaffThreadDetail>(
        `${basePath}/threads/${encodeURIComponent(threadId)}/name`,
        { name },
        fallback,
        "PUT",
      );
      return requireThread(result, fallback);
    },

    async addGroupMembers(threadId, accountIds) {
      const fallback = "Die Personen konnten nicht hinzugefügt werden";
      const result = await postEnvelope<StaffThreadDetail>(
        `${basePath}/threads/${encodeURIComponent(threadId)}/members`,
        { account_ids: accountIds },
        fallback,
      );
      return requireThread(result, fallback);
    },

    async removeGroupMember(threadId, accountId) {
      const fallback = "Die Person konnte nicht entfernt werden";
      const result = await postEnvelope<StaffThreadDetail>(
        `${basePath}/threads/${encodeURIComponent(threadId)}/members/${encodeURIComponent(accountId)}`,
        undefined,
        fallback,
        "DELETE",
      );
      return requireThread(result, fallback);
    },

    async leaveGroup(threadId) {
      await postEnvelope<null>(
        `${basePath}/threads/${encodeURIComponent(threadId)}/leave`,
        {},
        "Die Gruppe konnte nicht verlassen werden",
      );
    },
  };
}

/** The OGS (tenant) portal instance. */
export const tenantStaffMessagesApi = createStaffMessagesApi(
  "/api/staff-messages",
);

/** Unread total for the sidebar badge (tenant portal). */
export const fetchStaffUnreadCount = tenantStaffMessagesApi.fetchUnreadCount;
