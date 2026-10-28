import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/briefkopf` (#4653): Logo, Absender und Bankverbindungen. */
export function traegerBriefkopfTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerBriefkopf,
    title: "Briefkopf und Bankverbindungen hinterlegen",
    question: "Wie sehen die Briefe meines Trägers aus?",
    summary:
      "Logo, Absender, Farbe, Fußzeile, Ansprechpartner und Bankverbindungen stehen auf allen Briefen und Rechnungen Ihres Trägers.",
    group: "konfiguration",
    audience: "traeger",
    icon: "FileText",
    requirements: ["Sie dürfen im Träger-Büro Einstellungen ändern."],
    instructionGroups: [
      {
        title: "Logo hochladen",
        steps: [
          "Wählen Sie in der Navigation `Briefkopf`.",
          "Wählen Sie bei `Logo` den Knopf `Logo hochladen`.",
          "Wählen Sie ein Bild im Format PNG oder JPG, bis 1 MB.",
        ],
      },
      {
        title: "Absender, Farbe und Fußzeile eintragen",
        steps: [
          "Tragen Sie bei `Absender im Briefkopf` Name und Anschrift ein, jede Angabe in eine eigene Zeile.",
          "Klicken Sie bei `Farbe für Briefe und Rechnungen` auf den farbigen Kreis und wählen Sie eine Farbe. Oder tragen Sie den Farbcode Ihres Trägers in das Feld daneben ein: eine Raute und sechs Zeichen, so wie er im Logo-Entwurf steht.",
          "Tragen Sie bei `Fußzeile` zum Beispiel Vorstand, Amtsgericht und Steuernummer ein.",
          "moto speichert jedes Feld von selbst.",
        ],
      },
      {
        title: "Ansprechpartner eintragen",
        steps: [
          "Tragen Sie bei `Ansprechpartner` Name, Telefon und E-Mail-Adresse ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Bankverbindung hinzufügen",
        steps: [
          "Wählen Sie bei `Bankverbindungen` den Knopf `Bankverbindung hinzufügen`.",
          "Geben Sie eine Bezeichnung ein, zum Beispiel Elternbeiträge.",
          "Geben Sie Kontoinhaber und IBAN ein. BIC und Bank sind freiwillig.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    steps: [],
    result:
      "Die `Vorschau` oben auf der Seite zeigt, wie Kopf und Fuß eines Briefs aussehen.",
    notes: [
      "Eine Bankverbindung ist der `Standard`. Sie steht auf allen Briefen und Rechnungen.",
      "Eine andere Bankverbindung machen Sie im Menü der Zeile mit `Als Standard festlegen` zum Standard.",
      "Löschen Sie den Standard, wird die älteste andere Bankverbindung der Standard.",
      "Jedes Ansehen und jede Änderung von Ansprechpartner und Bankverbindungen steht im Protokoll des Trägers.",
    ],
    differences: [
      "Sie können die Angaben nur ansehen? Ihnen fehlt das Recht `Einstellungen`. Wenden Sie sich an eine Person mit der Rolle `Verwaltung`.",
    ],
    troubleshootingDetails: [
      "Das Logo wird nicht übernommen? Es muss ein PNG- oder JPG-Bild sein, höchstens 1 MB groß und höchstens 2000 Pixel breit und hoch.",
      "Die IBAN wird abgelehnt? Prüfen Sie jede Ziffer. moto prüft die Prüfziffern der IBAN.",
    ],
    related: [HELP_TOPICS.traegerSettings, HELP_TOPICS.errorMessages],
  };
}
