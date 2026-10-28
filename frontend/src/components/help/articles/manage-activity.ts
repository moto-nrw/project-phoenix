import type { HelpTopic, HelpPresenceMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function manageActivityTopic(
  presenceMode: HelpPresenceMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  if (nfcEnabled === false || presenceMode === "binary") {
    return {
      id: HELP_TOPICS.manageActivity,
      title: "Eine Aktivität anlegen oder ändern",
      question: "Wie lege ich eine Aktivität an oder ändere sie?",
      summary:
        nfcEnabled === false
          ? "Der Bereich ist nicht verfügbar, weil Ihre OGS NFC nicht nutzt."
          : "Bei einfacher Anwesenheit gibt es keine Aktivitäten mit eigener Kinderzuordnung.",
      group: "gruppen",
      audience: "caregiver",
      icon: "Sparkles",
      steps: [
        "Nutzen Sie den `Betreuungsplan`, um geplante Angebote anzusehen.",
        // Live geprueft: der Katalog `Aktivitäten` haengt an NFC, der
        // spontane Start in `Aktuelle Aufsicht` nicht.
        "Für heute können Sie in `Aktuelle Aufsicht` eine `Spontane Aktivität starten`.",
        "Fragen Sie Ihre Leitung, wenn ein Angebot dauerhaft geändert werden soll.",
      ],
      result:
        "Der Menüpunkt `Aktivitäten` wird in diesem Modus nicht angezeigt.",
      related: [HELP_TOPICS.carePlan, HELP_TOPICS.activeSupervision],
    };
  }
  if (nfcEnabled === null || presenceMode === "unknown") {
    return {
      id: HELP_TOPICS.manageActivity,
      title: "Eine Aktivität anlegen oder ändern",
      question: "Wie lege ich eine Aktivität an oder ändere sie?",
      summary:
        "Der Bereich erscheint nur bei NFC und detaillierter Anwesenheit.",
      group: "gruppen",
      audience: "caregiver",
      icon: "Sparkles",
      steps: [
        "Prüfen Sie, ob `Aktivitäten` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Wählen Sie `Aktivität erstellen`.",
        "Fehlt der Bereich? Fragen Sie Ihre Leitung nach der Arbeitsweise Ihrer OGS.",
      ],
      result:
        "Nur verfügbare Aktivitäten können am NFC-Tablet ausgewählt werden.",
      related: [HELP_TOPICS.activeSupervision, HELP_TOPICS.carePlan],
    };
  }

  return {
    id: HELP_TOPICS.manageActivity,
    title: "Eine Aktivität anlegen oder ändern",
    question: "Wie lege ich eine Aktivität an oder ändere sie?",
    summary:
      "Legen Sie eine Aktivität für NFC-Tablets an oder ändern Sie eine eigene Aktivität.",
    group: "gruppen",
    audience: "caregiver",
    icon: "Sparkles",
    steps: [
      "Öffnen Sie `Aktivitäten`.",
      "Wählen Sie die Schaltfläche `Aktivität erstellen` mit dem Plus-Symbol.",
      "Geben Sie einen kurzen Namen ein.",
      "Wählen Sie eine Kategorie und bei Bedarf eine maximale Anzahl an Kindern.",
      "Wählen Sie `Aktivität erstellen`.",
      "Öffnen Sie eine eigene Aktivität und wählen Sie `Bearbeiten`, wenn Sie Angaben ändern möchten.",
    ],
    result:
      "Die Aktivität ist sofort am NFC-Tablet verfügbar. Suche und Filter helfen in der Liste.",
    differences: [
      "Eine Aktivität kann am Tablet mehrfach gewählt werden. Termine mit Uhrzeit stehen im Betreuungsplan.",
      "Aktivitäten anderer Personen können Sie ansehen, aber nicht immer bearbeiten.",
    ],
    related: [HELP_TOPICS.activeSupervision, HELP_TOPICS.carePlan],
  };
}
