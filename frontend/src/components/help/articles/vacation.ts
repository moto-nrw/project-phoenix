import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function vacationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.vacation,
    title: "Urlaub beantragen und den Stand prüfen",
    question: "Wie beantrage ich Urlaub und prüfe den Stand?",
    summary:
      "Senden Sie einen Urlaubsantrag und verfolgen Sie die Entscheidung.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "SunHorizon",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Zeiterfassung`.",
      "Suchen Sie die Karte `Urlaub`.",
      "Wählen Sie `Urlaub beantragen`.",
      "Wählen Sie den Zeitraum und bei Bedarf einen halben Tag.",
      "Ergänzen Sie bei Bedarf eine Notiz.",
      "Prüfen Sie Ihren Resturlaub und mögliche Überschneidungen.",
      "Senden Sie den Antrag ab.",
    ],
    result:
      "Der Antrag erscheint unter `Meine Anträge`. Dort sehen Sie, ob er offen, genehmigt, abgelehnt oder zur Rückfrage zurückgegeben wurde.",
    differences: [
      "Krankheit, Fortbildung und andere Abwesenheiten melden Sie über `Abwesend` in der Stempeluhr. Dafür stellen Sie keinen Urlaubsantrag.",
    ],
    notes: [
      "Einen offenen oder zukünftigen genehmigten Antrag können Sie stornieren.",
      "Bei einer `Rückfrage` schreiben Sie eine Antwort und wählen `Antwort senden & erneut einreichen`.",
    ],
    related: [HELP_TOPICS.ownAbsence, HELP_TOPICS.correctWorkTime],
  };
}
