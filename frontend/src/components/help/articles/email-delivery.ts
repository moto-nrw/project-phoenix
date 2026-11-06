import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Die Zustellhinweise (#5934). Sie stehen an fünf Stellen: Einladung der Eltern,
 * Elternbrief, Einladung ins Personal, Anmeldung und, für den Träger, im Versand
 * der Schreiben. Hier steht, was jedes Wort bedeutet und wer als Nächstes handelt.
 * `Versendet` heißt nur: Die Mail ist raus. `Zugestellt` meldet erst der
 * Mail-Dienst, und ohne ihn (Strato, Brevo, Notweg) bleibt es bei `Versendet`.
 */
export function emailDeliveryTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEmailDelivery,
    title: "Wenn eine E-Mail nicht ankommt",
    question: "Was bedeuten Zugestellt, Verzögert und Adresse prüfen?",
    summary:
      "moto zeigt bei jeder Mail an Eltern, was aus ihr geworden ist und was Sie tun können.",
    group: "elternarbeit",
    audience: "lead",
    icon: "EnvelopeSimple",
    steps: [],
    instructionGroups: [
      {
        title: "Das bedeuten die Hinweise",
        ordered: false,
        steps: [
          "`Wird gesendet`: Die Mail wartet noch.",
          "`Versendet`: Die Mail ist raus. Ob sie im Postfach ankam, ist nicht bekannt.",
          "`Zugestellt`: Die Mail ist angekommen.",
          "`Verzögert`: Die Mail kommt später an. Es wird weiter versucht. Bitte senden Sie sie nicht noch einmal.",
          "`Nicht zustellbar`: Die Mail kam nicht an.",
          "`Adresse prüfen`: An diese Adresse geht keine Mail mehr raus, weil sie gesperrt ist.",
          "`Versand gestoppt`: Ihre Schule hat die Mails an diese Adresse gestoppt.",
        ],
      },
      {
        title: "Eine gesperrte Adresse prüfen",
        description:
          "Die Sperre gilt, bis jemand an Ihrer Schule die Adresse geprüft hat.",
        steps: [
          "Öffnen Sie das Kind und den Reiter `Erziehungsberechtigte`.",
          "Fragen Sie die Familie nach der richtigen Adresse.",
          "Ist die Adresse falsch: Wählen Sie im Menü der Person `Bearbeiten` und tragen Sie die richtige ein.",
          "Ist die Adresse richtig: Wählen Sie im Menü der Person `Sperre aufheben` und bestätigen Sie.",
          "Senden Sie danach neu, zum Beispiel mit `Erneut einladen`.",
        ],
      },
      {
        title: "Mails an eine Person stoppen",
        description:
          "Zum Beispiel, wenn eine Person keine Mails der Schule mehr bekommen möchte.",
        steps: [
          "Öffnen Sie das Kind und den Reiter `Erziehungsberechtigte`.",
          "Wählen Sie im Menü der Person `Mails stoppen` und bestätigen Sie mit `Stoppen`.",
          "Später heben Sie das im selben Menü mit `Mails wieder senden` auf.",
        ],
      },
    ],
    notes: [
      "Anmeldecodes und Mails zum Passwort kommen auch an eine gesperrte Adresse.",
      "Mails an das Team hält eine Sperre nicht zurück. Die Einladung zeigt den Hinweis trotzdem.",
    ],
    troubleshootingDetails: [
      "Hat die Person eine Mail als unerwünscht gemeldet? Dann können Sie die Sperre nicht aufheben. Sprechen Sie die Person direkt an. Eine andere Adresse tragen Sie unter `Bearbeiten` ein.",
      "`Erneut einladen` fehlt? Dann ist die Adresse gesperrt oder die letzte Mail noch unterwegs. Der Hinweis bei der Person sagt, was davon zutrifft.",
      "Kommt nach dem Aufheben wieder eine Mail zurück? Dann sperrt moto die Adresse erneut. Fragen Sie die Familie nach einer anderen Adresse.",
    ],
    related: [
      HELP_TOPICS.leadInviteGuardians,
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}
