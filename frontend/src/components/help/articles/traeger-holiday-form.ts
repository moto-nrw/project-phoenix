import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerHolidayFormTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerHolidayForm,
    title: "Anmeldeformular der Ferien",
    question: "Wie lege ich eigene Fragen für die Ferienanmeldung fest?",
    summary:
      "Jeder Ferienzeitraum hat ein eigenes Formular. Sie wählen, welche Fragen Eltern zusätzlich beantworten.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "NotePencil",
    image: "/help/screens/ferienformular-ogs-sieht-antwort.webp",
    imageAlt:
      "Eine Frage mit den Schaltern Gesundheitsangabe und OGS sieht die Antwort.",
    requirements: [
      "Ihre Rolle im Träger-Büro darf Anmeldungen verwalten. Das gilt für die Rolle Verwaltung.",
    ],
    steps: [],
    instructionGroups: [
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
          "Soll die OGS der Ferienwoche die Antwort sehen? Wählen Sie `OGS sieht die Antwort`. Das geht nur ohne `Gesundheitsangabe`.",
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
      "Ein neuer Ferienzeitraum startet mit der Standardvorlage. Alle Fragen darin sind zuerst freiwillig. Die OGS sieht dort die Antworten zu `Darf allein nach Hause gehen` und `Darf an Ausflügen teilnehmen`.",
      "Jedes Speichern legt eine neue Version an. Schon abgeschickte Anmeldungen behalten ihre Version und ihre Fragen.",
      "Mit `Änderungen verwerfen` holen Sie die gespeicherten Fragen zurück. Das gilt auch für Auswahloptionen.",
      "Eine Antwort mit Gesundheitsangabe braucht die Zustimmung der Eltern. Sie steht nur in der Anmeldung, nicht in Liste und Export.",
      "Antworten ohne Gesundheitsangabe stehen im Export in eigenen Spalten.",
      "Die ausrichtende OGS sieht nur Antworten mit `OGS sieht die Antwort`. Sie stehen in ihrer Liste unter `Ferienwochen` und beim Feriengast. Gesundheitsangaben sieht sie nie.",
      "`OGS sieht die Antwort` gilt ab der Version, in der Sie es speichern. Ältere Anmeldungen bleiben ohne Freigabe.",
      "Die OGS sieht auch Name und Telefon der zweiten sorgeberechtigten Person, keine E-Mail-Adresse.",
    ],
    troubleshootingDetails: [
      "`OGS sieht die Antwort` lässt sich nicht wählen? Die Frage ist eine `Gesundheitsangabe`. Gesundheitsangaben bleiben beim Träger.",
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
