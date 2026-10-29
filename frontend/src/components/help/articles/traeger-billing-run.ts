import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerBillingRunTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerBillingRun,
    title: "Ferien abrechnen",
    question: "Wie stelle ich die Rechnungen für die Ferien aus?",
    summary:
      "Ein Abrechnungslauf erstellt die Rechnungen für einen Ferienzeitraum. Sie prüfen die Vorschau, korrigieren einzelne Beträge, geben den Lauf frei und erstellen die Schreiben an die Familien.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "FileText",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung. Nur diese Rolle sieht in der Navigation `Abrechnung`.",
      "Der Ferienzeitraum ist veröffentlicht, und die Anmeldungen sind bestätigt.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Rechnungsnummern festlegen",
        description: "Das machen Sie einmal vor der ersten Freigabe.",
        steps: [
          "Wählen Sie in der Navigation `Abrechnung`.",
          "Wählen Sie bei `Rechnungsnummern` den Knopf `Muster festlegen`.",
          "Tragen Sie das Muster ein, zum Beispiel FE-{JJJJ}-{NNNN}. {NNNN} ist die laufende Nummer, {JJJJ} das Jahr.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Lauf starten und prüfen",
        steps: [
          "Wählen Sie `Lauf starten`.",
          "Wählen Sie den Ferienzeitraum und dann `Lauf starten`.",
          "Prüfen Sie die Vorschau unter `Rechnungen`. Je Familie steht dort eine Rechnung.",
          "Die Positionen einer Rechnung sehen Sie im Menü mit den drei Punkten unter `Aufschlüsselung ansehen`.",
        ],
      },
      {
        title: "Einen Betrag korrigieren",
        steps: [
          "Wählen Sie bei der Rechnung das Menü mit den drei Punkten.",
          "Wählen Sie `Betrag für … korrigieren`.",
          "Tragen Sie den neuen Betrag und eine Begründung ein.",
          "Wählen Sie `Korrigieren`.",
        ],
      },
      {
        title: "Lauf freigeben",
        steps: [
          "Wählen Sie `Lauf freigeben`.",
          "Prüfen Sie Zahl und Summe der Rechnungen und wählen Sie `Freigeben`.",
        ],
      },
      {
        title: "Bestätigungsschreiben erstellen",
        description:
          "Jede Familie bekommt ein Schreiben im Briefkopf des Trägers. Es bestätigt die Anmeldung und ist zugleich die Rechnung.",
        steps: [
          "Wählen Sie im freigegebenen Lauf bei `Bestätigungsschreiben` den Knopf `Schreiben erstellen`.",
          "Tragen Sie bei `Zahlen bis` das Datum ein, bis wann die Familien zahlen sollen.",
          "Schreiben Sie bei `Hinweise für die Eltern` zum Beispiel die Packliste und was bei Krankheit gilt.",
          "Tragen Sie bei `Notfallnummern der Betreuung` je Betreuung den Namen und die Telefonnummer ein.",
          "Wählen Sie `Vorschau ansehen`. Sie sehen das Schreiben der ersten Familie.",
          "Wählen Sie `Schreiben erstellen`.",
          "Das Schreiben einer Familie laden Sie über das Menü mit den drei Punkten bei ihrer Rechnung herunter: `Schreiben herunterladen`.",
        ],
      },
      {
        title: "Schreiben per Mail senden und drucken",
        description:
          "Jede Familie bekommt ihr Schreiben als PDF im Anhang einer Mail. Hat die Familie ein Konto in der Eltern-App mit derselben Mail-Adresse, liegt es auch dort unter `Dokumente`. Für Familien ohne Internet drucken Sie es.",
        steps: [
          "Wählen Sie im freigegebenen Lauf bei `Versand per Mail` den Knopf `An alle Familien senden`. moto schickt nur Mails, die noch nicht gesendet sind.",
          "Neben jeder Familie steht, ob die Mail gesendet ist. `Nicht angekommen` heißt: Die Mail kam nicht an.",
          "Wählen Sie bei dieser Familie `Erneut senden`. Mit `Senden` schicken Sie das Schreiben an eine einzelne Familie.",
          "Wählen Sie `Alle Schreiben drucken`. Sie bekommen alle Schreiben des Laufs in einer Datei.",
        ],
      },
      {
        title: "Übersicht für die Kämmerei herunterladen",
        description: "Das geht, sobald der Lauf freigegeben ist.",
        steps: [
          "Öffnen Sie den freigegebenen Lauf.",
          "Wählen Sie oben in der Karte `Rechnungen` das Menü mit den drei Punkten.",
          "Wählen Sie unter `Exportieren` das Format: `PDF`, `Excel` oder `CSV`.",
          "Geben Sie die Datei an die Kämmerei weiter.",
        ],
      },
    ],
    result:
      "Jede Rechnung hat ihre Nummer. Die Nummern sind lückenlos. Ausgestellte Rechnungen und ihre Schreiben lassen sich nicht mehr ändern. Die Übersicht zeigt jede Position je Familie, die Summe jeder Rechnung und die Gesamtsumme.",
    notes: [
      "Ferien werden im Voraus abgerechnet. Grundlage ist der Beitrag, den moto beim Bestätigen der Anmeldung festgehalten hat.",
      "Jede Korrektur steht als eigene Position auf der Rechnung. Unter der Rechnung sehen Sie alle Korrekturen mit Datum und Begründung.",
      "Wird nach dem Start noch eine Anmeldung bestätigt, kommt sie bis zur Freigabe in den Lauf. Nach der Freigabe starten Sie für sie einen neuen Lauf.",
      "Logo, Farbe, Absender, Fußzeile, Ansprechperson und Bankverbindung im Schreiben kommen aus dem `Briefkopf`. Als Verwendungszweck steht die Rechnungsnummer im Schreiben.",
    ],
    differences: [
      "Sie sehen in der Navigation kein `Abrechnung`? Ihre Rolle ist nicht Abrechnung. Fragen Sie im Träger-Büro nach dieser Rolle.",
      "`Lauf freigeben` lässt sich nicht wählen? Es fehlt das Muster der Rechnungsnummern, oder der Lauf hat keine Rechnung.",
      "Sie sehen oben in der Karte `Rechnungen` kein Menü mit den drei Punkten? Der Lauf ist noch nicht freigegeben.",
    ],
    troubleshootingDetails: [
      "Die Freigabe meldet, dass sich der Lauf geändert hat? Inzwischen wurde eine Anmeldung bestätigt. Prüfen Sie die neue Vorschau und geben Sie dann frei.",
      "Eine bestätigte Anmeldung fehlt in der Vorschau? Sie hat schon eine Rechnung, oder beim Bestätigen wurde kein Beitrag festgehalten.",
      "Die Schreiben lassen sich nicht erstellen, weil eine Bankverbindung fehlt? Die Verwaltung trägt sie unter `Briefkopf` ein. Danach erstellen Sie die Schreiben.",
      "Eine Mail kommt auch nach `Erneut senden` nicht an? Vielleicht stimmt die Adresse nicht. Drucken Sie das Schreiben und geben Sie es der Familie.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayContribution,
      HELP_TOPICS.traegerHolidayRegistrationReview,
    ],
  };
}
