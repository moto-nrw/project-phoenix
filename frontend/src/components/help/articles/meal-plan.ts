import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Essensplan unter `/meal-plan`. Wochenweise, mit ausdruecklichem Speichern:
 * Aenderungen sind bis dahin nur Entwurf (page.tsx:799).
 */
export function mealPlanTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadMealPlan,
    title: "Einen Speiseplan veröffentlichen",
    question: "Wie zeige ich den Eltern den Speiseplan?",
    summary:
      "Tragen Sie die Gerichte je Tag ein. Eltern sehen den Plan sofort.",
    group: "elternarbeit",
    audience: "lead",
    icon: "UtensilsCrossed",
    steps: [
      "Klappen Sie in der Seitenleiste `Eltern` auf.",
      "Öffnen Sie `Speiseplan`.",
      "Blättern Sie oben unter dem Titel mit den Pfeilen zur gewünschten Woche. `Diese Woche` bringt Sie zurück.",
      "Tragen Sie bei einem Tag unter `Gericht eintragen…` das Gericht ein.",
      "Tragen Sie bei Bedarf einen `Hinweis (optional)` dazu ein.",
      "Öffnen Sie unter dem Gericht die Auswahl `Allergene` und haken Sie die Allergene an. Danach stehen dort die Buchstaben, zum Beispiel `A, G`.",
      "Mit `+ Gericht` ergänzen Sie ein zweites Gericht für denselben Tag.",
      "Wählen Sie `Speichern`.",
    ],
    result: "Der Plan erscheint sofort im Elternportal.",
    notes: [
      "Pro Tag sind mehrere Gerichte möglich.",
      "Zur Auswahl stehen die 14 Hauptallergene, jedes mit einem festen Buchstaben. Die Liste ist für alle Schulen gleich.",
      "Eltern sehen die Buchstaben am Gericht und darunter die Erklärung. Dort steht auch, dass die Angaben von der Schule oder vom Caterer kommen.",
      "Mit `Vorwoche übernehmen` holen Sie die Gerichte der Vorwoche in diese Woche.",
      "Der heutige Tag trägt das Kennzeichen `Heute`.",
      "Die Woche oben gilt für den Plan und für die `Bestellung` darunter.",
      "Ganz unten steht die Liste für die Küche. Mit `Tag` und `Woche` wechseln Sie die Ansicht.",
      "Die Wochenliste zeigt jedes angemeldete Kind mit einem Haken je Tag. Darüber steht, wie viele Kinder an jedem Tag essen.",
      "Öffnen Sie `Tagesliste herunterladen` oder `Wochenliste herunterladen`. Wählen Sie `PDF` oder `Excel`.",
      "Bis wann Eltern ändern können, legen Sie unter `Einstellungen` im Bereich `Mittagessen` fest.",
      "An Feiertagen und Schließtagen gibt es kein Essen. Die Wochenliste zeigt dort `kein Essen`.",
    ],
    differences: [
      "Ihre Änderungen zählen erst mit `Speichern`. Vorher steht `Verwerfen` daneben.",
      "Beim Wechseln der Woche mit offenen Änderungen fragt moto nach.",
    ],
    troubleshootingDetails: [
      "Fehlt `Speiseplan` in der Seitenleiste? Dann ist die Funktion für Ihre OGS ausgeschaltet, oder Ihrer Rolle fehlt das Recht `Mittagessen: Lesen`.",
    ],
    related: [
      HELP_TOPICS.leadParentAnnouncement,
      HELP_TOPICS.leadParentVisibility,
      HELP_TOPICS.settings,
    ],
  };
}
