import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/parents/anmeldung`: Phase waehlen, Formular ausfuellen, absenden. */
export function parentEnrollTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentEnroll,
    title: "Mein Kind anmelden",
    question: "Wie melde ich mein Kind für die OGS an?",
    summary: "Sie wählen die Anmeldung und füllen ein Formular aus.",
    group: "anmeldung",
    audience: "parent",
    icon: "ListChecks",
    steps: [
      "Tippen Sie unten auf `Mehr`.",
      "Wählen Sie `Neue Anmeldung`.",
      "Wählen Sie unter `Ihre Schulen` oder `Weitere Schulen` die Schule.",
      "Tippen Sie auf die passende Anmeldung. Das Formular öffnet sich sofort.",
      "Tragen Sie Ihre Kontaktdaten und die Angaben zum Kind ein.",
      "Wählen Sie unter `Zusätzlich wählen` mindestens ein Angebot. `Pflichtangebote` sind schon gesetzt.",
      "Fragt das Formular unten nach Ihrer `Unterschrift`? Unterschreiben Sie im Feld und tippen Sie auf `Unterschrift bestätigen`.",
      "Tippen Sie auf `Anmeldung absenden`.",
    ],
    result:
      "Sie bekommen eine Bestätigungs-E-Mail mit einem Link zum Stand Ihrer Anmeldung.",
    notes: [
      "Mit `Weiteres Kind` melden Sie Geschwister in derselben Anmeldung an.",
      "Bewahren Sie die Bestätigungs-E-Mail auf. Der Link darin führt zum Stand.",
      "Eine Anmeldung gilt für ein `Schuljahr`, eine `Ferienbetreuung` oder `Sonstiges`.",
      "Fragt das Formular nach einem Nachweis? Tippen Sie auf `Datei auswählen`. Die Datei wird sofort hochgeladen.",
      "Unter dem Knopf steht, welche Dateien erlaubt sind und wie groß sie sein dürfen. Meist sind es PDF, JPG oder PNG bis 10 MB.",
      "Auf dem Handy können Sie den Nachweis auch direkt fotografieren.",
      "Eine falsche Datei entfernen Sie mit dem Kreuz neben dem Dateinamen.",
      "Statt zu zeichnen können Sie mit `Namen tippen` Ihren Vor- und Nachnamen eintippen.",
    ],
    differences: [
      "Manche Anmeldungen sind `Nur bereits angemeldete Kinder`.",
      "Nicht jede Schule fragt nach einer Unterschrift. Ohne Feld `Unterschrift` senden Sie die Anmeldung direkt ab.",
      "An jeder Anmeldung steht `Anmeldung bis` mit der Frist.",
      "Über dem Formular steht `Zurück zu Neue Anmeldung`. Damit wählen Sie eine andere Anmeldung.",
    ],
    troubleshootingDetails: [
      "moto sagt, aktuell sei keine Anmeldephase geöffnet? Dann wenden Sie sich an die OGS.",
      "moto sagt, die Online-Anmeldung sei nicht freigeschaltet? Dann nimmt diese OGS keine Anmeldungen über moto an.",
      "moto sagt, eine hochgeladene Datei sei abgelaufen? Dann ist sie älter als einen Tag. Laden Sie sie noch einmal hoch und senden Sie die Anmeldung ab.",
      "Ein roter Hinweis nennt einen Tag und zwei Angebote? Diese Angebote gehen am selben Tag nicht zusammen. Nehmen Sie den Tag bei einem der beiden heraus.",
    ],
    related: [
      HELP_TOPICS.parentEnrollStatus,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentCareChange,
    ],
  };
}
