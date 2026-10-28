import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { parentAnnouncementSteps } from "../help-article-shared";

export function parentAnnouncementTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadParentAnnouncement,
    title: "Eine Elternmitteilung veröffentlichen",
    question: "Wie erreiche ich alle Eltern auf einmal?",
    summary:
      "Eine Mitteilung erscheint im Elternportal der gewählten Familien.",
    group: "elternarbeit",
    audience: "lead",
    icon: "Megaphone",
    steps: [
      ...parentAnnouncementSteps("Mitteilungen", "Mitteilung", "Titel"),
      "Wählen Sie `Veröffentlichen`.",
    ],
    result:
      "Die Mitteilung steht sofort im Elternportal der erreichten Eltern.",
    notes: [
      "Mit `Als Entwurf speichern` legen Sie die Mitteilung zurück, ohne sie zu senden.",
      "Sie können mehrere Empfänger kombinieren: `Ganze Schule`, `Offene Anmeldungen`, `Klassen`, `Gruppen`, `AGs / Betreuung` oder einzelne Kinder.",
      "Jedes Elternteil bekommt die Mitteilung höchstens einmal.",
      "Mit `Dateien anhängen` geben Sie Dateien mit. Sie liegen im Elternportal und gehen nicht per E-Mail mit.",
    ],
    differences: [
      "Eine Mitteilung trägt `Entwurf`, `Veröffentlicht` oder `Abgelaufen`.",
      "Unter einer Mitteilung steht `Von …, Lehrkraft der Klasse … · nur zum Lesen`? Die hat eine Lehrkraft in moto schule geschrieben. Sie können sie lesen, ändern kann sie nur die Lehrkraft.",
      "Unter `Priorität` stehen `Info` und `Wichtig` zur Wahl.",
      "Mit `Ablaufdatum (optional)` blenden Sie die Mitteilung später wieder aus.",
    ],
    troubleshootingDetails: [
      "`Veröffentlichen` bleibt grau? Dann fehlt noch ein Empfänger.",
      "Soll die Nachricht auch per E-Mail gehen? Schalten Sie `Eltern zusätzlich per E-Mail benachrichtigen` ein.",
      "Sollen Eltern den Erhalt bestätigen? Schalten Sie `Lesebestätigung erforderlich` ein.",
    ],
    related: [
      HELP_TOPICS.leadParentLetter,
      HELP_TOPICS.leadParentSurvey,
      HELP_TOPICS.parentMessage,
    ],
  };
}
