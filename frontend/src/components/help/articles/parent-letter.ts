import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { parentAnnouncementSteps } from "../help-article-shared";

/**
 * Elternbrief: Mitteilung, die zusaetzlich per E-Mail geht und von den Eltern
 * bestaetigt wird (page.tsx:171).
 */
export function parentLetterTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadParentLetter,
    title: "Einen Elternbrief versenden",
    question: "Wie versende ich einen Elternbrief?",
    summary: "Ein Elternbrief geht zusätzlich per E-Mail und wird bestätigt.",
    group: "elternarbeit",
    audience: "lead",
    icon: "FileText",
    steps: [
      ...parentAnnouncementSteps("Elternbriefe", "Elternbrief", "Titel"),
      "Wählen Sie unter `Wer erhält die E-Mail?`, wer sie bekommt.",
      "Wählen Sie `Veröffentlichen`.",
    ],
    result:
      "Der Brief steht im Elternportal. Die erreichten Eltern bekommen zusätzlich eine E-Mail.",
    notes: [
      "Unter `Wer erhält die E-Mail?` wählen Sie `Nur mit Portalzugang` oder `Alle Bezugspersonen`.",
      "Die Eltern bestätigen den Brief. Eine Bestätigung pro Kind genügt.",
      "Der Kasten im Fenster zählt auf, was beim Veröffentlichen automatisch passiert.",
      "Brief nur an bestimmte Kinder, zum Beispiel ohne Fotoeinwilligung? Filtern Sie unter `Alle Kinder` die Liste. Wählen Sie oben rechts im Menü `Elternbrief schreiben`. Der Brief öffnet sich mit genau diesen Kindern als Empfänger.",
      "Die Kinder werden beim Öffnen übernommen. Ändern Sie danach den Filter, bleibt der Brief bei denselben Kindern.",
    ],
    differences: [
      "Nach dem Veröffentlichen lässt sich ein Brief nicht mehr ändern.",
    ],
    troubleshootingDetails: [
      "Reicht eine Nachricht ohne E-Mail und ohne Bestätigung? Nehmen Sie stattdessen eine Mitteilung. Dort sind beides Schalter.",
    ],
    related: [
      HELP_TOPICS.leadParentAnnouncement,
      HELP_TOPICS.leadParentSurvey,
      HELP_TOPICS.leadInviteGuardians,
    ],
  };
}
