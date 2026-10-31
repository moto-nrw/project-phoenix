import type { ErrorCode } from "~/lib/error-codes.generated";

/**
 * Codes with which the backend refuses the resource behind a sent message
 * (#5011): the conversation is gone or no longer the sender's. The optimistic
 * helper drops every representation of it on a 401, 403 or 404 with one of
 * these codes (status and code, ADR 0006).
 *
 * Not listed on purpose, because they are states and keep the conversation:
 * the school switched messaging off (`communication.staff_messaging_disabled`,
 * `communication.parent_messaging_disabled`,
 * `care.notes_disabled`), the leadership is not reachable
 * (`communication.leadership_messages_unavailable`), the child left the OGS
 * (`care.child_care_ended`, reading stays open), session-level codes
 * (`identity.session_*`, handled by the session guards) and failures without
 * a response.
 *
 * `general.permission` is the class code of a 401/403 without a code of its
 * own (`ErrorClassCode`, `AuthorizationForbidden`); `general.input` with
 * status 404 is the class code of a not-found answer.
 */

/**
 * Staff reply to parents, POST /api/messages/threads/{id}
 * (`modules/communication/http/parentmessages/api.go`, `renderMessagingError`):
 * 403 without code for `users:read` missing or `ErrParentMessagingForbidden`,
 * 404 for a thread that is gone. A school that switched parent messaging off
 * answers with `communication.parent_messaging_disabled`, a state (#5416).
 */
export const PARENT_THREAD_SEND_REFUSALS: readonly ErrorCode[] = [
  "general.permission",
  "general.input",
];

/**
 * Further message in a Gruppennachricht, POST
 * /api/messages/group-threads/{id}/messages (`renderGroupThreadError`, then
 * `renderMessagingError`): 404 for a thread that is gone, 403 as above.
 */
export const GROUP_THREAD_SEND_REFUSALS: readonly ErrorCode[] = [
  "general.permission",
  "general.input",
];

/**
 * Team chat in the OGS and the school portal, POST
 * /api/staff-messages/threads/{id} (`modules/communication/http/staffmessages/errors.go`):
 * a thread that is gone and one the sender no longer belongs to both answer
 * 404 on purpose; 403 without code from the session's permission checks.
 */
export const TEAM_CHAT_SEND_REFUSALS: readonly ErrorCode[] = [
  "general.input",
  "general.permission",
];

/**
 * Parent to the OGS, POST /api/parent/me/messages/children/{id}
 * (`backend/workflows/parentportal/http/child_write_errors.go`): the
 * relationship to the child is gone (`care.child_not_linked`) or no longer
 * grants writing (`care.guardian_permission_denied`, the notes.write
 * permission of guardian-parent-permissions.md); 403 without code from the
 * portal's access checks.
 */
export const PARENT_PORTAL_SEND_REFUSALS: readonly ErrorCode[] = [
  "care.child_not_linked",
  "care.guardian_permission_denied",
  "general.permission",
];
