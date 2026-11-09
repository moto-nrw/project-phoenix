import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/einstellungen/briefkopf` (#4653, #5685): Logo, Absender und Fußzeile; dazu Ansprechpartner und Bankverbindungen. */
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
          "Wählen Sie unten in der Navigation `Einstellungen`.",
          "Wählen Sie `Briefkopf und Vorlagen`.",
          "Wählen Sie bei `Logo` den Knopf `Logo hochladen`.",
          "Wählen Sie ein Bild im Format PNG oder JPG, bis 1 MB.",
        ],
      },
      {
        title: "Absender, Farbe und Fußzeile eintragen",
        steps: [
          "Wählen Sie in den Einstellungen `Briefkopf und Vorlagen`.",
          "Tragen Sie bei `Absender im Briefkopf` Name und Anschrift ein. Jede Zeile im Feld ist eine Zeile im Brief. Die erste Zeile steht fett und in Ihrer Farbe.",
          "Klicken Sie bei `Farbe für Briefe und Rechnungen` auf eine der Farben.",
          "Passt keine Farbe, wählen Sie `Eigene Farbe`. Klicken Sie dann auf den farbigen Kreis und wählen Sie Ihre Farbe. Steht im Logo-Entwurf ein Farbcode, tragen Sie ihn in das Feld daneben ein: eine Raute und sechs Zeichen.",
          "Tragen Sie bei `Fußzeile` zum Beispiel Vorstand, Amtsgericht und Steuernummer ein, jede Angabe in eine eigene Zeile.",
          "moto speichert jedes Feld von selbst.",
        ],
      },
      {
        title: "Ansprechpartner eintragen",
        steps: [
          "Wählen Sie in den Einstellungen `Allgemein` und dann den Bereich `Kontakt`.",
          "Tragen Sie bei `Ansprechpartner` Name, Telefon und E-Mail-Adresse ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Bankverbindung hinzufügen",
        steps: [
          "Wählen Sie in den Einstellungen `Abrechnungsvorgaben`.",
          "Wählen Sie bei `Bankverbindungen` den Knopf `Bankverbindung hinzufügen`.",
          "Geben Sie eine Bezeichnung ein, zum Beispiel Elternbeiträge.",
          "Geben Sie Kontoinhaber und IBAN ein. BIC und Bank sind freiwillig.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    steps: [],
    result:
      "Die `Vorschau` oben auf der Seite `Briefkopf und Vorlagen` zeigt, wie Kopf und Fuß eines Briefs aussehen.",
    notes: [
      "Ein leeres Feld für Absender oder Fußzeile zeigt ein graues Beispiel. Das Beispiel steht nie im Brief.",
      "Eine Bankverbindung ist der `Standard`. Sie steht auf allen Briefen und Rechnungen.",
      "Eine andere Bankverbindung machen Sie im Menü der Zeile mit `Als Standard festlegen` zum Standard.",
      "Löschen Sie den Standard, wird die älteste andere Bankverbindung der Standard.",
      "Jedes Ansehen und jede Änderung von Ansprechpartner und Bankverbindungen steht im Protokoll des Trägers.",
    ],
    differences: [
      "Auf dem Handy öffnen Sie in der Leiste unten `Mehr` und dann `Einstellungen`. `Abrechnungsvorgaben` steht dort wieder unter `Mehr`.",
      "Sie können die Angaben nur ansehen? Ihnen fehlt das Recht `Einstellungen`. Wenden Sie sich an eine Person mit der Rolle `Verwaltung`.",
    ],
    troubleshootingDetails: [
      "Das Logo wird nicht übernommen? Es muss ein PNG- oder JPG-Bild sein, höchstens 1 MB groß und höchstens 2000 Pixel breit und hoch.",
      "Die IBAN wird abgelehnt? Prüfen Sie jede Ziffer. moto prüft die Prüfziffern der IBAN.",
    ],
    related: [HELP_TOPICS.traegerSettings, HELP_TOPICS.errorMessages],
  };
}
