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
          "`Überfällig`: Die Mail ist seit Tagen verzögert. Sie können sie noch einmal senden.",
          "`Nicht zustellbar`: Die Mail kam nicht an.",
          "`Nicht gesendet`: Die Mail ging nicht raus. Der Hinweis nennt den Grund. Besteht er nicht mehr, können Sie neu senden.",
          "`Gesperrt`: Diese Adresse ist gesperrt. Eine neue Mail hilft nicht. Der moto-Support muss das klären.",
          "`Stand unklar`: Ob die Adresse gesperrt ist, lässt sich gerade nicht prüfen. Versuchen Sie es später noch einmal.",
          "`Nicht versendet`: Es gab keine Mail, zum Beispiel weil die Adresse fehlt.",
          "`Fehlgeschlagen`: Das Senden hat nicht geklappt. Bitte senden Sie neu.",
          "`Ergebnis unbekannt`: Ob die Mail rausging, ist nicht bekannt.",
          "`Abgebrochen`: Die Mail wurde nicht mehr gesendet.",
          "`Abgemeldet`: Die Person hat sich von den Mails abgemeldet. Sprechen Sie sie direkt an.",
          "`Adresse prüfen`: An diese Adresse geht keine Mail mehr raus, weil sie gesperrt ist.",
          "`Versand gestoppt`: Ihre Schule hat die Mails an diese Person gestoppt. Nutzt noch jemand diese Adresse, gehen Mails an die Adresse weiter raus.",
        ],
      },
      {
        title: "Einen Elternbrief neu senden",
        description:
          "Das geht für Mails, die nicht ankamen. Gesperrte Adressen und gestoppte Personen bekommen keine neue Mail.",
        steps: [
          "Öffnen Sie den Elternbrief und sehen Sie sich den Status an.",
          "Steht bei einer Person `Adresse prüfen`? Prüfen Sie zuerst die Adresse wie unten beschrieben.",
          "Wählen Sie `Fehlgeschlagene erneut senden`. moto sendet an die aktuelle Adresse der Person.",
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
          "Zum Beispiel, wenn eine Person keine Mails der Schule mehr bekommen möchte. Der Stopp bleibt, auch wenn sich die Adresse ändert.",
        steps: [
          "Öffnen Sie das Kind und den Reiter `Erziehungsberechtigte`.",
          "Wählen Sie im Menü der Person `Mails stoppen` und bestätigen Sie mit `Stoppen`.",
          "Später heben Sie das im selben Menü mit `Mails wieder senden` auf.",
        ],
      },
    ],
    notes: [
      "Anmeldecodes und Mails zum Passwort kommen auch an eine gesperrte Adresse.",
      "Abrechnungsbriefe des Trägers gehen auch an gesperrte Adressen und gestoppte Personen.",
      "Mails an das Team hält eine Sperre nicht zurück. Die Einladung zeigt den Hinweis trotzdem.",
    ],
    troubleshootingDetails: [
      "Hat die Person eine Mail als unerwünscht gemeldet? Dann können Sie die Sperre nicht aufheben. Sprechen Sie die Person direkt an. Eine andere Adresse tragen Sie unter `Bearbeiten` ein.",
      "`Erneut einladen` fehlt? Dann ist die Adresse gesperrt oder die letzte Mail noch unterwegs. Der Hinweis bei der Person sagt, was davon zutrifft.",
      "Steht bei der Person `Gesperrt`? Die Schule kann das nicht aufheben. Wenden Sie sich an den moto-Support. Die Sperre lässt sich nicht weiter zuordnen.",
      "Hat sich die Person abgemeldet oder eine Mail als unerwünscht gemeldet? An dieselbe Adresse geht kein Elternbrief mehr. Hat sie eine andere Adresse? Tragen Sie diese unter `Bearbeiten` ein. Dann können Sie den Brief neu senden.",
      "Ist die Adresse in einem Elternbrief geändert? Die Zeile nennt die Adresse, an die die Mail ging, und darunter die aktuelle Adresse. Das Ergebnis gehört zur ersten.",
      "Kommt nach dem Aufheben wieder eine Mail zurück? Dann sperrt moto die Adresse erneut. Fragen Sie die Familie nach einer anderen Adresse.",
    ],
    related: [
      HELP_TOPICS.leadInviteGuardians,
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}
