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
  // Structured discriminators the backend sends so the localized parents portal
  // can render the label in the guardian's language instead of the German
  // `label` (authoritative only for the German-only staff portal). `field_key`
  // is set for master-data rows; `weekday` (1-5) + `care_kind` for care-schedule
  // rows. Absent on the staff side / legacy payloads → fall back to `label`.
  readonly field_key?: string;
  readonly weekday?: number;
  readonly care_kind?: "arrival" | "pickup" | "departure_mode";
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

/**
 * next-intl keys (parentOgsMessaging namespace) for a diff entry's label on the
 * localized parents portal, keyed by the backend's structured discriminators so
 * the German `label` is never shown to a non-German guardian. Care-schedule rows
 * compose a weekday key (`diffWeekday{1..5}`) with one of these.
 */
export const PARENT_DIFF_FIELD_I18N_KEYS: Record<string, string> = {
  first_name: "diffFieldFirstName",
  last_name: "diffFieldLastName",
  birthday: "diffFieldBirthday",
  guardian_email: "diffFieldGuardianEmail",
  guardian_phone: "diffFieldGuardianPhone",
  extra_info: "diffFieldExtraInfo",
};

export const PARENT_DIFF_CARE_KIND_I18N_KEYS: Record<string, string> = {
  arrival: "diffCareArrival",
  pickup: "diffCarePickup",
  departure_mode: "diffCareDepartureMode",
};

/**
 * Descriptor for localizing a request-decision system event on the parents
 * portal. Backend system-event bodies are German ("Anfrage bestätigt",
 * "Anfrage abgelehnt: …", "Anfrage zurückgezogen"), so the parent side renders
 * from the event's structured fields (event_type + request_status + request_type
 * + decision_reason) instead. Returns null for any event the parent side can't
 * localize from fields — the caller then falls back to the raw German body
 * (matching the previous behavior for unknown events).
 */
export function parentEventI18nDescriptor(message: {
  readonly kind?: MessageKind;
  readonly event_type?: string;
  readonly request_status?: string;
  readonly request_type?: string;
  readonly decision_reason?: string;
}): { key: string; values?: { reason: string } } | null {
  if (
    message.kind !== "event" ||
    message.event_type !== "request_status" ||
    !message.request_status
  ) {
    return null;
  }
  switch (message.request_status) {
    case "erledigt":
      if (message.request_type === "care_schedule") {
        return { key: "eventRequestConfirmedCareSchedule" };
      }
      if (message.request_type === "student_master_data") {
        return { key: "eventRequestConfirmedMasterData" };
      }
      return { key: "eventRequestConfirmed" };
    case "abgelehnt":
      return message.decision_reason
        ? {
            key: "eventRequestRejectedReason",
            values: { reason: message.decision_reason },
          }
        : { key: "eventRequestRejected" };
    case "zurueckgezogen":
      return { key: "eventRequestWithdrawn" };
    default:
      return null;
  }
}
