import { LOCATION_COLORS } from "~/lib/location-helper";

/**
 * Email delivery status for guardian invitations (#1937).
 *
 * The backend keeps two separate, orthogonal axes and so does this module:
 *
 * - `dispatchStatus` — did WE hand the message to the mail server?
 * - `deliveryStatus` — what did the RECEIVING side do with it?
 *
 * Collapsing them is exactly the bug this feature fixes: `sent` used to be
 * rendered as "gesendet", which schools read as "angekommen".
 */

export type InvitationDispatchStatus =
  "pending" | "sending" | "sent" | "failed";

export type InvitationDeliveryStatus =
  | "unknown"
  | "queued"
  | "deferred"
  | "delivered"
  | "soft_bounced"
  | "suppressed"
  | "complained"
  | "bounced";

export interface InvitationDeliveryEvent {
  readonly type: string;
  readonly occurredAt: string;
  readonly reasonCode?: string;
  readonly reason?: string;
}

export interface InvitationDeliveryAttempt {
  readonly outboxId: string;
  readonly dispatchStatus: InvitationDispatchStatus;
  readonly deliveryStatus: InvitationDeliveryStatus;
  readonly queuedAt: string;
  readonly sentAt?: string;
  readonly deliveryStatusAt?: string;
  readonly attempts: number;
  readonly nextRetryAt?: string;
  readonly lastError?: string;
  readonly deliveryDetail?: string;
  readonly events: readonly InvitationDeliveryEvent[];
}

export interface InvitationDelivery {
  readonly invitationId: string;
  /** Newest attempt first. A resend adds another entry. */
  readonly attempts: readonly InvitationDeliveryAttempt[];
}

interface BackendDeliveryEvent {
  type: string;
  occurred_at: string;
  reason_code?: string;
  reason?: string;
}

interface BackendDeliveryAttempt {
  outbox_id: number;
  dispatch_status: InvitationDispatchStatus;
  delivery_status: InvitationDeliveryStatus;
  queued_at: string;
  sent_at?: string;
  delivery_status_at?: string;
  attempts: number;
  next_retry_at?: string;
  last_error?: string;
  delivery_detail?: string;
  events?: BackendDeliveryEvent[];
}

export interface BackendInvitationDelivery {
  invitation_id: number;
  attempts?: BackendDeliveryAttempt[];
}

export function mapInvitationDelivery(
  data: BackendInvitationDelivery,
): InvitationDelivery {
  return {
    invitationId: data.invitation_id.toString(),
    attempts: (data.attempts ?? []).map((attempt) => ({
      outboxId: attempt.outbox_id.toString(),
      dispatchStatus: attempt.dispatch_status,
      deliveryStatus: attempt.delivery_status,
      queuedAt: attempt.queued_at,
      sentAt: attempt.sent_at,
      deliveryStatusAt: attempt.delivery_status_at,
      attempts: attempt.attempts,
      nextRetryAt: attempt.next_retry_at,
      lastError: attempt.last_error,
      deliveryDetail: attempt.delivery_detail,
      events: (attempt.events ?? []).map((event) => ({
        type: event.type,
        occurredAt: event.occurred_at,
        reasonCode: event.reason_code,
        reason: event.reason,
      })),
    })),
  };
}

/** Visual + textual presentation of one delivery state. */
export interface DeliveryPresentation {
  readonly label: string;
  readonly color: string;
  /** Full-sentence explanation for the detail dialog. */
  readonly hint: string;
  /**
   * True while the mail server is still working on it. The resend button is
   * disabled in this state — sending a second invitation would not speed
   * anything up and confuses the family with two links.
   */
  readonly retryInProgress: boolean;
  /** True when only a corrected address can fix it. */
  readonly needsAddressFix: boolean;
}

const DELIVERED: DeliveryPresentation = {
  label: "Zugestellt",
  color: LOCATION_COLORS.GROUP_ROOM,
  hint: "Die Einladung wurde im Postfach der Empfängerin oder des Empfängers abgelegt.",
  retryInProgress: false,
  needsAddressFix: false,
};

const HANDED_OVER: DeliveryPresentation = {
  label: "E-Mail übergeben",
  color: LOCATION_COLORS.OTHER_ROOM,
  hint: "Der Mailserver hat die Nachricht angenommen. Eine Bestätigung, dass sie im Postfach angekommen ist, liegt noch nicht vor.",
  retryInProgress: false,
  needsAddressFix: false,
};

const QUEUED: DeliveryPresentation = {
  label: "Einladung eingeplant",
  color: LOCATION_COLORS.UNKNOWN,
  hint: "Die E-Mail ist eingeplant und wird in Kürze versendet.",
  retryInProgress: true,
  needsAddressFix: false,
};

const SENDING: DeliveryPresentation = {
  label: "Wird gesendet",
  color: LOCATION_COLORS.UNKNOWN,
  hint: "Die E-Mail wird gerade an den Mailserver übergeben.",
  retryInProgress: true,
  needsAddressFix: false,
};

const DEFERRED: DeliveryPresentation = {
  label: "Zustellung verzögert",
  color: LOCATION_COLORS.SCHOOLYARD,
  hint: "Die Zustellung ist verzögert. Der Mailserver versucht es weiter. Sie müssen die Einladung nicht erneut senden.",
  retryInProgress: true,
  needsAddressFix: false,
};

const SOFT_BOUNCED: DeliveryPresentation = {
  label: "Vorübergehend nicht zustellbar",
  color: LOCATION_COLORS.SCHOOLYARD,
  hint: "Der Mailserver hat die Zustellung nach mehreren Versuchen aufgegeben. Bitte prüfen Sie die Adresse und senden Sie danach erneut ein.",
  retryInProgress: false,
  needsAddressFix: true,
};

const BOUNCED: DeliveryPresentation = {
  label: "Nicht zustellbar",
  color: LOCATION_COLORS.HOME,
  hint: "Die Adresse hat die Nachricht endgültig abgelehnt. Bitte korrigieren Sie die E-Mail-Adresse und senden Sie die Einladung danach erneut.",
  retryInProgress: false,
  needsAddressFix: true,
};

const SUPPRESSED: DeliveryPresentation = {
  label: "Blockiert",
  color: LOCATION_COLORS.HOME,
  hint: "Der Mailanbieter blockiert Nachrichten an diese Adresse. Bitte klären Sie die Adresse mit der Familie ab.",
  retryInProgress: false,
  needsAddressFix: true,
};

const COMPLAINED: DeliveryPresentation = {
  label: "Als Spam markiert",
  color: LOCATION_COLORS.HOME,
  hint: "Die Nachricht wurde als Spam gemeldet. Weitere E-Mails an diese Adresse erreichen die Familie voraussichtlich nicht.",
  retryInProgress: false,
  needsAddressFix: true,
};

const DISPATCH_FAILED: DeliveryPresentation = {
  label: "Versand fehlgeschlagen",
  color: LOCATION_COLORS.HOME,
  hint: "Die E-Mail konnte nicht an den Mailserver übergeben werden. Bitte versuchen Sie es erneut oder wenden Sie sich an den Support.",
  retryInProgress: false,
  needsAddressFix: false,
};

const DELIVERY_PRESENTATIONS: Record<
  InvitationDeliveryStatus,
  DeliveryPresentation
> = {
  unknown: HANDED_OVER,
  queued: HANDED_OVER,
  deferred: DEFERRED,
  delivered: DELIVERED,
  soft_bounced: SOFT_BOUNCED,
  suppressed: SUPPRESSED,
  complained: COMPLAINED,
  bounced: BOUNCED,
};

/**
 * Resolve the single state to show for one dispatch attempt.
 *
 * Dispatch state wins while the message has not left us: there is nothing the
 * receiving side could have reported yet. Once dispatched, the provider's
 * verdict takes over.
 */
export function presentDeliveryAttempt(
  attempt: InvitationDeliveryAttempt,
): DeliveryPresentation {
  switch (attempt.dispatchStatus) {
    case "pending":
      return QUEUED;
    case "sending":
      return SENDING;
    case "failed":
      return DISPATCH_FAILED;
    case "sent":
      return DELIVERY_PRESENTATIONS[attempt.deliveryStatus] ?? HANDED_OVER;
  }
}

/**
 * The state to show for a guardian: the newest attempt. Older attempts stay
 * visible in the detail dialog, because a school debugging "why is this
 * family not registered" needs the whole history.
 */
export function presentLatestAttempt(
  delivery: InvitationDelivery | undefined,
): DeliveryPresentation | undefined {
  const latest = delivery?.attempts[0];
  return latest ? presentDeliveryAttempt(latest) : undefined;
}
