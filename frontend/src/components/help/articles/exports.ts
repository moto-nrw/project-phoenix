import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/database/exports`: fertige Listen als Datei (Kinder, Personal, Notfall). */
export function exportsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadExports,
    title: "Eine Liste exportieren",
    question: "Wie bekomme ich Daten aus moto als Datei?",
    summary: "Fertige Listen lassen sich als Datei herunterladen.",
    group: "auswertung",
    audience: "lead",
    icon: "Download",
    steps: [
      "Öffnen Sie `Datenverwaltung` und danach `Exporte`.",
      "Suchen Sie die passende Liste und wählen Sie `Liste erstellen`.",
      "Haken Sie unter `Spalten` an, was in der Liste stehen soll.",
      "Wählen Sie unter `Format` `PDF`, `Word` oder `Excel`.",
      "Wählen Sie `Herunterladen`.",
    ],
    result: "Die Datei liegt im Download-Ordner Ihres Geräts.",
    notes: [
      "Die Listen sind in `Kinderlisten`, `Personallisten` und `Momentaufnahmen` geordnet.",
      "Eine `Momentaufnahme` wie die `Notfallliste` zeigt den Stand von jetzt. Dort öffnet `Liste erstellen` ein Menü: `Drucken` oder das Format.",
      "Die `Gesundheitsliste` zeigt Allergien, Medikamente und andere Gesundheitsinformationen.",
      "Sie enthält zuerst nur Kinder mit einem Eintrag.",
      "`Auch Kinder ohne Eintrag` nimmt alle Kinder auf.",
      "Jeder Export der `Gesundheitsliste` wird protokolliert.",
      "Jede Datei enthält personenbezogene Daten. Behandeln Sie sie wie jede andere Unterlage dieser Art.",
    ],
    differences: [
      "Sie sehen weniger Listen als eine Kollegin? Jede Liste hängt an einem eigenen Recht.",
    ],
    troubleshootingDetails: [
      "Fehlt ein Kind auf der `Gesundheitsliste`? Dann ist bei ihm nichts eingetragen. Haken Sie `Auch Kinder ohne Eintrag` an.",
      "Brauchen Sie Zahlen statt Namen? Nutzen Sie die `Statistik`.",
      "Brauchen Sie eine Liste für einen bestimmten Tag? Nutzen Sie die `Tageslisten`.",
    ],
    related: [
      HELP_TOPICS.leadStatistics,
      HELP_TOPICS.leadDayLists,
      HELP_TOPICS.dataManagement,
    ],
  };
}
