import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/traeger/ferien/[id]/anmeldungen` (#4663): Anmeldungen prüfen. */
export function traegerHolidayRegistrationReviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayRegistrationReview,
    title: "Ferienanmeldungen prüfen",
    question: "Wie bestätige, storniere und buche ich Ferienanmeldungen um?",
    summary:
      "Sie sehen alle Anmeldungen eines Ferienzeitraums in einer Liste. Dort bestätigen, stornieren und buchen Sie um.",
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
          "Prüfen Sie die Angaben der Eltern und der Kinder. Antworten auf eigene Fragen stehen beim Kind unter `Weitere Angaben`, Antworten zur ganzen Familie unter `Weitere Angaben zur Familie`.",
          "Unter `Hinweise und Einwilligungen` sehen Sie, was die Eltern bestätigt haben und wann. Dort stehen die damaligen Texte und PDFs.",
          "Prüfen Sie unter `Ferienbeitrag`, was die Familie zahlt.",
          "Geben die Eltern an, dass sie Leistungen beziehen? Prüfen Sie den Nachweis. Haben die Eltern ihn hochgeladen, wählen Sie unter `Nachweis für den Leistungsbezug` den Dateinamen. Die Datei öffnet sich in einem neuen Tab. Lassen Sie dann den Haken bei `Beim Bestätigen von der Zahlung befreien (Leistungsbezug)` oder nehmen Sie ihn weg.",
          "Wählen Sie `Bestätigen`. Damit ist der Beitrag festgehalten.",
          "Oder wählen Sie `Stornieren` und bestätigen Sie mit `Stornieren`.",
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
      "Unter `Verlauf` steht bei jeder Anmeldung, wann sie bestätigt, storniert, umgebucht, eingefroren oder wieder geöffnet wurde.",
    notes: [
      "Eltern bekommen beim Bestätigen und Stornieren noch keine Nachricht von moto.",
      "Eine eingefrorene Anmeldung können Sie bestätigen. Stornieren und Umbuchen gehen erst nach `Wieder öffnen`.",
      "Der Export hat eine Zeile je Kind und Woche. Gesundheitsangaben stehen nicht darin. Antworten auf eigene Fragen ohne Gesundheitsangabe haben eigene Spalten.",
      "Eine `Zweite sorgeberechtigte Person` steht in der Einzelansicht, nicht in der Liste. Im Export stehen Name und Telefon.",
      "Ein hochgeladener `Nachweis für den Leistungsbezug` steht nur in der Einzelansicht. Er steht nicht in der Liste, nicht im Export und nicht bei der ausrichtenden OGS. Er wird mit der Anmeldung gelöscht.",
      "Jedes Öffnen einer Anmeldung oder eines Nachweises steht im `Zugriffsprotokoll` des Trägers. Auch jeder Export steht dort.",
      "Ein festgehaltener Beitrag ändert sich nicht mehr. Auch nicht, wenn Sie später Preise oder Regeln ändern oder umbuchen.",
    ],
    differences: [
      "Ein Kind ist als `Mögliche Doppelanmeldung` markiert? Ein Kind mit gleichem Namen und Geburtsdatum steht auch in einer anderen Anmeldung. Öffnen Sie beide und stornieren Sie die doppelte.",
    ],
    troubleshootingDetails: [
      "`Umbuchen` fehlt? Die Anmeldung ist eingefroren oder storniert. Eine eingefrorene Anmeldung öffnen Sie mit `Wieder öffnen`.",
      "Eine stornierte Anmeldung lässt sich nicht wieder öffnen. Die Eltern melden das Kind dann neu an.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayRegistration,
      HELP_TOPICS.traegerHolidayPeriod,
      HELP_TOPICS.traegerHolidayContribution,
    ],
  };
}
