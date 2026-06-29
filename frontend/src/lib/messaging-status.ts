/**
 * Single source of truth for the parent-OGS messaging wire types, shared by the
 * staff (tenant) portal and the parents portal so the two cannot drift.
 *
 * Beyond plain messages, a conversation timeline also carries system events and
 * structured change-requests. The staff portal is German-only and uses
 * STAFF_STATUS_LABELS directly; the parents portal is localized and maps each
 * status to a next-intl key in the `parentOgsMessaging` namespace. Both maps are
 * exhaustive `Record<RequestStatus, …>`, so adding a status to the union fails to
 * compile until both are updated.
 */

/** Who sent a message: the guardian, the OGS staff, or a system event. */
export type MessageSenderKind = "guardian" | "staff" | "system";

/** A timeline entry kind: a plain message, a system event, or a request. */
export type MessageKind = "message" | "event" | "request";

/** The structured change-request types a guardian can submit. */
export type RequestType = "care_schedule" | "student_master_data";

/** The lifecycle status of a parent-OGS change request. */
export type RequestStatus =
  | "offen"
  | "erledigt"
  | "abgelehnt"
  | "zurueckgezogen";

/**
 * One field a still-open request would change, rendered "current → requested".
 * Single source of truth for both portals and the shared RequestDiffPanel.
 */
export interface RequestDiffEntry {
  readonly label: string;
  readonly old: string;
  readonly new: string;
}

/**
 * One message/event/request in a parent-OGS conversation, as it arrives over the
 * wire (int64 ids already stringified by the backend). Shared verbatim by the
 * staff client (parent-messages-api `Message`) and the parent client (parent-api
 * `ParentMessage`). For staff messages on the parent side `sender_name` is the
 * "OGS [Schulname]" label.
 */
export interface ChatMessage {
  readonly id: string;
  readonly sender_kind: MessageSenderKind;
  readonly sender_name: string;
  readonly body: string;
  readonly created_at: string; // ISO timestamp
  // A plain message ("message"), a system event ("event"), or a structured
  // change-request ("request"). Defaults to a plain message when absent so an
  // older payload still renders as a bubble.
  readonly kind?: MessageKind;
  readonly event_type?: string;
  readonly request_type?: RequestType;
  readonly request_status?: RequestStatus;
  readonly payload?: Record<string, unknown>;
  readonly decision_reason?: string;
  // read_by_staff: a guardian message the OGS has read ("Gelesen", shown to the
  // parent). read_by_guardian: a staff message the guardian has read ("Gelesen",
  // shown to staff). Each side renders the receipt for its OWN messages the other
  // side has read.
  readonly read_by_staff?: boolean;
  readonly read_by_guardian?: boolean;
  // Server-computed "current → requested" comparison for a still-open request.
  // Absent once the request is decided (confirmed/rejected/withdrawn).
  readonly diff?: RequestDiffEntry[];
}

const STAFF_STATUS_LABELS: Record<RequestStatus, string> = {
  offen: "Offen",
  erledigt: "Erledigt",
  abgelehnt: "Abgelehnt",
  zurueckgezogen: "Zurückgezogen",
};

const PARENT_STATUS_I18N_KEYS: Record<RequestStatus, string> = {
  offen: "statusOpen",
  erledigt: "statusDone",
  abgelehnt: "statusRejected",
  zurueckgezogen: "statusWithdrawn",
};

const PARENT_REQUEST_TYPE_I18N_KEYS: Record<RequestType, string> = {
  care_schedule: "requestTypeCareSchedule",
  student_master_data: "requestTypeMasterData",
};

/** German label for the German-only staff portal. Unknown → "Offen". */
export function staffRequestStatusLabel(status?: string): string {
  return (
    STAFF_STATUS_LABELS[status as RequestStatus] ?? STAFF_STATUS_LABELS.offen
  );
}

/**
 * next-intl key (parentOgsMessaging namespace) for the localized parents portal.
 * Unknown → the "open" key, matching the previous default branch.
 */
export function parentRequestStatusI18nKey(status?: string): string {
  return (
    PARENT_STATUS_I18N_KEYS[status as RequestStatus] ??
    PARENT_STATUS_I18N_KEYS.offen
  );
}

/**
 * next-intl key (parentOgsMessaging namespace) for a request card's title,
 * derived from request_type so the localized parents portal never displays the
 * backend's German message body verbatim. Unknown → a generic "Request" key.
 */
export function parentRequestTypeI18nKey(requestType?: string): string {
  return (
    PARENT_REQUEST_TYPE_I18N_KEYS[requestType as RequestType] ??
    "requestTitleFallback"
  );
}
