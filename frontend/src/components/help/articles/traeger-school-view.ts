import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/schulen/{id}` (#5686): Ansicht einer Schule im Träger-Portal. */
export function traegerSchoolViewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerSchoolView,
    title: "Zahlen und Stammdaten einer Schule ansehen",
    question: "Wie steht es gerade an einer Schule meines Trägers?",
    summary:
      "Sie sehen die Zahlen und Stammdaten einer Schule, ohne das Träger-Portal zu verlassen.",
    group: "einstieg",
    audience: "traeger",
    icon: "ChartPieSlice",
    steps: [
      "Wählen Sie in der Navigation `Schulen`.",
      "Wählen Sie die Schule in der Liste.",
      "Wählen Sie den Reiter `Übersicht` oder `Stammdaten`.",
    ],
    result:
      "`Übersicht` zeigt die Zahlen von jetzt. `Stammdaten` zeigt Anschrift, Kontakt und die Zahl der Kinder und Betreuungskräfte.",
    notes: [
      "Die Zahlen aktualisieren sich jede Minute von selbst.",
      "Sie sehen nur Zahlen, keine Namen.",
      "`Kinder gesamt` ist dieselbe Zahl wie im Abrechnungsbericht des moto-Teams.",
      "`Betreuungsschlüssel` heißt: Eine anwesende Betreuungskraft kommt auf so viele anwesende Kinder.",
      "Stammdaten können Sie hier nur lesen. Änderungen übernimmt das moto-Team.",
    ],
    differences: [
      "`Betreuungskräfte anwesend` zählt nur, wer sich in der Zeiterfassung angemeldet hat. Homeoffice zählt nicht. Nutzt eine Schule die Zeiterfassung nicht, steht dort 0 und beim Betreuungsschlüssel `–`.",
    ],
    related: [HELP_TOPICS.traegerSchoolSwitch],
  };
}
