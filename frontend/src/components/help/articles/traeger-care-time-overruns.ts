import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/zeitvergleich` (#4683): wie oft Kinder an den Schulen des Trägers
 * länger da waren als gebucht, je Schule und mit Recht auch je Kind.
 */
export function traegerCareTimeOverrunsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareTimeOverruns,
    title: "Länger als gebucht",
    question: "Welche Kinder waren länger da als gebucht?",
    summary:
      "moto vergleicht die Zeiten im Vertrag mit dem Ein- und Auschecken. Sie sehen je Schule, wie oft Kinder länger da waren.",
    group: "auswertung",
    audience: "traeger",
    icon: "Hourglass",
    requirements: [
      "Für die Zahlen je Schule brauchen Sie das Recht `Berichte lesen`. Es gehört zu den Rollen `Verwaltung`, `Abrechnung`, `Personal` und `Nur lesen`.",
      "Für die Namen der Kinder brauchen Sie das Recht `Verträge verwalten`. Es gehört zur Rolle `Verwaltung`.",
    ],
    steps: [
      "Wählen Sie in der Navigation `Länger als gebucht`.",
      "Wählen Sie oben den Monat. Am Anfang ist es der laufende Monat bis heute.",
      "Wählen Sie bei Bedarf eine Schule. Sonst sehen Sie alle Schulen.",
      "Lesen Sie oben die Summen: `Kinder`, `Tage` und `Zusätzliche Zeit`.",
      "Lesen Sie unter `Je Schule` die Zahlen jeder Schule. Tippen Sie auf eine Spalte, um nach ihr zu sortieren.",
      "Mit der Rolle `Verwaltung` sehen Sie unter `Kinder` jedes Kind mit Namen. Das Kind mit der meisten zusätzlichen Zeit steht oben.",
    ],
    result:
      "Rote Zahlen zeigen Überschreitungen. Sprechen Sie mit den Eltern, oder ändern Sie die Buchung.",
    notes: [
      "Ändern Sie die Zeiten eines Zeitmodells? Die neuen Zeiten gelten ab dem Änderungstag. Vergangene gebuchte Tage behalten ihre bisherigen Zeiten. Auch nach Umbuchung oder Archivierung bleiben diese Zeiten erhalten.",
      "Das Soll kommt aus dem versendeten Vertrag: je Wochentag die Module und die Endzeit. Ohne Vertrag gilt der Betreuungsplan der Schule.",
      "An einem Tag mit bestätigter Ferienbuchung gilt die Ferienbuchung, nicht der Vertrag. Jeder Tag zählt nur einmal.",
      "Das Ist reicht vom ersten Einchecken bis zum letzten Auschecken des Tages.",
      "Eine Überschreitung ist mehr als 15 Minuten früher oder später als gebucht. Ein Tag ohne Buchung zählt ganz.",
      "Krank, entschuldigt, abgemeldet, Schließtage und Feiertage zählen nicht.",
      "Jedes Mal, wenn Sie Namen sehen, steht das im Zugriffsprotokoll der Schule.",
    ],
    differences: [
      "Fehlt der Bereich mit den Namen? Ihre Rolle hat das Recht `Verträge verwalten` nicht. Dann sehen Sie nur die Zahlen je Schule.",
      "Steht bei einer Schule `Check-ins erst ab` mit einem Datum? Die Schule bewahrt Check-ins nicht so lange auf. Verglichen wird ab diesem Tag.",
      "Steht bei einer Schule `–` und `Keine Check-ins im Zeitraum`? Die Schule hat in diesem Zeitraum keine Check-ins. Dann kann moto nicht vergleichen. Das heißt nicht, dass alle Zeiten passen.",
      "Steht bei einer Schule `Tage nicht verglichen`? An diesen Tagen war ein Kind da, aber es gab keinen Plan oder es ist noch nicht ausgecheckt. Diese Tage zählen nicht mit.",
    ],
    related: [HELP_TOPICS.traegerCareContracts, HELP_TOPICS.traegerAccessLog],
  };
}
