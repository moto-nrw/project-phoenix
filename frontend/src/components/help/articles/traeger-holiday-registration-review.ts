import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/ferien/[id]/anmeldungen` (#4663): Anmeldungen prüfen. */
export function traegerHolidayRegistrationReviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayRegistrationReview,
    title: "Ferienanmeldungen prüfen",
    question: "Wie bestätige, storniere und buche ich Ferienanmeldungen um?",
    summary:
      "Sie sehen alle Anmeldungen eines Ferienzeitraums in einer Liste. Dort bestätigen, stornieren und buchen Sie um. Geschwister aus zwei Anmeldungen führen Sie zusammen.",
    image: "/help/screens/ferienanmeldung-zweite-person-traeger.webp",
    imageAlt:
      "Die zweite sorgeberechtigte Person in der Einzelansicht beim Träger.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "ListChecks",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Liste öffnen und filtern",
        steps: [
          "Wählen Sie in der Navigation `Ferien`.",
          "Öffnen Sie den Ferienzeitraum.",
          "Wählen Sie den Reiter `Anmeldungen`.",
          "Filtern Sie nach Status, `Schule`, `Woche` oder `Zeitmodell`. Unter `Schule` finden Sie auch `Feriengäste`.",
          "Wählen Sie eine Anmeldung, um sie zu öffnen.",
        ],
      },
      {
        title: "Bestätigen oder stornieren",
        steps: [
          "Prüfen Sie die Angaben der Eltern und der Kinder. Antworten auf eigene Fragen stehen beim Kind unter `Weitere Angaben`, allgemeine Antworten unter `Allgemeine Angaben`.",
          "Unter `Hinweise und Einwilligungen` sehen Sie, was die Eltern bestätigt haben und wann. Dort stehen die damaligen Texte und PDFs.",
          "Unter `Eltern` stehen `Sorgerecht bestätigt`, `Name als Erklärung` und `Fassung der Texte`. Den genauen Text öffnen Sie unter `Wortlaut der Erklärung`.",
          "Prüfen Sie unter `Ferienbeitrag`, was die Familie zahlt.",
          "Geben die Eltern an, dass sie Leistungen beziehen? Prüfen Sie den Nachweis. Haben die Eltern ihn hochgeladen, wählen Sie unter `Nachweis für den Leistungsbezug` den Dateinamen. Die Datei öffnet sich in einem neuen Tab. Lassen Sie dann den Haken bei `Beim Bestätigen von der Zahlung befreien (Leistungsbezug)` oder nehmen Sie ihn weg.",
          "Fragt Ihr Träger die Debitorennummer ab, steht über `Bestätigen` das Feld `Debitorennummer`. Tragen Sie dort die Nummer der Familie bei der Kasse der Gemeinde ein. Ohne Nummer geht `Bestätigen` nicht.",
          "Wählen Sie `Bestätigen`. Damit ist der Beitrag festgehalten.",
          "Oder wählen Sie `Stornieren` und bestätigen Sie mit `Stornieren`.",
        ],
      },
      {
        title: "Debitorennummer nachtragen oder ändern",
        description:
          "Das geht bei einer bestätigten Anmeldung, bis der Abrechnungslauf ihre Rechnung ausgestellt hat.",
        steps: [
          "Öffnen Sie die bestätigte Anmeldung.",
          "Wählen Sie unter `Status` den Knopf `Debitorennummer eintragen` oder `Debitorennummer ändern`.",
          "Das Feld `Debitorennummer` öffnet sich in der Karte. Tragen Sie die Nummer ein und wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Geschwister zusammenführen",
        description:
          "Eltern melden Geschwister manchmal in zwei Anmeldungen an. Führen Sie die Anmeldungen zusammen, zählen die Kinder beim Ferienbeitrag zusammen. moto führt nie von selbst zusammen.",
        steps: [
          "In der Spalte `Geschwister` steht ein Hinweis wie `Vielleicht Geschwister: gleiche E-Mail` oder `Vielleicht Geschwister: gleicher Nachname`. Der Hinweis ändert nichts.",
          "Prüfen Sie, ob die Kinder wirklich Geschwister sind.",
          "Setzen Sie in der Liste einen Haken bei jeder Anmeldung der Familie.",
          "Wählen Sie `Als Geschwister zusammenführen`.",
          "In der Spalte `Geschwister` steht jetzt `Geschwister` mit den Nummern der anderen Anmeldungen.",
        ],
      },
      {
        title: "Geschwister trennen",
        steps: [
          "Öffnen Sie die Anmeldung.",
          "Wählen Sie in der Karte `Geschwister` den Knopf `Trennen`.",
          "Bestätigen Sie mit `Trennen`.",
        ],
      },
      {
        title: "Umbuchen",
        steps: [
          "Wählen Sie beim Kind `Umbuchen`.",
          "Wählen Sie je Woche ein Zeitmodell oder `Nicht angemeldet`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Einfrieren und wieder öffnen",
        steps: [
          "Nach dem Anmeldeschluss ist jede Anmeldung von selbst eingefroren.",
          "Wollen Sie eine Anmeldung vorher festhalten? Wählen Sie `Einfrieren`.",
          "Wollen Sie eine eingefrorene Anmeldung ändern? Wählen Sie `Wieder öffnen`.",
        ],
      },
      {
        title: "Exportieren",
        steps: [
          "Stellen Sie die Filter ein, zum Beispiel eine Schule.",
          "Wählen Sie `Herunterladen` und dann `Excel` oder `CSV`.",
        ],
      },
    ],
    result:
      "Unter `Verlauf` steht bei jeder Anmeldung, wann sie bestätigt, storniert, umgebucht, eingefroren oder wieder geöffnet wurde. Auch jede Debitorennummer steht dort, mit der Nummer vorher und nachher, und jedes Zusammenführen und Trennen von Geschwistern.",
    notes: [
      "Eltern bekommen beim Bestätigen und Stornieren noch keine Nachricht von moto.",
      "Eine eingefrorene Anmeldung können Sie bestätigen. Stornieren und Umbuchen gehen erst nach `Wieder öffnen`.",
      "Der Export hat eine Zeile je Kind und Woche. Gesundheitsangaben stehen nicht darin. Antworten auf eigene Fragen ohne Gesundheitsangabe haben eigene Spalten.",
      "Gesundheitsangaben sieht in der Einzelansicht nur die Rolle Verwaltung. Jedes Öffnen steht im `Zugriffsprotokoll` des Trägers.",
      "Eine `Zweite sorgeberechtigte Person` steht in der Einzelansicht, nicht in der Liste. Im Export stehen Name und Telefon.",
      "Fragt das Formular die Anschrift ab, steht sie in der Einzelansicht bei `Eltern` und im Export. In der Liste und bei der ausrichtenden OGS steht sie nicht.",
      "Ein hochgeladener `Nachweis für den Leistungsbezug` steht nur in der Einzelansicht. Er steht nicht in der Liste, nicht im Export und nicht bei der ausrichtenden OGS. Er wird mit der Anmeldung gelöscht.",
      "Jedes Öffnen einer Anmeldung oder eines Nachweises steht im `Zugriffsprotokoll` des Trägers. Auch jeder Export steht dort.",
      "Ein festgehaltener Beitrag ändert sich nicht mehr. Auch nicht, wenn Sie später Preise oder Regeln ändern oder umbuchen.",
      "Die Debitorennummer steht im Bestätigungsschreiben als Verwendungszweck. Im Export steht sie in der Spalte `Debitorennummer`. In der Liste steht sie nur, wenn Ihr Träger sie abfragt.",
      "Bei Geschwistern ist das Kind mit dem höchsten Beitrag das 1. Kind. Bei gleichem Beitrag zählt die früher eingegangene Anmeldung zuerst.",
      "Zusammenführen und Trennen rechnen auch einen schon bestätigten Beitrag neu. Der alte Beitrag bleibt gespeichert. Ein offener Abrechnungslauf zeigt den neuen Betrag.",
      "Jede Anmeldung bekommt weiter ihre eigene Rechnung und ihr eigenes Schreiben. Ein Schreiben nennt nie die Kinder oder Eltern der anderen Anmeldung.",
      "Alle bestätigten Anmeldungen der Geschwister bekommen dieselbe Debitorennummer. Beim Bestätigen steht die Nummer der Geschwister schon im Feld.",
    ],
    differences: [
      "Ein Kind ist als `Mögliche Doppelanmeldung` markiert? Ein Kind mit gleichem Namen und Geburtsdatum steht auch in einer anderen Anmeldung. Öffnen Sie beide und stornieren Sie die doppelte.",
      "Beim Bestätigen gibt es kein Feld `Debitorennummer`? Ihr Träger fragt sie nicht ab. Das legen Sie unter `Einstellungen` im Bereich `Ferienbetreuung` fest.",
    ],
    troubleshootingDetails: [
      "`Umbuchen` fehlt? Die Anmeldung ist eingefroren oder storniert. Eine eingefrorene Anmeldung öffnen Sie mit `Wieder öffnen`.",
      "Eine stornierte Anmeldung lässt sich nicht wieder öffnen. Die Eltern melden das Kind dann neu an.",
      "Die Debitorennummer lässt sich nicht mehr ändern? Für die Anmeldung gibt es schon eine Rechnung. Die Rechnung behält die Nummer, mit der sie ausgestellt wurde.",
      "`Trennen` fehlt oder Zusammenführen geht nicht? Für eine Anmeldung der Geschwister gibt es schon eine Rechnung. Geschwister können Sie dann nicht mehr ändern.",
      "Eine stornierte Anmeldung können Sie nicht zusammenführen. Trennen geht.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayRegistration,
      HELP_TOPICS.traegerHolidayPeriod,
      HELP_TOPICS.traegerHolidayContribution,
    ],
  };
}
