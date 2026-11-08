// Der Stand der Mail eines Schreibens (#4670), gemeinsam für die Schreiben
// der Ferienbetreuung und der Monatsbeiträge (#4684): das kurze Etikett, die
// Zeile daneben und ob sich die Mail jetzt (erneut) senden lässt.

import { formatBerlinDate, formatTime } from "~/lib/date-helpers";
import {
  type MailOutcome,
  type MailProviderBlock,
  mailStatusCopy,
} from "~/lib/email-delivery-status";
import type { LetterMailStatus } from "~/lib/traeger-billing-api";

/** What both kinds of letter know about their last mail. */
export interface LetterMailState {
  readonly status: LetterMailStatus;
  readonly recipientEmail: string;
  readonly mailOutcome?: MailOutcome;
  readonly providerBlock?: MailProviderBlock;
  readonly requestedAt: string | null;
  readonly sentAt: string | null;
}

type BadgeTone = "blue" | "green" | "orange" | "red" | "gray" | "outline";

const STATUS_TONES: Record<LetterMailStatus, BadgeTone> = {
  not_sent: "gray",
  queued: "blue",
  sent: "gray",
  failed: "red",
  unknown: "gray",
};

const STATUS_LABELS: Record<LetterMailStatus, string> = {
  not_sent: "Nicht gesendet",
  queued: "Wird gesendet",
  sent: "Gesendet",
  failed: "Nicht angekommen",
  unknown: "Ergebnis unbekannt",
};

// The provider's word outranks the outbox: "Gesendet" alone only means a server
// took the mail, so it never turns green before a report says so (#5934).
const MAIL_OUTCOME_BADGES: Partial<
  Record<MailOutcome, { label: string; tone: BadgeTone }>
> = {
  accepted: { label: "Gesendet", tone: "gray" },
  delivered: { label: "Zugestellt", tone: "green" },
  delayed: { label: "Verzögert", tone: "orange" },
  overdue: { label: "Überfällig", tone: "orange" },
  undeliverable: { label: "Nicht zustellbar", tone: "red" },
  complained: { label: "Nicht zustellbar", tone: "red" },
  blocked: { label: "Nicht gesendet", tone: "gray" },
  provider_blocked: { label: "Nicht gesendet", tone: "gray" },
  unsubscribed: { label: "Abgemeldet", tone: "gray" },
};

/** The short status pill of the last mail. */
export function mailBadge(mailing: LetterMailState): {
  label: string;
  tone: BadgeTone;
} {
  const outcome = mailing.mailOutcome
    ? MAIL_OUTCOME_BADGES[mailing.mailOutcome]
    : undefined;
  if (outcome) return outcome;
  if (mailing.status === "not_sent" && !mailing.recipientEmail.trim()) {
    return { label: "Ohne E-Mail", tone: "outline" };
  }
  return {
    label: STATUS_LABELS[mailing.status],
    tone: STATUS_TONES[mailing.status],
  };
}

// The provider's block as it is now decides, not what became of the last mail. The
// family still needs the letter, so a block points to the print.
export function heldBackNote(mailing: LetterMailState): string | null {
  if (mailing.providerBlock === "unreadable") {
    return "Ob diese Adresse gesperrt ist, lässt sich gerade nicht prüfen. Bitte versuchen Sie es später noch einmal.";
  }
  if (mailing.providerBlock !== "active") return null;
  if (mailing.mailOutcome === "unsubscribed") {
    return "Diese Person hat sich von den Mails abgemeldet. Bitte drucken Sie das Schreiben.";
  }
  if (mailing.mailOutcome === "complained") {
    return "Diese Person hat eine Mail als unerwünscht gemeldet. Bitte drucken Sie das Schreiben.";
  }
  return "Diese Adresse ist gesperrt. Bitte drucken Sie das Schreiben. Der moto-Support klärt die Sperre.";
}

/** The line beside the short status: when, or what happens next. */
export function mailStatusDetail(mailing: LetterMailState): string | null {
  const at = (iso: string) =>
    `${formatBerlinDate(iso)}, ${formatTime(iso)} Uhr`;
  if (mailing.mailOutcome === "delayed" || mailing.mailOutcome === "overdue") {
    return mailStatusCopy(mailing.mailOutcome)?.explanation ?? null;
  }
  const heldBack = heldBackNote(mailing);
  if (heldBack) return heldBack;
  if (mailing.providerBlock === "none") {
    return (
      mailStatusCopy(mailing.mailOutcome, "", false, "none")?.explanation ??
      null
    );
  }
  if (mailing.status === "sent" && mailing.sentAt) {
    return `am ${at(mailing.sentAt)}`;
  }
  if (mailing.status === "unknown" && mailing.requestedAt) {
    return `Versand angefragt am ${at(mailing.requestedAt)}. Ob die Mail ankam, ist nicht bekannt.`;
  }
  return null;
}

/**
 * A mail can go out now: the family has an address, no mail waits or is
 * delayed, and the provider holds nothing back.
 */
export function canSendMail(mailing: LetterMailState): boolean {
  return (
    Boolean(mailing.recipientEmail.trim()) &&
    mailing.status !== "queued" &&
    mailing.mailOutcome !== "delayed" &&
    !heldBackNote(mailing)
  );
}
