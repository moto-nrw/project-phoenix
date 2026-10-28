import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function editStudentTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.editStudent,
    title: "Angaben oder Betreuungszeiten eines Kindes ändern",
    question: "Wie ändere ich Angaben oder Betreuungszeiten eines Kindes?",
    summary:
      "Ändern Sie persönliche Angaben, den Wochenplan oder einen einzelnen Tag.",
    group: "kinder",
    audience: "caregiver",
    icon: "FileText",
    steps: [],
    instructionGroups: [
      {
        title: "Persönliche Angaben ändern",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie das Kind und wählen Sie `Stammdaten`.",
          "Wählen Sie `Bearbeiten`.",
          "Ändern Sie die benötigten Angaben.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Regelmäßige Betreuungszeiten ändern",
        description:
          "Der Wochenplan gilt ab sofort für alle kommenden Wochen. Schon eingetragene Ausnahmen bleiben bestehen.",
        steps: [
          "Öffnen Sie beim Kind den Reiter `Betreuungszeiten`.",
          "Wählen Sie oben rechts `Wochenplan`.",
          "Setzen Sie bei den Betreuungstagen einen Haken.",
          "Tragen Sie je Tag `Ankunft` und `Abholung` ein.",
          "Kommt das Kind an einem Betreuungstag nicht? Lassen Sie `Abholung` leer.",
          "Öffnen Sie `Notizen`. Tragen Sie den Hinweis unter `Notiz zum Tag (jede Woche)` ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        // Live geprueft: jede Tageskarte hat oben rechts den Knopf
        // `Ausnahme`. Ankunft und Abholung haben je drei Auswahlfelder.
        title: "Nur einen Tag ändern",
        description:
          "Eine Ausnahme gilt nur an diesem Tag. Die festen Zeiten der Woche bleiben unverändert.",
        steps: [
          "Öffnen Sie beim Kind den Reiter `Betreuungszeiten`.",
          "Wählen Sie bei Bedarf `Vorherige Woche` oder `Nächste Woche`.",
          "Wählen Sie auf der Karte des Tages `Ausnahme`.",
          "Wählen Sie unter `Ankunft` `Regulär`, `Andere Zeit` oder `Kommt nicht`.",
          "Wählen Sie unter `Abholung` `Regulär`, `Andere Zeit` oder `Keine Abholung`.",
          // #3945: eigener Abschnitt unter Ankunft und Abholung.
          "Geht das Kind an diesem Tag anders nach Hause? Wählen Sie unter `Abholart` zum Beispiel `Wird abgeholt`. `Wie im Wochenplan` nimmt die Änderung zurück.",
          "Tragen Sie bei Bedarf unter `Hinweise nur für diesen Tag` etwas ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      // #3946: student-card.tsx (dayHomeChange) zeigt unten einen orangen Streifen.
      "Die gespeicherten Angaben gelten sofort. Eine Notiz ohne Abholzeit steht auf der Kinderkarte unter `Kommt heute nicht`. Eine andere Abholart steht an diesem Tag auf der Kinderkarte und in der Klassenansicht. Eine Ausnahme ändert den regelmäßigen Wochenplan nicht. Gilt heute eine andere Gehzeit oder Abholart, steht unten auf der Kinderkarte ein oranger Hinweis, zum Beispiel `Gehzeit heute geändert`.",
    differences: [
      "Die Kennzeichnung `Für Eltern sichtbar` zeigt Angaben, die Eltern sehen.",
      "Kommen die Betreuungstage aus Buchungen? Dann lassen sich die Tage nicht auswählen. Ändern Sie nur Zeiten an gebuchten Tagen.",
      "Fehlt `Bearbeiten` oder `Wochenplan`? Fragen Sie Ihre Leitung nach den nötigen Rechten.",
    ],
    notes: [
      "Steht unter dem Feld `Ankunft` die Auswahl `Nach Schulstunde`? Wählen Sie zum Beispiel `5. Stunde`. moto trägt die passende Uhrzeit ein.",
      // #3371: Knopf erscheint nur mit gepflegter Vorgabe unter einem leeren
      // Feld an einem Betreuungstag (care-weekly-plan-editor.tsx).
      "Steht unter einem leeren Feld zum Beispiel `16:00 Uhr eintragen`? Ein Klick trägt die übliche Zeit Ihrer Schule ein.",
      "Betreut Ihre OGS am Wochenende? Dann gilt dort der Freitagsplan. Am Wochenende zeigt die Wochenansicht dieser Woche auch Samstag und Sonntag.",
    ],
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.carePlan,
      HELP_TOPICS.dayLog,
    ],
  };
}
