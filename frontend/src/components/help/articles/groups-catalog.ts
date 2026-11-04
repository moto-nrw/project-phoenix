import type { HelpTopic, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Gruppenkatalog unter `/database/groups`. An keine Einstellung gebunden: der
 * `OpenCareModeGuard` schuetzt nur `/ogs-groups`, und die Seitenleiste blendet
 * bei offener Betreuung nur das Akkordeon `Gruppen` aus
 * (sidebar.tsx:1391). Der Katalog selbst bleibt in jeder Arbeitsweise stehen.
 */
export function groupsCatalogTopic(groupMode: HelpGroupMode): HelpTopic {
  const openCare = groupMode === "open_care";
  return {
    id: HELP_TOPICS.leadGroups,
    title: "Gruppen anlegen",
    question: "Wie lege ich die Gruppen der OGS an?",
    summary: openCare
      ? "Legen Sie Gruppen an, wenn Sie Kinder darin ordnen möchten."
      : "Legen Sie jede Gruppe an und geben Sie ihr Raum und Leitung.",
    group: "einrichten",
    audience: "lead",
    icon: "UsersThree",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Gruppe anlegen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Gruppendaten`.",
          "Wählen Sie oben rechts `+ Gruppe`.",
          "Tragen Sie bei `Gruppenname` den Namen der Gruppe ein.",
          "Wählen Sie bei Bedarf einen `Gruppenraum`.",
          "Wählen Sie bei Bedarf eine `Gruppenleitung`. Mehrere sind möglich.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Eine Gruppe ändern",
        steps: [
          "Öffnen Sie `Gruppendaten` und wählen Sie die Gruppe aus der Liste.",
          "Ändern Sie die Angaben unter `Stammdaten`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Eine Gruppe löschen",
        steps: [
          "Öffnen Sie `Gruppendaten` und wählen Sie die Gruppe aus der Liste.",
          "Wählen Sie oben rechts `Löschen`.",
          "Bestätigen Sie mit `Ja, löschen`.",
        ],
      },
    ],
    result:
      "Die Gruppe steht sofort zur Auswahl. Sie können Kinder darin eintragen.",
    notes: [
      "Ein Kind kommt über sein Feld `OGS Gruppe` in die Gruppe, nicht hier.",
      "Über `Alle Räume` filtern Sie die Liste auf die Gruppen eines Raums.",
      ...(openCare
        ? [
            "Ihre OGS arbeitet ohne feste Gruppen. Ihr Team sieht deshalb den Bereich `Gruppen` nicht.",
          ]
        : []),
    ],
    differences: [
      "Auf dem Handy steht statt `+ Gruppe` ein rundes Plus unten rechts.",
      "Bei `Gruppenraum` steht `Kein Gruppenraum`, solange kein Raum gewählt ist.",
    ],
    troubleshootingDetails: [
      "Fehlt `Datenverwaltung` in der Seitenleiste? Dann fehlen Ihnen die Leitungsrechte. Fragen Sie Ihre Leitung.",
      "Fehlt eine Person bei `Gruppenleitung`? Dann ist sie noch nicht als Betreuungskraft angelegt.",
    ],
    related: [
      HELP_TOPICS.leadRooms,
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadCreateStudent,
    ],
  };
}
