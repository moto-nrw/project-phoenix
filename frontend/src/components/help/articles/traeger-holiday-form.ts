import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerHolidayFormTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayForm,
    title: "Anmeldeformular der Ferien",
    question: "Wie lege ich eigene Fragen für die Ferienanmeldung fest?",
    summary:
      "Jeder Ferienzeitraum hat ein eigenes Formular. Sie wählen, welche Fragen Eltern zusätzlich beantworten.",
    image: "/help/screens/ferienformular-hinweise.webp",
    imageAlt:
      "Eigene Hinweise mit PDF und Pflichtbestätigung im Ferienformular.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "NotePencil",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Hinweise und Einwilligungen ergänzen",
        steps: [
          "Wählen Sie unter `Hinweise und Einwilligungen` die Aktion `Eigene Zustimmung hinzufügen`.",
          "Tragen Sie `Titel` und `Text neben dem Häkchen oder Hinweis` ein.",
          "Ergänzen Sie den Text unter `Rechtstext / Erklärung`.",
          "Für ein PDF wählen Sie `PDFs hinzufügen`. Höchstens 10 MB je Datei.",
          "Tragen Sie unter `Bezeichnung für Eltern` einen passenden Namen ein.",
          "Müssen Eltern den Hinweis bestätigen? Wählen Sie `Muss bestätigt werden`.",
          "Eine freiwillige Einwilligung lassen Sie ohne diese Pflicht.",
          "Wählen Sie `Als neue Version speichern`.",
        ],
      },
      {
        title: "Fragen festlegen",
        steps: [
          "Wählen Sie in der Navigation `Ferien`.",
          "Öffnen Sie den Ferienzeitraum.",
          "Wählen Sie den Reiter `Formular`.",
          "Wählen Sie `Frage hinzufügen` oder `Infotext hinzufügen`.",
          "Schreiben Sie die Frage und wählen Sie den `Typ`, zum Beispiel `Ja/Nein` oder `Auswahl`.",
          "Soll jede Familie antworten oder jedes Kind? Für Fragen zum Kind wählen Sie `Pro Kind abfragen`.",
          "Muss die Frage beantwortet werden? Wählen Sie `Pflichtfrage`.",
          "Geht es um die Gesundheit des Kindes? Wählen Sie `Gesundheitsangabe`.",
          "Mit den Pfeilen ändern Sie die Reihenfolge.",
          "Wählen Sie `Als neue Version speichern`.",
        ],
      },
      {
        title: "Standardvorlage einfügen",
        steps: [
          "Wählen Sie `Standardvorlage einfügen`.",
          "moto fügt die Fragen der Vorlage ein, die im Formular fehlen. Zum Beispiel Lebensmittelunverträglichkeiten, Tetanus-Impfschutz und Ausflüge.",
          "Ändern oder entfernen Sie Fragen, die Sie nicht brauchen.",
          "Wählen Sie `Als neue Version speichern`.",
        ],
      },
      {
        title: "Frage nur unter einer Bedingung zeigen",
        steps: [
          "Legen Sie zuerst eine Frage mit `Ja/Nein` oder `Auswahl` an.",
          "Wählen Sie bei der anderen Frage `Nur unter einer Bedingung anzeigen`.",
          "Wählen Sie die Frage und die Antwort, bei der sie erscheinen soll.",
        ],
      },
    ],
    result:
      "Neue Anmeldungen nutzen das gespeicherte Formular. Rechts sehen Sie, wie Eltern es sehen.",
    notes: [
      "Name, Geburtsdatum, Schule, Wochen, Allergien, Medikamente, Kontakte und Zustimmungen fragt die Anmeldung immer ab.",
      "Ein neuer Ferienzeitraum startet mit der Standardvorlage. Alle Fragen darin sind zuerst freiwillig.",
      "Schon verwendete PDFs bleiben bei der alten Anmeldung lesbar. Neue Anmeldungen sehen die neue Version.",
      "Jedes Speichern legt eine neue Version an. Schon abgeschickte Anmeldungen behalten ihre Version und ihre Fragen.",
      "Mit `Änderungen verwerfen` holen Sie die gespeicherten Fragen zurück. Das gilt auch für Auswahloptionen.",
      "Eine Antwort mit Gesundheitsangabe braucht die Zustimmung der Eltern. Sie steht nur in der Anmeldung, nicht in Liste und Export.",
      "Antworten ohne Gesundheitsangabe stehen im Export in eigenen Spalten.",
      "Die ausrichtende OGS sieht die Antworten nicht.",
    ],
    troubleshootingDetails: [
      "`Nur unter einer Bedingung anzeigen` lässt sich nicht wählen? Es gibt noch keine Frage mit `Ja/Nein` oder `Auswahl`. Eine Frage zur Familie kann nicht von einer Frage zum Kind abhängen.",
      "`Standardvorlage einfügen` lässt sich nicht wählen? Alle Fragen der Vorlage sind schon im Formular.",
    ],
    related: [
      HELP_TOPICS.traegerHolidayPeriod,
      HELP_TOPICS.traegerHolidayRegistration,
      HELP_TOPICS.traegerHolidayRegistrationReview,
    ],
  };
}
