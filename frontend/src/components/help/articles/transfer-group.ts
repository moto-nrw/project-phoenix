import type { HelpTopic, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function transferGroupTopic(groupMode: HelpGroupMode): HelpTopic {
  if (groupMode === "open_care") {
    return {
      id: HELP_TOPICS.transferGroup,
      title: "Meine Gruppe vorübergehend übergeben",
      question: "Wie übergebe ich meine Gruppe vorübergehend?",
      summary:
        "Bei offener Betreuung gibt es keine feste eigene Gruppe zum Übergeben.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "ArrowsLeftRight",
      steps: [
        "Sie müssen keine Gruppe übergeben.",
        "Öffnen Sie `Alle Kinder`, um mit den Kindern der OGS zu arbeiten.",
      ],
      result:
        "Alle Betreuungskräfte arbeiten ohne feste Gruppenzuordnung mit der gemeinsamen Kinderliste.",
      related: [HELP_TOPICS.ownGroups, HELP_TOPICS.studentSearch],
    };
  }
  if (groupMode === "unknown") {
    return {
      id: HELP_TOPICS.transferGroup,
      title: "Meine Gruppe vorübergehend übergeben",
      question: "Wie übergebe ich meine Gruppe vorübergehend?",
      summary: "Eine Übergabe ist nur bei festen Gruppen möglich.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "ArrowsLeftRight",
      steps: [
        "Prüfen Sie, ob `Gruppen` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Öffnen Sie Ihre Gruppe und das Menü mit den drei Punkten.",
        "Fehlt der Bereich? Dann müssen Sie keine feste Gruppe übergeben.",
      ],
      result: "Eine andere Betreuungskraft erhält Zugriff bis zum Tagesende.",
      related: [HELP_TOPICS.ownGroups, HELP_TOPICS.missingChildOrGroup],
    };
  }

  return {
    id: HELP_TOPICS.transferGroup,
    title: "Meine Gruppe vorübergehend übergeben",
    question: "Wie übergebe ich meine Gruppe vorübergehend?",
    summary:
      "Geben Sie einer anderen Betreuungskraft bis zum Tagesende Zugriff auf Ihre Gruppe.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "ArrowsLeftRight",
    requirements: ["Sie sind dieser Gruppe fest zugeordnet."],
    steps: [],
    instructionGroups: [
      {
        title: "Gruppe übergeben",
        steps: [
          "Öffnen Sie Ihre Gruppe unter `Gruppen`.",
          "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
          "Wählen Sie `Gruppe übergeben`.",
          "Wählen Sie unter `Übergeben an:` im Feld `Fachkraft auswählen...` die Person aus.",
          "Wählen Sie `Übergeben`.",
        ],
      },
      {
        title: "Eine Übergabe früher beenden",
        description:
          "Eine Übergabe gilt nur für heute. Sie können sie auch vorher beenden.",
        steps: [
          "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
          "Wählen Sie `Gruppe übergeben`.",
          "Wählen Sie unter `Aktuell übergeben an:` neben der Person `Zurücknehmen`.",
          "Bestätigen Sie mit `Übergabe zurücknehmen`.",
        ],
      },
    ],
    result:
      "Die andere Betreuungskraft sieht die Gruppe bis zum Ende des Tages unter `Gruppen`, markiert mit `Vertretung`. Sie behalten selbst den Zugriff.",
    notes: [
      "Sie können die Gruppe an mehrere Betreuungskräfte übergeben. Wiederholen Sie dafür die Schritte.",
      "Alle Übergaben stehen im Fenster unter `Aktuell übergeben an:`.",
    ],
    troubleshootingDetails: [
      "Fehlt der Eintrag `Gruppe übergeben` im Menü? Dann betreuen Sie die Gruppe nur als Vertretung. Eine Vertretung kann die Gruppe nicht weitergeben.",
      "Steht dort `Keine pädagogische Fachkraft verfügbar`? Dann gibt es niemanden zum Übergeben. Fragen Sie Ihre Leitung.",
      // Einstellung „Kinder nur über ‚Alle Kinder‘ finden“ (#1321).
      "Fehlt `Gruppen` in der Navigation? Öffnen Sie `Alle Kinder`. Wählen Sie im Menü mit den drei Punkten `Zur Gruppenseite`.",
    ],
    related: [HELP_TOPICS.ownGroups, HELP_TOPICS.missingChildOrGroup],
  };
}
