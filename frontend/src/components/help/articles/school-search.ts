import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

// Die Suche über die ganze Schul-App (#6954, ADR 0227). Was jemand findet,
// entscheidet der Server nach den Rechten des Kontos; der Artikel nennt nur,
// was die Person dann anders sieht.
export function schoolSearchTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.schoolSearch,
    title: "Mit der Suche finden",
    question: "Wie finde ich schnell ein Kind, eine Familie oder eine Seite?",
    summary:
      "Eine Suche findet Kinder, Familien, Team, Räume, Gruppen und Seiten.",
    group: "einstieg",
    audience: ["caregiver", "lead"],
    icon: "ListSearch",
    steps: [],
    instructionGroups: [
      {
        title: "Die Suche öffnen",
        ordered: false,
        steps: [
          "Am Computer: Wählen Sie oben `Überall suchen`. Oder drücken Sie Strg und K. Am Mac drücken Sie ⌘ und K.",
          "Am Tablet und am Handy: Wählen Sie oben neben dem Fragezeichen die Lupe. Der Reiter `Suchen` unten öffnet dagegen `Alle Kinder`.",
        ],
      },
      {
        title: "Suchen und öffnen",
        steps: [
          "Geben Sie mindestens zwei Zeichen ein.",
          "Sie können einen Namen, eine Klasse, eine Gruppe, eine Telefonnummer oder eine E-Mail-Adresse eingeben.",
          "Die Treffer stehen nach Art sortiert: `Kinder`, `Sorgeberechtigte und Kontakte`, `Mitarbeitende`, `Räume`, `Gruppen` und `Seiten`.",
          "Wählen Sie einen Treffer. Oder wählen Sie ihn mit den Pfeiltasten und drücken Sie Enter.",
        ],
      },
      {
        // Der Anlass aus #6954: verpasste Anrufe im Büro.
        title: "Eine Telefonnummer suchen",
        steps: [
          "Geben Sie die Nummer so ein, wie Sie sie sehen. Leerzeichen, Striche, +49 oder eine 0 am Anfang sind egal.",
          "Auch die letzten Ziffern reichen, mindestens vier.",
          "Der Treffer zeigt die Person und das Kind, zu dem sie gehört.",
          "Wählen Sie den Treffer. moto öffnet beim Kind den Reiter `Erziehungsberechtigte`.",
        ],
      },
      {
        title: "Nach Angaben aus der Anmeldung suchen",
        steps: [
          "Geben Sie einen Begriff aus dem Anmeldeformular ein, zum Beispiel `schwimmen`.",
          "Sie sehen die Kinder, bei denen die Eltern dazu etwas angegeben haben. Darunter steht die Frage und die Antwort.",
          "Eine ganze Liste brauchen Sie? Öffnen Sie `Alle Kinder` und wählen Sie das Filtersymbol. Unter `Anmeldung` wählen Sie eine Antwort aus `Angabe aus der Anmeldung`.",
        ],
      },
    ],
    result:
      "Sie landen direkt beim Kind, bei der Person, im Raum oder auf der Seite.",
    differences: [
      "Sie finden nur, was Sie auch ohne Suche sehen dürfen. Telefonnummern, E-Mail-Adressen, Elternnotizen und Angaben aus der Anmeldung finden nur Konten, die Kontaktdaten sehen dürfen.",
      "Ein Gruppen-Treffer öffnet `Alle Kinder` mit den Kindern dieser Gruppe.",
      "Gesundheitsangaben und Notizen aus dem Reiter `Notizen` durchsucht die Suche nicht. Öffnen Sie dafür das Kind.",
    ],
    notes: [
      "Ohne Eingabe zeigt die Suche die Seiten, die Sie öffnen dürfen.",
      "Mit `Alle Treffer in „Alle Kinder“` sehen Sie alle Kinder mit diesem Namen.",
    ],
    related: [HELP_TOPICS.studentSearch, HELP_TOPICS.appOverview],
  };
}
