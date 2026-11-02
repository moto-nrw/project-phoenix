import type { HelpTopic, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function missingChildOrGroupTopic(groupMode: HelpGroupMode): HelpTopic {
  return {
    id: HELP_TOPICS.missingChildOrGroup,
    title: "Ein Kind oder eine Gruppe fehlt",
    question: "Was kann ich tun, wenn ein Kind oder eine Gruppe fehlt?",
    summary:
      "Prüfen Sie Suche, Filter, gewählten Tag und Ihre Gruppenzuordnung.",
    group: "probleme",
    audience: "caregiver",
    icon: "Search",
    steps: [
      "Löschen Sie den Text im Suchfeld.",
      // Jede Liste hat eigene Filter: ein Symbol, ein Umschalter oder eine
      // Auswahl. Einen Knopf namens `Filter` gibt es nirgends.
      "Setzen Sie die Filter über der Liste zurück.",
      "Prüfen Sie, ob oben der richtige Tag gewählt ist.",
      "Suchen Sie das Kind unter `Alle Kinder`.",
      ...(groupMode === "fixed_groups"
        ? [
            "Fehlt das Kind nur unter `Gruppen`? Prüfen Sie, ob Sie die richtige Gruppe geöffnet haben.",
            "Bitten Sie Ihre Leitung, die Gruppe des Kindes und Ihre Gruppenzuordnung zu prüfen.",
          ]
        : groupMode === "open_care"
          ? [
              "Bei offener Betreuung gibt es keine festen eigenen Gruppen. Arbeiten Sie über `Alle Kinder`.",
            ]
          : [
              "Prüfen Sie, ob Ihre OGS mit festen Gruppen oder einer gemeinsamen Kinderliste arbeitet.",
            ]),
    ],
    result:
      "Das Kind bleibt unsichtbar? Dann muss Ihre Leitung Daten und Zugriff prüfen.",
    differences: [
      "Ein Kind kann an einem anderen Tag anders eingeplant sein.",
      "Vorübergehend übernommene Gruppen gelten nur bis zum Ende des Tages.",
      "Gelöschte oder noch nicht übernommene Kinder erscheinen nicht in der normalen Kinderliste.",
    ],
    related: [HELP_TOPICS.studentSearch, HELP_TOPICS.ownGroups],
  };
}
