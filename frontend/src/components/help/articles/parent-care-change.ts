import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Zwei getrennte Anfragen: der Wochenplan (i18n `careSchedule`) und die
 * gebuchten Angebote (i18n `careOfferings`). Welche moeglich ist, entscheidet
 * die OGS -- bei buchungsgefuehrten Schulen laeuft alles ueber die Angebote.
 */
export function parentCareChangeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentCareChange,
    title: "Eine Änderung der Betreuung anfragen",
    question: "Wie ändere ich die Betreuungstage meines Kindes?",
    summary: "Sie stellen eine Anfrage. Die OGS entscheidet darüber.",
    group: "mein-kind",
    audience: "parent",
    icon: "CalendarDays",
    steps: [],
    instructionGroups: [
      {
        title: "Den Wochenplan ändern",
        steps: [
          "Tippen Sie unten auf `Mein Kind`.",
          "Wählen Sie den Reiter `Betreuung`.",
          "Tippen Sie bei `Dein OGS Wochenplan` auf `Änderungen anfragen`.",
          "Schalten Sie je Tag `Betreuung an diesem Tag` ein oder aus.",
          "Wählen Sie bei einem Tag `Geht allein`? Dann unterschreiben Sie die Erlaubnis im Feld und tippen auf `Unterschrift bestätigen`.",
          "Tippen Sie auf `Anfrage an OGS senden`.",
        ],
      },
      {
        // #3075, #4767: Kurse mit Warteliste.
        title: "Einen Kurs anfragen",
        steps: [
          "Gehen Sie im Reiter `Betreuung` zum Abschnitt `Kurse`.",
          "Tippen Sie beim gewünschten Kurs auf `Anfragen`.",
          "Ist der Kurs voll? Dann kommt Ihre Anfrage auf die Warteliste. Sie sehen Ihren Platz, zum Beispiel `Warteliste, Platz 2`.",
          "Wird ein Platz frei, bestätigt die OGS die Anfrage in dieser Reihenfolge. Sie bekommen dann eine Nachricht.",
        ],
      },
      {
        title: "Die gebuchte Betreuung ändern",
        steps: [
          "Gehen Sie zum Abschnitt `Gebuchte Betreuung`.",
          "Tippen Sie auf `Betreuung ändern`.",
          "Wählen Sie die gewünschten Angebote.",
          "Senden Sie die Anfrage ab.",
        ],
      },
    ],
    result:
      "Bis zur Bestätigung bleibt alles wie bisher. Die Anfrage trägt `In Prüfung`.",
    notes: [
      "Eine Änderung am Wochenplan ist dauerhaft und gilt jede Woche.",
      "Geht es nur um einen Tag? Ändern Sie stattdessen die Abholung für diesen Tag.",
      "Eine offene Anfrage lässt sich mit `Anfrage bearbeiten` noch ändern.",
      "Betreut Ihre OGS am Wochenende? Dann gilt der Freitagsplan auch für Samstag und Sonntag.",
    ],
    differences: [
      "Die Entscheidung der OGS steht unter `Beantragte Änderung`: `Änderung übernommen` oder `Anfrage abgelehnt` mit Begründung.",
      "Sie sehen nur Änderungen, die Ihre OGS erlaubt.",
    ],
    troubleshootingDetails: [
      "moto sagt, Ihre OGS nehme keine Anfragen zum Wochenplan an? Dann laufen Änderungen über die gebuchten Angebote.",
      "moto sagt, für dieses Kind werde bereits eine Änderung geprüft? Warten Sie die Entscheidung ab.",
      "moto sagt, der Betreuungszeitraum sei abgelaufen? Dann melden Sie Ihr Kind für den neuen Zeitraum neu an.",
      "Ein roter Hinweis nennt einen Tag und zwei Angebote? Diese Angebote gehen am selben Tag nicht zusammen. Nehmen Sie den Tag bei einem der beiden heraus.",
    ],
    related: [
      HELP_TOPICS.parentPickupChange,
      HELP_TOPICS.parentEnroll,
      HELP_TOPICS.parentChildOverview,
    ],
  };
}
