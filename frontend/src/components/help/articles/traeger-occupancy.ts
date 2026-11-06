import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/belegung` (#4676): die Kinder von heute je Schule, Gruppe und Betreuungsform. */
export function traegerOccupancyTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerOccupancy,
    title: "Belegung der Schulen",
    question: "Wie viele Kinder hat jede Schule, Gruppe und Betreuungsform?",
    summary:
      "Die Belegung zeigt, wie viele Kinder Ihre Schulen heute betreuen: je Schule, je OGS-Gruppe und je Betreuungsform.",
    group: "einstieg",
    audience: "traeger",
    icon: "ChartLine",
    steps: [
      "Wählen Sie in der Navigation `Belegung`.",
      "Lesen Sie oben die Summe für alle Schulen.",
      "Wählen Sie einen Reiter: `Je Schule`, `Je Gruppe` oder `Je Betreuungsform`.",
      "Wählen Sie `Herunterladen` und dann `CSV`, um alle Zahlen als Tabelle zu speichern.",
    ],
    result:
      "Sie sehen für jede Schule, wie viele Kinder in welcher Gruppe sind und welches Betreuungsangebot sie heute gebucht haben.",
    notes: [
      "Suchen Sie oben nach einer Schule oder einem Ort. Die Summe zählt dann nur diese Schulen.",
      "Es zählen dieselben Kinder wie bei der Stichtagszahl, aber von heute.",
      "Eine Betreuungsform ist ein Betreuungsangebot, das als Betreuung zählt, zum Beispiel „OGS bis 16 Uhr“. Mittagessen zählt nicht.",
      "Unter `Je Betreuungsform` stehen Angebote mit gleichem Namen an mehreren Schulen auch zusammengezählt.",
      "Sie sehen nur Zahlen, keine Namen von Kindern.",
    ],
    differences: [
      "Steht bei einem Angebot `Keine Grenze`, hat die Schule keine Plätze festgelegt.",
      "Steht bei einem Angebot `Voll` oder `Überbelegt`, sind alle Plätze vergeben. Die Plätze legt die Schule fest.",
      "Steht bei einer Schule `Ohne Betreuungsform`, haben diese Kinder heute kein Betreuungsangebot gebucht. Das ist bei Schulen mit Wochenplan üblich.",
    ],
    related: [HELP_TOPICS.traegerDashboard, HELP_TOPICS.traegerKeyDateCounts],
  };
}
