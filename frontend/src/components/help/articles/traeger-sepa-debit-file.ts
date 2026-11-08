import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerSepaDebitFileTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerSepaDebitFile,
    title: "Lastschriftdatei bei der Bank einreichen",
    question: "Wie reiche ich die Lastschriftdatei bei meiner Bank ein?",
    summary:
      "Für jeden freigegebenen Lauf laden Sie eine Datei für Ihre Bank herunter. Sie laden die Datei in Ihrem Online-Banking hoch. Danach bestätigen Sie es in moto. moto bucht kein Geld ab, das macht Ihre Bank.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "ArrowUpRight",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung.",
      "Der Monatslauf oder Ferienlauf ist freigegeben.",
      "Die Familien haben ein gültiges SEPA-Mandat.",
      "Unter `Einstellungen` → `Abrechnung` gibt es eine Bankverbindung als Standard. Auf dieses Konto gehen die Beiträge.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Datei herunterladen",
        steps: [
          "Öffnen Sie unter `Abrechnung` den freigegebenen Lauf.",
          "Prüfen Sie in der Karte `Lastschrift` Zahl, Summe und Tag der Abbuchung.",
          "Wählen Sie `Datei herunterladen`. Sie bekommen eine XML-Datei.",
          "Lassen Sie diese Seite offen, bis Sie das Einreichen bestätigt haben.",
        ],
      },
      {
        title: "Datei bei der Bank einreichen",
        description:
          "So geht es in den meisten Online-Bankings. Die Wörter können bei Ihrer Bank etwas anders heißen.",
        steps: [
          "Melden Sie sich im Online-Banking Ihres Vereins an.",
          "Suchen Sie `Datei hochladen`, `Sammellastschrift` oder `SEPA-Datei einreichen`.",
          "Wählen Sie die heruntergeladene Datei aus.",
          "Prüfen Sie Zahl und Summe. Sie müssen zur Karte `Lastschrift` passen.",
          "Geben Sie den Auftrag frei, zum Beispiel mit TAN.",
        ],
      },
      {
        title: "In moto bestätigen",
        description:
          "Bestätigen Sie erst, wenn Ihre Bank die Datei angenommen hat.",
        steps: [
          "Wählen Sie in der Karte `Lastschrift` den Knopf `Bei der Bank eingereicht`.",
          "Wählen Sie `Ja, eingereicht`.",
        ],
      },
    ],
    result:
      "Die Karte zeigt `Bei der Bank eingereicht`. Bei einem neuen Mandat ist die nächste Abbuchung eine Folgelastschrift. Auf der Seite des Mandats steht jede Abbuchung unter `Abbuchungen`.",
    notes: [
      "Sie können die Datei mehrmals herunterladen. Ohne Änderungen am selben Tag bleibt sie gleich. Reichen Sie dieselbe Datei aber nur einmal ein.",
      "Eine Lastschrift, die schon fällig war, bucht die Datei am nächsten Tag ab.",
      "Ein Einmalmandat darf nur eine Rechnung einziehen. Bei mehreren Rechnungen klären Sie mit der Familie, wie sie zahlt.",
      "Ein Mandat gilt 36 Monate nach der letzten Abbuchung. Danach braucht die Familie ein neues Mandat.",
    ],
    differences: [
      "Die Karte `Lastschrift` fehlt? In diesem Lauf zahlt keine Familie per Lastschrift.",
      "Die Karte `Nicht in der Datei` zeigt Familien? Für sie darf die Bank nicht abbuchen. Der Grund steht dabei. Prüfen Sie das Mandat mit `Mandat prüfen`.",
    ],
    troubleshootingDetails: [
      "`Datei herunterladen` geht nicht? Es fehlt die Bankverbindung als Standard. Wer das Recht `Einstellungen` hat, legt sie fest.",
      "moto meldet, die Datei hat sich geändert? Seit dem Herunterladen wurde eine Rechnung storniert oder ein Mandat geändert. Laden Sie die Datei neu herunter und reichen Sie die neue ein.",
      "Ihre Bank lehnt die Datei ab? Bestätigen Sie in moto nichts. Fragen Sie Ihre Bank nach dem Grund.",
    ],
    related: [HELP_TOPICS.traegerSepaMandates, HELP_TOPICS.traegerCareMonths],
  };
}
