import type { HelpTopic, HelpPresenceMode, HelpGroupMode } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Einstiegs-Checkliste. Verweist nur, statt Schritte zu wiederholen
 * (Informationsarchitektur Abschnitt 7). Alle drei Einstellungen wirken hier:
 * ohne NFC entfaellt die Geraetevorbereitung, bei einfacher Anwesenheit gibt
 * es im Testlauf keine Aufsicht, und bei offener Betreuung keine Gruppe.
 */
export function goLiveTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  const usesGroups = groupMode !== "open_care";
  const tracksRooms = presenceMode !== "binary";
  const usesNfc = nfcEnabled !== false;
  return {
    id: HELP_TOPICS.leadGoLive,
    title: "moto für den ersten Betreuungstag vorbereiten",
    question: "Was muss ich vor dem Start erledigen?",
    summary: "Arbeiten Sie diese Liste einmal von oben nach unten ab.",
    group: "einrichten",
    audience: "lead",
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      {
        title: "In dieser Reihenfolge einrichten",
        description:
          "Jeder Punkt hat eine eigene Anleitung. Die Reihenfolge zählt, weil ein Schritt den vorigen braucht.",
        steps: [
          "Legen Sie die `Räume` an.",
          "Legen Sie Ihr Team an und laden Sie es ein.",
          ...(usesGroups
            ? [
                "Legen Sie die `Gruppen` an. Jede Gruppe bekommt einen Raum und eine Gruppenleitung.",
              ]
            : []),
          "Übernehmen Sie die Kinder aus einer Liste. Oder legen Sie sie einzeln an.",
          "Tragen Sie die Betreuungszeiten der Kinder ein.",
          "Tragen Sie unter `Schuljahr und Kalender` Schuljahr, Ferien und Schließtage ein.",
          ...(usesNfc
            ? ["Richten Sie die NFC-Geräte und die Armbänder ein."]
            : []),
          "Prüfen Sie zum Schluss die `Einstellungen` Ihrer OGS.",
        ],
      },
      {
        title: "Vor dem ersten Tag einmal durchspielen",
        steps: [
          "Melden Sie sich mit einem Leitungskonto an.",
          ...(usesGroups
            ? ["Öffnen Sie eine Gruppe und prüfen Sie die zugeordneten Kinder."]
            : ["Prüfen Sie unter `Alle Kinder` drei Kinder."]),
          ...(tracksRooms
            ? ["Starten Sie eine Aufsicht und beenden Sie sie wieder."]
            : ["Melden Sie ein Kind an und wieder ab."]),
          "Melden Sie ein Kind krank und heben Sie die Meldung wieder auf.",
          "Weisen Sie Ihr Team kurz ein.",
        ],
        ordered: false,
      },
    ],
    result:
      "Sind alle Punkte erledigt, kann Ihr Team am ersten Tag ohne Vorbereitung starten.",
    notes: [
      "Sie müssen nicht alles an einem Tag schaffen. Die Liste bleibt gültig.",
      "Laden Sie Ihr Team früh ein. Als `Gruppenleitung` wählbar ist nur, wer die Einladung schon angenommen hat.",
    ],
    troubleshootingDetails: [
      "Ein Punkt fehlt Ihnen in der Seitenleiste? Dann fehlen Ihnen die Leitungsrechte. Fragen Sie Ihre Leitung.",
    ],
    related: [
      HELP_TOPICS.leadFirstSteps,
      HELP_TOPICS.leadRooms,
      ...(usesGroups ? [HELP_TOPICS.leadGroups] : []),
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadCreateStudent,
      HELP_TOPICS.leadCareTimes,
      ...(usesNfc ? [HELP_TOPICS.leadTabletSetup] : []),
      HELP_TOPICS.settings,
    ],
  };
}
