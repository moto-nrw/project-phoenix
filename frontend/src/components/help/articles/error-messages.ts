import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function errorMessagesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.errorMessages,
    title: "Fehlermeldungen verstehen",
    question: "Was kann ich bei einer Fehlermeldung tun?",
    summary:
      "Hier finden Sie den nächsten Schritt, wenn moto einen Fehler meldet.",
    group: "probleme",
    audience: "all",
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      {
        title: "Eingabe prüfen",
        steps: [
          "Prüfen Sie die markierten Felder.",
          "Verbessern Sie die Angaben und versuchen Sie es erneut.",
        ],
      },
      {
        title: "Zugriff prüfen",
        steps: [
          "Melden Sie sich erneut an, falls moto Sie dazu auffordert.",
          "Fehlt Ihnen weiterhin der Zugriff? Fragen Sie Ihre Ansprechperson.",
        ],
      },
      {
        title: "Vorgang nicht möglich",
        steps: [
          "Lesen Sie, warum moto den Vorgang nicht ausführen kann.",
          "Ändern Sie die Angaben oder fragen Sie Ihre Ansprechperson.",
        ],
      },
      {
        title: "Gerade nicht erreichbar",
        steps: [
          "Prüfen Sie Ihre Internetverbindung.",
          "Tippen Sie auf `Wiederholen`, wenn der Knopf zu sehen ist.",
        ],
      },
      {
        title: "Unerwarteter Fehler",
        steps: [
          "Tippen Sie auf `Wiederholen`, wenn der Knopf zu sehen ist.",
          "Klappt es weiter nicht? Tippen Sie auf die Vorgangskennung und nennen Sie sie dem moto-Team.",
        ],
      },
    ],
    related: [],
  };
}
