import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { unavailableNfcTopic, unknownNfcTopic } from "../help-article-shared";

export function nfcWorkTimeTopic(nfcEnabled: boolean | null): HelpTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.nfcWorkTime,
      "Die eigene Arbeitszeit mit dem Armband erfassen",
      "Wie erfasse ich meine Arbeitszeit mit dem Armband?",
      "Timer",
      [HELP_TOPICS.trackWorkTime, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcWorkTime,
      "Die eigene Arbeitszeit mit dem Armband erfassen",
      "Wie erfasse ich meine Arbeitszeit mit dem Armband?",
      "Timer",
      [HELP_TOPICS.trackWorkTime, HELP_TOPICS.nfcProblem],
    );
  }

  return {
    id: HELP_TOPICS.nfcWorkTime,
    title: "Die eigene Arbeitszeit mit dem Armband erfassen",
    question: "Wie erfasse ich meine Arbeitszeit mit dem Armband?",
    summary: "Stempeln Sie sich am NFC-Tablet ein, aus oder in eine Pause.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "Timer",
    requirements: ["Ihr persönliches, aktives NFC-Armband"],
    steps: [
      "Öffnen Sie im `Menü` den Bereich `Mitarbeiter-Stempeln`.",
      "Wählen Sie `Armband scannen` und halten Sie Ihr Armband an den NFC-Sensor.",
      "Wählen Sie `Einstempeln`.",
      "Scannen Sie das Armband erneut, um `Pause starten`, `Pause beenden` oder `Ausstempeln` zu wählen.",
      "Geben Sie bei einer Abweichung vom Dienstplan den verlangten Grund ein.",
    ],
    result:
      "Der neue Status erscheint am Tablet. Die Buchung steht auch in Ihrer `Zeiterfassung` in der moto-App.",
    differences: [
      "Kinderarmbänder und nicht zugewiesene Armbänder funktionieren für die persönliche Arbeitszeit nicht.",
      "Eine laufende Pause wird als `In Pause` angezeigt.",
      "Ihre Schule kann das Einstempeln erst kurz vor Ihrer Schicht erlauben. Dann zeigt das Tablet, ab wann es geht.",
    ],
    notes: [
      "Am Tablet gibt es keine Wahl des Arbeitsorts. Wer dort einstempelt, ist in der OGS. Arbeiten Sie unterwegs, stempeln Sie in der moto-App unter `Zeiterfassung` ein.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.trackWorkTime, HELP_TOPICS.tagAssignment],
  };
}
