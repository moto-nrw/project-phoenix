import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Die Mensa-Freigabe (#4686): Die OGS-Leitung prüft am Monatsende, wer
 * gegessen hat, und gibt den Monat für die Abrechnung des Trägers frei.
 */
export function mealBillingReleaseTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadMealBillingRelease,
    title: "Mensa-Monat freigeben",
    question: "Wie gebe ich die Mensa eines Monats für die Abrechnung frei?",
    summary:
      "Am Monatsende prüfen Sie, wer gegessen hat. Dann geben Sie den Monat frei. Erst danach rechnet der Träger die Mensa ab.",
    group: "elternarbeit",
    audience: "lead",
    icon: "ClipboardList",
    requirements: [
      "`Anmeldung zum Mittagessen` ist eingeschaltet.",
      "Sie dürfen den Speiseplan pflegen und Listen mit Namen sehen.",
      "Der Monat ist vorbei. Freigeben können Sie ab dem 1. des nächsten Monats.",
    ],
    steps: [
      "Öffnen Sie in der Seitenleiste `Speiseplan`.",
      "Wählen Sie oben `Mensa-Freigabe`.",
      "Wählen Sie den Monat. Ein Monat mit `Freigabe fehlt` wartet auf Sie.",
      "Prüfen Sie in der Liste die Spalte `Nicht da`. Dort stehen die Tage, an denen ein angemeldetes Kind nicht da war.",
      "Fehlt eine Anwesenheit, tragen Sie sie zuerst nach. Laden Sie die Seite danach neu.",
      "Wählen Sie `Monat freigeben` und bestätigen Sie mit `Freigeben`.",
    ],
    result:
      "Der Monat steht auf `Freigegeben`. Der Träger rechnet die Essen mit dem nächsten Monatslauf ab. Die Zahlen bleiben so.",
    notes: [
      "Ein Essen zählt, wenn das Kind angemeldet und da war. Tage, an denen die Eltern abgemeldet haben, zählen nicht.",
      "Ferien, Feiertage und Schließtage zählen nie.",
      "Sie können einen Monat später freigeben, solange alle Anwesenheiten aufbewahrt werden. Der Träger rechnet ihn dann nachträglich ab.",
    ],
    troubleshootingDetails: [
      "`Nicht mehr abrechenbar`: Die Anwesenheiten dieses Monats sind nach der Aufbewahrungsfrist gelöscht. Eine Freigabe ist nicht mehr möglich. Geben Sie Monate deshalb bald nach dem Monatsende frei.",
      "`Seit dem Laden hat sich der Monat geändert`: Jemand hat eine Anwesenheit oder Anmeldung geändert. Prüfen Sie die Liste noch einmal und geben Sie dann frei.",
      "Haben Sie nach der Freigabe eine Anwesenheit nachgetragen, steht oben `Seit der Freigabe hat sich die Anwesenheit geändert`. Prüfen Sie die Liste, wählen Sie `Erneut freigeben` und nennen Sie den Grund. Schon gestellte Rechnungen ändert das nicht. Der Träger sieht die Änderung und korrigiert die Rechnung.",
    ],
    related: [HELP_TOPICS.leadMealPlan],
  };
}
