import type { StatusBadgeTone } from "~/components/ui/status-badge";

/**
 * What became of a mail, as Delivery names it (#5934). "accepted" only means a
 * mail server took it; "delivered" needs a report of the mail provider. The two
 * must never read alike.
 */
export type MailOutcome =
  | "queued"
  | "accepted"
  | "delivered"
  | "delayed"
  | "undeliverable"
  | "blocked"
  | "complained"
  | "send_failed"
  | "unknown"
  | "cancelled";

/** Why the school's list holds an address. It stays until someone checked it. */
export type EmailAddressMark = "failed" | "complained" | "school_stop";

export interface MailStatusCopy {
  readonly label: string;
  readonly tone: StatusBadgeTone;
  /** One line that says what happened and who acts next. */
  readonly explanation?: string;
}

const ADDRESS_COPY: Record<EmailAddressMark, MailStatusCopy> = {
  failed: {
    label: "Adresse prüfen",
    tone: "red",
    explanation:
      "Mails an diese Adresse kommen nicht an. Sie ist deshalb gesperrt, und moto sendet nichts mehr dorthin.",
  },
  complained: {
    label: "Adresse prüfen",
    tone: "red",
    explanation:
      "Diese Person hat eine Mail als unerwünscht gemeldet. Die Adresse ist deshalb gesperrt. Sprechen Sie die Person direkt an.",
  },
  school_stop: {
    label: "Versand gestoppt",
    tone: "gray",
    explanation:
      "Ihre Schule hat die Mails an diese Adresse gestoppt. Anmeldecodes und Mails zum Passwort kommen weiter an.",
  },
};

const OUTCOME_COPY: Record<MailOutcome, MailStatusCopy> = {
  queued: { label: "Wird gesendet", tone: "blue" },
  accepted: {
    label: "Versendet",
    tone: "gray",
    explanation:
      "Die Mail ist raus. Ob sie im Postfach angekommen ist, ist nicht bekannt.",
  },
  delivered: { label: "Zugestellt", tone: "green" },
  delayed: {
    label: "Verzögert",
    tone: "orange",
    explanation:
      "Die Mail kommt später an, es wird weiter versucht. Bitte senden Sie sie nicht noch einmal, sonst kommt sie doppelt an.",
  },
  undeliverable: {
    label: "Nicht zustellbar",
    tone: "red",
    explanation: "Die Mail kam nicht an. Bitte prüfen Sie die Adresse.",
  },
  blocked: {
    label: "Nicht gesendet",
    tone: "gray",
    explanation:
      "Die Mail ging nicht raus, weil die Adresse gesperrt war. Die Sperre ist aufgehoben, Sie können neu senden.",
  },
  complained: {
    label: "Nicht zustellbar",
    tone: "red",
    explanation: "Diese Person hat die Mail als unerwünscht gemeldet.",
  },
  send_failed: {
    label: "Fehlgeschlagen",
    tone: "red",
    explanation: "Das Senden hat nicht geklappt. Bitte senden Sie neu.",
  },
  unknown: {
    label: "Ergebnis unbekannt",
    tone: "gray",
    explanation: "Ob die Mail rausging, ist nicht bekannt.",
  },
  cancelled: { label: "Abgebrochen", tone: "gray" },
};

/**
 * The words for one mail and its address. A mark outranks the mail: while the
 * address is held, the next step is to check it, whatever the last mail did.
 * Returns null when nothing is known yet, so a caller shows no status at all.
 */
export function mailStatusCopy(
  outcome?: MailOutcome | "",
  mark?: EmailAddressMark | "",
): MailStatusCopy | null {
  if (mark) return ADDRESS_COPY[mark];
  if (!outcome) return null;
  return OUTCOME_COPY[outcome] ?? null;
}

/**
 * The next step on a guardian's card. Only a bounced address can be freed by the
 * school: a complaint stays, and a stop is the school's own decision.
 */
export function guardianAddressNextStep(mark: EmailAddressMark): string {
  switch (mark) {
    case "failed":
      return "Stimmt die Adresse nicht, ändern Sie sie unter „Bearbeiten“. Stimmt sie, wählen Sie „Sperre aufheben“.";
    case "complained":
      return "Sie können diese Sperre nicht aufheben. Eine andere Adresse tragen Sie unter „Bearbeiten“ ein.";
    case "school_stop":
      return "Mit „Mails wieder senden“ heben Sie den Stopp auf.";
  }
}

/** Where a school corrects an address it sees outside the guardian's card. */
export const CHECK_ADDRESS_AT_GUARDIAN =
  "Korrigieren Sie die Adresse beim Kind unter „Erziehungsberechtigte“.";
