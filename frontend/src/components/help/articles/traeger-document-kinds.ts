import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/einstellungen/dokumentarten` (#4688): Rechnung, Gebührenbescheid
 * oder Bestätigung mit Rechnung je Leistung; die Rechtsbehelfsbelehrung des
 * Bescheids schreibt der Träger in der Vorlage „Bescheid“.
 */
export function traegerDocumentKindsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerDocumentKinds,
    title: "Rechnung oder Gebührenbescheid",
    question:
      "Wie lege ich fest, ob Familien eine Rechnung oder einen Bescheid bekommen?",
    summary:
      "Je Leistung wählen Sie das Schreiben: Rechnung, Gebührenbescheid oder bei den Ferien die Bestätigung mit Rechnung. Ein Gebührenbescheid braucht Ihre Rechtsbehelfsbelehrung.",
    group: "konfiguration",
    audience: "traeger",
    icon: "ClipboardList",
    requirements: ["Ihre Rolle im Träger-Büro ist Abrechnung."],
    steps: [],
    instructionGroups: [
      {
        title: "Dokumentart wählen",
        description:
          "Ohne Ihre Wahl bekommen Familien bei den Ferien die Bestätigung mit Rechnung und beim Monatsbeitrag eine Rechnung.",
        steps: [
          "Wählen Sie unten in der Navigation `Einstellungen` und dann `Dokumentarten`.",
          "In der Karte `Ferienbetreuung` oder `Monatsbeitrag` wählen Sie eine Art: `Rechnung`, `Gebührenbescheid` oder `Bestätigung mit Rechnung`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Rechtsbehelfsbelehrung eintragen",
        description:
          "Den Text schreiben Sie selbst. moto gibt keinen Rechtstext vor.",
        steps: [
          "Wählen Sie auf der Seite `Dokumentarten` in der Karte `Rechtsbehelfsbelehrung` den Knopf `Belehrung eintragen`.",
          "Schreiben Sie den Text in das Feld `Rechtsbehelfsbelehrung`. Nennen Sie Frist, Stelle und Form des Widerspruchs.",
          "Wählen Sie `Vorschau zeigen`. Die Belehrung steht unten im Bescheid.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      "Die nächste Freigabe stellt die gewählte Art aus. Im Lauf steht sie bei `Dokumentart`. Schon ausgestellte Schreiben bleiben, wie sie sind.",
    notes: [
      "Ein Gebührenbescheid nennt die Bescheidnummer und das Buchungszeichen der Familie. Das Buchungszeichen ist die Debitorennummer.",
      "Ein Storno eines Gebührenbescheids heißt `Aufhebungsbescheid`. Eine Gutschrift heißt `Änderungsbescheid`. Die Nummern kommen aus demselben Nummernkreis.",
      "Ändern Sie die Dokumentart nach der Prüfung eines Laufs, lehnt moto die Freigabe ab. Laden Sie den Lauf neu und prüfen Sie ihn noch einmal.",
    ],
    troubleshootingDetails: [
      "Im Lauf steht `Rechtsbehelfsbelehrung fehlt` und `Freigeben` geht nicht? Tragen Sie die Belehrung in der Vorlage `Bescheid` ein. Danach können Sie freigeben.",
      "Die Seite `Dokumentarten` fehlt? Ihnen fehlt die Rolle `Abrechnung`.",
    ],
    related: [
      HELP_TOPICS.traegerDocumentTemplates,
      HELP_TOPICS.traegerBillingRun,
      HELP_TOPICS.traegerCareMonths,
    ],
  };
}
