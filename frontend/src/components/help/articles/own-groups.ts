import type { HelpTopic, HelpPresenceMode, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function ownGroupsTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): HelpTopic {
  if (groupMode === "open_care") {
    return {
      id: HELP_TOPICS.ownGroups,
      title: "Gruppen ansehen",
      question: "Wie sehe ich meine Gruppen an?",
      summary:
        "Bei offener Betreuung gibt es keine festen eigenen Gruppen in moto.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "UsersThree",
      steps: [
        "Öffnen Sie `Alle Kinder`.",
        "Nutzen Sie die Suche oder die Filter für die benötigten Kinder.",
      ],
      result:
        "Sie sehen die Kinder der OGS gemeinsam. Der Bereich `Gruppen` wird nicht angezeigt.",
      related: [HELP_TOPICS.studentSearch, HELP_TOPICS.webAttendance],
    };
  }
  if (groupMode === "unknown") {
    return {
      id: HELP_TOPICS.ownGroups,
      title: "Gruppen ansehen",
      question: "Wie sehe ich meine Gruppen an?",
      summary:
        "Ob Sie eigene Gruppen sehen, hängt von der Arbeitsweise Ihrer OGS ab.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "UsersThree",
      steps: [
        "Prüfen Sie, ob `Gruppen` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Wählen Sie dort die gewünschte Gruppe.",
        "Fehlt der Bereich? Öffnen Sie `Alle Kinder` und nutzen Sie Suche und Filter.",
      ],
      result:
        "Sie sehen die Kinder entweder in festen eigenen Gruppen oder in einer gemeinsamen Kinderliste.",
      related: [HELP_TOPICS.studentSearch, HELP_TOPICS.missingChildOrGroup],
    };
  }

  const statusResult =
    presenceMode === "detailed"
      ? "Die Karten zeigen die geplanten Zeiten und den aktuellen Aufenthaltsort der Kinder."
      : "Die Karten zeigen die geplanten Zeiten und die Anwesenheit der Kinder.";

  return {
    id: HELP_TOPICS.ownGroups,
    title: "Gruppen ansehen",
    question: "Wie sehe ich meine Gruppen an?",
    summary: "Wählen Sie eine Gruppe und sehen Sie ihre Kinder.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "UsersThree",
    steps: [
      "Öffnen Sie `Gruppen` in der Seitenleiste.",
      "Wählen Sie die gewünschte Gruppe. Ihre eigene Gruppe ist mit `Meine Gruppe` markiert.",
      "Prüfen Sie die Zahlen für `krank` und `entschuldigt`.",
      "Suchen Sie bei Bedarf über `Name suchen...` nach einem Kind.",
      // Live geprueft: die Seite hat keinen Knopf `Filter`, sondern einen
      // Umschalter fuer die Reihenfolge und eine Auswahl fuer den Ort.
      "Ordnen Sie die Liste nach `Alphabetisch`, `Nächste Ankunft` oder `Nächste Abholung`.",
      "Grenzen Sie über `Alle Orte` auf einen Aufenthaltsort ein.",
      "Wählen Sie die Karte eines Kindes, um seine Angaben zu öffnen.",
    ],
    result: statusResult,
    differences: [
      "Sehen Sie nur eine Gruppe? Dann öffnet moto sie direkt, ohne Auswahl.",
      "Ist für eine Gruppe keine Gruppenleitung eingetragen, fehlt diese Zeile auf ihrer Karte.",
      "Eine vorübergehend übernommene Gruppe ist mit `Vertretung` markiert.",
      "In der Seitenleiste stehen unter `Gruppen` nur Ihre eigenen Gruppen. Dahinter steht, wie viele Kinder gerade da sind.",
      "Fehlt eine Gruppe? Bitten Sie Ihre Leitung, Ihre Gruppenzuordnung zu prüfen.",
      // Einstellung „Kinder nur über ‚Alle Kinder‘ finden“ (#1321).
      "Fehlt `Gruppen` in der Navigation? Öffnen Sie `Alle Kinder` und wählen Sie oben `Meine Gruppe`. Die Gruppenseite öffnen Sie im Menü mit den drei Punkten unter `Zur Gruppenseite`.",
    ],
    troubleshooting: HELP_TOPICS.missingChildOrGroup,
    related: [
      HELP_TOPICS.transferGroup,
      HELP_TOPICS.activeSupervision,
      HELP_TOPICS.studentSearch,
    ],
  };
}
