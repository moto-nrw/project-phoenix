import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/vertraege` (#4678): Vertragsbedingungen je Angebot festlegen,
 * Entwürfe aus angenommenen Anmeldungen erstellen, prüfen und senden.
 */
export function traegerCareContractsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareContracts,
    title: "Betreuungsverträge erstellen und senden",
    question: "Wie schicke ich den Eltern den Betreuungsvertrag?",
    summary:
      "Aus jeder angenommenen Anmeldung erstellt moto einen Vertragsentwurf. Sie prüfen ihn und senden ihn per Mail und in die Eltern-App.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Signature",
    requirements: [
      "Sie haben die Rolle `Verwaltung`. Sie gibt das Recht `Verträge verwalten`.",
      "Die Schule hat die Anmeldung angenommen, und das Kind hat ein Angebot gebucht.",
    ],
    instructionGroups: [
      {
        title: "Einmal: Vertragsbedingungen festlegen",
        steps: [
          "Wählen Sie in der Navigation `Verträge`.",
          "Wählen Sie oben den Reiter `Vertragsbedingungen`.",
          "Wählen Sie links ein Angebot. `Fehlt` heißt: Für dieses Angebot gibt es noch keine Bedingungen.",
          "Wählen Sie die `Laufzeit`, die `Frist in Monaten` und wann die `Kündigung wirkt`.",
          "Erlauben Sie bei Bedarf eine `Sonderkündigung`. Schreiben Sie dazu, wann sie möglich ist.",
          "Müssen die Eltern eine Satzung anerkennen? Setzen Sie den Haken bei `Satzung`. Tragen Sie den Namen und, wenn vorhanden, den Link ein.",
          "Wählen Sie `Bedingungen speichern`.",
        ],
      },
      {
        title: "Entwürfe erstellen",
        steps: [
          "Wählen Sie den Reiter `Verträge`.",
          "Wählen Sie `Verträge erstellen`. Die Zahl dahinter nennt die Kinder ohne Vertrag.",
          "Prüfen Sie die Liste. moto übernimmt Kind, Eltern, Tage und Beginn aus der Anmeldung.",
          "Wählen Sie die Kinder aus. Fehlen bei einem Angebot die Bedingungen, legen Sie sie zuerst fest.",
          "Wählen Sie `Entwürfe erstellen`.",
        ],
      },
      {
        title: "Prüfen und senden",
        steps: [
          "Wählen Sie in der Liste ein Kind. Die Seite zeigt die `Vertragsangaben` und die `Vorschau` des PDF.",
          "Prüfen Sie Tage, Beginn und Beitrag.",
          "Wählen Sie `Vertrag senden`.",
          "Mehrere Entwürfe auf einmal: Setzen Sie in der Liste die Haken und wählen Sie oben `Entwürfe senden`.",
        ],
      },
    ],
    steps: [],
    result:
      "Die Eltern bekommen den Vertrag als PDF per Mail. Haben sie ein Konto in der Eltern-App, liegt er dort auch unter `Dokumente`. Der Status wird `Versendet`.",
    notes: [
      "Beim ersten Senden hält moto das PDF fest. Ändern Sie danach Bedingungen oder Brieftext, bleibt der versendete Vertrag gleich.",
      "Unter `Versand` sehen Sie, ob die Mail angekommen ist. Mit `Erneut senden` geht dasselbe PDF noch einmal an die Eltern.",
      "Den Brieftext des Vertrags ändern Sie unter `Einstellungen` → `Briefkopf und Vorlagen` bei der Vorlage `Vertrag`.",
      "Mit `PDF herunterladen` bekommen Sie den Vertrag zum Drucken, etwa für Familien ohne Mail.",
    ],
    differences: [
      "Sie sehen `Verträge` nicht in der Navigation? Ihnen fehlt das Recht `Verträge verwalten`. Wenden Sie sich an eine Person mit der Rolle `Leitung`.",
    ],
    troubleshootingDetails: [
      "Ein Kind fehlt in `Verträge erstellen`? Die Schule hat die Anmeldung noch nicht angenommen, oder das Kind hat schon einen Vertrag für dieses Angebot.",
      "Ein Entwurf ist falsch? Wählen Sie auf seiner Seite oben `Entwurf verwerfen`. Danach erstellen Sie ihn neu. Einen versendeten Vertrag können Sie nicht verwerfen.",
      "Der Versand zeigt `Nicht angekommen`? Die Mail-Adresse stimmt vielleicht nicht. Klären Sie sie mit der Schule und senden Sie erneut.",
    ],
    related: [
      HELP_TOPICS.traegerDocumentTemplates,
      HELP_TOPICS.traegerBriefkopf,
      HELP_TOPICS.traegerAccounts,
    ],
  };
}
