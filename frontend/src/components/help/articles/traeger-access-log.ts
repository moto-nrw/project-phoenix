import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/access-log` (#4655): wer an den Schulen Daten gelesen oder geändert hat. */
export function traegerAccessLogTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerAccessLog,
    title: "Zugriffsprotokoll lesen",
    question: "Wie sehe ich, wer Daten gelesen oder geändert hat?",
    summary:
      "Das Zugriffsprotokoll zeigt, wer an Ihren Schulen Daten von Kindern, Eltern oder Personal gelesen oder geändert hat.",
    group: "auswertung",
    audience: "traeger",
    icon: "ShieldCheck",
    requirements: [
      "Sie haben im Träger-Büro die Rolle Verwaltung. Nur diese Rolle darf das Protokoll lesen.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Protokoll ansehen",
        steps: [
          "Melden Sie sich im Träger-Portal an.",
          "Wählen Sie in der Navigation `Zugriffsprotokoll`.",
          "Wählen Sie den Zeitraum. Am Anfang sind es die letzten 30 Tage.",
          "Wählen Sie bei Bedarf eine Schule und eine Person.",
          "Wählen Sie `Gelesen` oder `Geändert`, wenn Sie nur eine Art sehen möchten.",
          "Gibt es mehr Einträge? Wählen Sie unten `Weitere Einträge laden`.",
        ],
      },
      {
        title: "Als CSV herunterladen",
        description: "Für einen Nachweis, zum Beispiel in einer Tabelle.",
        steps: [
          "Stellen Sie die Filter ein.",
          "Wählen Sie `Herunterladen` und dann `CSV`.",
          "moto lädt eine Datei mit allen Einträgen zu den Filtern herunter.",
        ],
      },
    ],
    result:
      "Jede Zeile zeigt Zeitpunkt, Person, Schule, Art und welche Daten es waren.",
    notes: [
      "Auch Ihr Lesen steht im Protokoll. Die Schule sieht so, dass das Träger-Büro hineingeschaut hat.",
    ],
    differences: [
      "Sie sehen in der Navigation kein `Zugriffsprotokoll`? Dann hat Ihre Rolle kein Recht dazu.",
      "Steht bei Person `Unbekannte Person`? Zu diesem Konto kennt moto an der Schule keinen Namen, zum Beispiel bei Eltern.",
    ],
    troubleshootingDetails: [
      "moto sagt, der Export habe zu viele Einträge? Wählen Sie einen kürzeren Zeitraum oder eine Schule.",
      "Sie brauchen das Recht für Ihr Konto? Wenden Sie sich an das moto-Team.",
    ],
    related: [HELP_TOPICS.traegerLogin],
  };
}
