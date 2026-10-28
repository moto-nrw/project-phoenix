import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/lists`: druckbare Tageslisten aus den Betreuungsplan-Slots (#1565). */
export function dayListsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadDayLists,
    title: "Tageslisten erstellen",
    question: "Wie drucke ich eine Liste für einen Tag?",
    summary: "Tageslisten zeigen für einen Tag, wer wo sein soll.",
    group: "planung",
    audience: "lead",
    icon: "ClipboardList",
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Tageslisten`.",
      "Wählen Sie oben das Datum.",
      "Wählen Sie unter `Quelle` die Art der Liste.",
      "Prüfen Sie die `Vorschau`.",
      "Öffnen Sie das Menü mit den drei Punkten.",
      "Wählen Sie `Drucken`, `PDF herunterladen` oder `Excel herunterladen`.",
    ],
    result: "Die Liste ist gedruckt oder als Datei gespeichert.",
    notes: [
      "Unter `Quelle` stehen `Randstunden`, `Lernzeit`, `AG-Angebote`, `Mensa`, `Ganztag (frühe Abholung)`, `Ganztag (späte Abholung)` und `Freie Angebotsauswahl`.",
      "Als Datenbasis wählen Sie `Plan`, `Ist` oder `Abgleich`.",
      "Eine Liste mit `Plan` hat eine leere Spalte `Abhaken`. Dort haken Sie die Kinder auf dem Ausdruck ab.",
      "Unter der Datumszeile steht, woher die Daten kommen.",
    ],
    differences: [
      "Eine Art steht nicht zur Wahl? Dann gibt es an diesem Tag nichts dafür.",
    ],
    troubleshootingDetails: [
      "Die Vorschau bleibt leer? Prüfen Sie Datum und Art. An einem Schließtag gibt es keine Liste.",
    ],
    related: [
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.leadClassListEntries,
      HELP_TOPICS.leadExports,
    ],
  };
}
