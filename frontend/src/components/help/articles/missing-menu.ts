import type { HelpTopic, HelpPresenceMode, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";
import { missingMenuDifferences } from "../help-article-shared";

export function missingMenuTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  return {
    id: HELP_TOPICS.missingMenu,
    title: "Ein Menüpunkt fehlt",
    question: "Was kann ich tun, wenn ein Menüpunkt fehlt?",
    summary:
      "Ein Bereich kann wegen Ihrer Rolle, einer Einstellung oder der gewählten Arbeitsweise der OGS fehlen.",
    group: "probleme",
    audience: "caregiver",
    icon: "EyeSlash",
    steps: [
      "Öffnen Sie auf dem Handy unten `Mehr`. Dort stehen alle Bereiche.",
      "Klappen Sie am Computer in der Seitenleiste die passende Gruppe auf.",
      "Laden Sie die Seite neu.",
      "Melden Sie sich ab und wieder an, wenn Ihre Rolle oder Rechte gerade geändert wurden. `Abmelden` steht am Computer im Menü oben rechts, auf dem Handy unter `Mehr`.",
      "Fragen Sie Ihre Leitung, ob die Funktion für Ihre OGS eingeschaltet ist.",
      "Bitten Sie Ihre Leitung, Ihre Rolle und Rechte zu prüfen.",
    ],
    result:
      "Ihre Leitung prüft die Einstellungen und Ihren Zugang. Danach kann sie die Ursache klären.",
    differences: missingMenuDifferences(presenceMode, groupMode, nfcEnabled),
    related: [HELP_TOPICS.missingChildOrGroup, HELP_TOPICS.loginProblem],
  };
}
