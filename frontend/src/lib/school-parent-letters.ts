// Elternbriefe der Lehrkraft in moto schule (#4800): was die Lehrkraft im
// Formular wählt und wie daraus die Elternmitteilung wird, die dieselbe ist
// wie im OGS-Portal. Die Empfänger sind die Eltern der gewählten Klasse; das
// Backend prüft, dass die Klasse zur Lehrkraft gehört.

import type {
  Announcement,
  AnnouncementInput,
} from "./parent-announcements-api";

/** Die Adresse der Elternbriefe in moto schule. */
export const SCHOOL_PARENT_LETTERS_ROUTE = "/school/elternbriefe";

/** SWR-Schlüssel der eigenen Elternbriefe. */
export const SCHOOL_PARENT_LETTERS_KEY = "school-parent-letters";

/** Was die Eltern mit dem Brief tun. */
type SchoolParentLetterKind = "info" | "letter" | "consent";

export interface SchoolParentLetterKindOption {
  readonly value: SchoolParentLetterKind;
  /** Die Art der Elternmitteilung, die daraus wird. */
  readonly deliveryMode: Announcement["delivery_mode"];
  readonly label: string;
  readonly hint: string;
}

export const SCHOOL_PARENT_LETTER_KINDS: readonly SchoolParentLetterKindOption[] =
  [
    {
      value: "info",
      deliveryMode: "standard",
      label: "Nur zur Information",
      hint: "Die Eltern lesen den Brief. Sie müssen nichts tun.",
    },
    {
      value: "letter",
      deliveryMode: "letter",
      label: "Mit Lesebestätigung",
      hint: "Die Eltern bestätigen, dass sie den Brief gelesen haben.",
    },
    {
      value: "consent",
      deliveryMode: "declaration",
      label: "Einverständnis mit Unterschrift",
      hint: "Die Eltern stimmen zu oder lehnen ab. Zum Zustimmen unterschreiben sie.",
    },
  ];

/** Die Art eines gesendeten Briefs, mit derselben Bezeichnung wie im Formular. */
export function schoolParentLetterKindLabel(
  deliveryMode: Announcement["delivery_mode"],
): string {
  return (
    SCHOOL_PARENT_LETTER_KINDS.find(
      (option) => option.deliveryMode === deliveryMode,
    )?.label ?? ""
  );
}

/** Der Stand eines Briefs in den Worten von moto schule: gesendet statt veröffentlicht. */
export const SCHOOL_PARENT_LETTER_STATUS: Readonly<
  Record<
    Announcement["status"],
    { label: string; tone: "gray" | "green" | "orange" }
  >
> = {
  draft: { label: "Entwurf", tone: "gray" },
  published: { label: "Gesendet", tone: "green" },
  expired: { label: "Abgelaufen", tone: "orange" },
};

export interface SchoolParentLetterDraft {
  readonly schoolClass: string;
  readonly title: string;
  readonly body: string;
  readonly kind: SchoolParentLetterKind;
}

export function emptySchoolParentLetterDraft(
  classes: readonly string[],
): SchoolParentLetterDraft {
  return {
    schoolClass: classes.length === 1 ? (classes[0] ?? "") : "",
    title: "",
    body: "",
    kind: "info",
  };
}

/** Feldfehler des Formulars, leer wenn der Brief gesendet werden kann. */
export function schoolParentLetterFieldErrors(
  draft: SchoolParentLetterDraft,
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (draft.schoolClass.trim() === "") {
    errors.school_class = "Bitte wählen Sie eine Klasse.";
  }
  if (draft.title.trim() === "") {
    errors.title = "Bitte geben Sie einen Betreff ein.";
  }
  if (draft.body.trim() === "") {
    errors.body = "Bitte schreiben Sie einen Text.";
  }
  return errors;
}

/**
 * Der Brief als Elternmitteilung: genau die gewählte Klasse, immer mit
 * E-Mail an die Eltern mit Zugang zu moto. Ein Brief mit Lesebestätigung
 * ist ein Elternbrief, ein Einverständnis verlangt die Unterschrift.
 */
export function schoolParentLetterRequest(
  draft: SchoolParentLetterDraft,
): AnnouncementInput {
  const base: AnnouncementInput = {
    title: draft.title.trim(),
    body: draft.body.trim(),
    priority: "info",
    requires_acknowledgement: false,
    send_email: true,
    email_audience: "portal_only",
    targets: [{ target_type: "class", ref_text: draft.schoolClass }],
  };
  switch (draft.kind) {
    case "letter":
      return {
        ...base,
        delivery_mode: "letter",
        requires_acknowledgement: true,
      };
    case "consent":
      return {
        ...base,
        delivery_mode: "declaration",
        declaration_kind: "consent",
        declaration_signers: "any",
        declaration_requires_signature: true,
      };
    default:
      return { ...base, delivery_mode: "standard" };
  }
}
