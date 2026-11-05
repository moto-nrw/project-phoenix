import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/uebersicht` (#4674), in der Navigation „Dashboard": die Zahlen von jetzt für alle Schulen. */
export function traegerDashboardTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerDashboard,
    title: "Alle Schulen auf einen Blick",
    question: "Wie steht es gerade an allen Schulen meines Trägers?",
    summary:
      "Das Dashboard zeigt die Zahlen von jetzt für alle Schulen Ihres Trägers auf einer Seite.",
    group: "einstieg",
    audience: "traeger",
    icon: "BarChart3",
    steps: [
      "Wählen Sie in der Navigation `Dashboard`.",
      "Lesen Sie oben die Summe für alle Schulen.",
      "Lesen Sie unter `Zahlen je Schule` die Zahlen jeder Schule.",
      "Wählen Sie eine Schule, um ihre Zahlen und Stammdaten zu sehen.",
    ],
    result:
      "Sie sehen für jede Schule, wie viele Kinder gerade da sind, wie viele heute krank sind und wie viele Betreuungskräfte angemeldet sind.",
    notes: [
      "Die Zahlen aktualisieren sich jede Minute von selbst.",
      "Sie sehen nur Zahlen, keine Namen.",
      "Mit einem Klick auf eine Spaltenüberschrift sortieren Sie die Schulen, zum Beispiel nach `Heute krank`.",
      "Bei vielen Schulen finden Sie eine Schule schneller über das Suchfeld. Sie können nach dem Namen oder dem Ort suchen.",
      "`Betreuungsschlüssel` heißt: Eine anwesende Betreuungskraft kommt auf so viele anwesende Kinder. Oben gilt das für alle Schulen zusammen.",
    ],
    differences: [
      "`Betreuungskräfte` zählt nur, wer sich in der Zeiterfassung angemeldet hat. Homeoffice zählt nicht. Nutzt eine Schule die Zeiterfassung nicht, steht dort 0 und beim Schlüssel `–`.",
      "Ohne das Recht `In OGS wechseln` fehlt die Aktion `Schul-App öffnen`. Die Zahlen sehen Sie trotzdem.",
    ],
    related: [HELP_TOPICS.traegerSchoolView, HELP_TOPICS.traegerSchoolSwitch],
  };
}
