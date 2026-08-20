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
type MessageSenderKind = "guardian" | "staff" | "system";

/** A timeline entry kind: a plain message, a system event, or a request. */
export type MessageKind = "message" | "event" | "request";

/**
 * The structured change-request types a guardian can submit. `care_schedule` is
 * the permanent weekly plan, `pickup_change` a single day's pickup time — two
 * different promises to the parent, so they never share a label.
 */
type StructuredRequestType = "care_schedule" | "pickup_change";

/** Every request_type token emitted on the shared message/event wire. */
type WireRequestType =
  | StructuredRequestType
  | "master_data"
  | "excused_absence"
  | "sick_absence"
  | "care_offering";

/** The lifecycle status of a parent-OGS change request. */
type RequestStatus = "offen" | "erledigt" | "abgelehnt" | "zurueckgezogen";

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
  // `label` (authoritative only for the German-only staff portal). `weekday`
  // (1-5) + `care_kind` identify a care-schedule row. Absent on the staff side /
  // legacy payloads → fall back to `label`.
  readonly weekday?: number;
  readonly care_kind?: "arrival" | "pickup" | "departure_mode";
  // Raw departure-mode keys behind `old`/`new` for `departure_mode` rows, so the
  // localized parents portal renders mode names in the guardian's language
  // instead of the German `old`/`new` strings. `old_modes` is the day's allowed
  // set (an "alone" entry stands in for an empty set); `new_mode` is the single
  // requested mode. Absent for arrival/pickup rows (times are language-neutral)
  // and for legacy payloads → fall back to `old`/`new`.
  readonly old_modes?: string[];
  readonly new_mode?: string;
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
  readonly request_type?: WireRequestType;
  readonly request_status?: RequestStatus;
  // Deep-link reference to the underlying request row (e.g.
  // "users.student_data_change_requests" / "schedule.care_schedule_change_requests"
  // + row id, both stringified). A request_created pill and the request_status
  // pill that resolves it carry the SAME ref, so open/closed is tracked per row
  // — essential for multi-row master-data submissions where several rows of the
  // same request_type are pending at once. Optional (older/non-request events).
  readonly ref_table?: string;
  readonly ref_id?: string;
  readonly payload?: Record<string, unknown>;
  readonly decision_reason?: string;
  // read_by_staff: a guardian message the OGS has read ("Gelesen", shown to the
  // parent). read_by_guardian: a staff message the guardian has read ("Gelesen",
  // shown to staff). Each side renders the receipt for its OWN messages the other
  // side has read.
  readonly read_by_staff?: boolean;
  readonly read_by_guardian?: boolean;
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

const PARENT_REQUEST_TYPE_I18N_KEYS: Record<StructuredRequestType, string> = {
  care_schedule: "requestTypeCareSchedule",
  pickup_change: "requestTypePickupChange",
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
    PARENT_REQUEST_TYPE_I18N_KEYS[requestType as StructuredRequestType] ??
    "requestTitleFallback"
  );
}

const PARENT_REQUEST_CREATED_I18N_KEYS: Readonly<Record<string, string>> = {
  care_schedule: "eventRequestCreatedCareSchedule",
  pickup_change: "eventRequestCreatedPickupChange",
  master_data: "eventRequestCreatedMasterData",
  excused_absence: "eventRequestCreatedExcusedAbsence",
  sick_absence: "eventRequestCreatedSickAbsence",
  care_offering: "eventRequestCreatedCareOffering",
};

const PARENT_REQUEST_CONFIRMED_I18N_KEYS: Readonly<Record<string, string>> = {
  care_schedule: "eventRequestConfirmedCareSchedule",
  pickup_change: "eventRequestConfirmedPickupChange",
  master_data: "eventRequestConfirmedMasterData",
  excused_absence: "eventRequestConfirmedExcusedAbsence",
  sick_absence: "eventRequestConfirmedSickAbsence",
  care_offering: "eventRequestConfirmedCareOffering",
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
  if (message.kind !== "event") {
    return null;
  }
  // A new request was just submitted from the Stammdaten page (#1803). The pill
  // announces it in the chat; the text differs per request_type so a guardian
  // reads which domain the request touches.
  if (message.event_type === "request_created") {
    const key = message.request_type
      ? PARENT_REQUEST_CREATED_I18N_KEYS[message.request_type]
      : undefined;
    return key ? { key } : null;
  }
  // sick_note / care_exception / care_exception_correction pills carry a
  // German-only body with dates and times embedded, so there is nothing
  // structured to localize — return null and let the caller render the raw
  // body verbatim. (Same fallback used for any unrecognized event below.)
  if (message.event_type !== "request_status" || !message.request_status) {
    return null;
  }
  switch (message.request_status) {
    case "erledigt": {
      const key = message.request_type
        ? PARENT_REQUEST_CONFIRMED_I18N_KEYS[message.request_type]
        : undefined;
      return { key: key ?? "eventRequestConfirmed" };
    }
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

/**
 * Localized preview (parentOgsMessaging namespace key) for a thread's last
 * message on the parents portal, or null when the last message is a plain
 * guardian/staff message whose body is already language-neutral — the caller
 * then shows `last_message_body` verbatim.
 *
 * System-generated bodies are German on the wire: a structured request keeps its
 * German title ("Anfrage: …"), and a decision/withdrawal system event keeps its
 * German body ("Anfrage bestätigt", "Anfrage abgelehnt: …", "Anfrage
 * zurückgezogen"). The thread list / child-detail preview only receive
 * `last_message_body`, so without this they show that German text to a
 * non-German guardian until the next plain message arrives. Mirrors how the full
 * conversation localizes each card from structured fields.
 *
 * The reject reason is deliberately NOT passed through: it is staff free text
 * (not localizable), so a preview shows the localized status only ("Anfrage
 * abgelehnt") rather than appending a German sentence; the full conversation
 * still renders the reason.
 */
export function parentThreadPreviewI18nDescriptor(thread: {
  readonly last_message_kind?: string;
  readonly last_event_type?: string;
  readonly last_request_type?: string;
  readonly last_request_status?: string;
}): { key: string; values?: { reason: string } } | null {
  if (thread.last_message_kind === "request") {
    return { key: parentRequestTypeI18nKey(thread.last_request_type) };
  }
  if (thread.last_message_kind === "event") {
    return parentEventI18nDescriptor({
      kind: "event",
      event_type: thread.last_event_type,
      request_status: thread.last_request_status,
      request_type: thread.last_request_type,
    });
  }
  return null;
}
