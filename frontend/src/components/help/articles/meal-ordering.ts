import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Anmeldemodell und Fristen fürs Mittagessen (#3873): Einstellungen, Reiter
 * `Mittagessen`. Die Fristen gelten für Eltern; die Schule ändert jederzeit.
 */
export function mealOrderingTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadMealOrdering,
    title: "Fristen für das Mittagessen festlegen",
    question: "Bis wann melden Eltern ihr Kind zum Essen an oder ab?",
    summary:
      "Sie legen fest, wie Eltern anmelden und bis wann sie ändern dürfen.",
    group: "elternarbeit",
    audience: "lead",
    icon: "Hourglass",
    requirements: ["`Anmeldung zum Mittagessen` ist eingeschaltet."],
    steps: [
      "Wählen Sie unten in der Seitenleiste `Einstellungen`.",
      "Wählen Sie oben den Reiter `Mittagessen`.",
      "Wählen Sie bei `Wie Eltern anmelden` eine Möglichkeit.",
      "Legen Sie unter `Fristen` die `Anmeldefrist` fest: Tage vor dem Essenstag oder ein Wochentag der Vorwoche, dazu die Uhrzeit.",
      "Nutzen Sie eine eigene Abmeldefrist, legen Sie auch die `Abmeldefrist` fest.",
      "Wählen Sie bei `Krank oder abwesend nach der Frist`, was mit dem Essen passiert.",
    ],
    result:
      "Eltern sehen bei jedem Tag, bis wann sie an- oder abmelden können.",
    notes: [
      "`Laufend, eine Frist` ist der Standard: Bis zur Frist dürfen Eltern an- und abmelden.",
      "`Laufend, mit eigener Abmeldefrist` passt, wenn der Caterer früh bestellt, Abmelden aber später noch geht.",
      "`Für das Schuljahr`: Eltern melden bis zum `Anmeldeschluss für das Schuljahr` an. Danach meldet nur die Schule an.",
      "Bei `Abmelden nach dem Anmeldeschluss` legen Sie fest, ob Eltern danach noch Tage oder Wochen abmelden.",
      "An Schließtagen und Feiertagen isst niemand. In den Ferien gelten die festen Tage nicht, einzeln angemeldete Tage zählen.",
      "`Entschuldigt` und `Klassenfahrt` melden das Essen ab wie eine Krankmeldung.",
      "Für die Küche legen Sie eine Rolle mit den Rechten unter `Mittagessen` an. Sie braucht keinen Zugriff auf alle Kinderdaten.",
      "Zur Frist friert moto die Bestellung ein. Unter `Speiseplan` zeigt `Bestellung` je Tag `Offen`, `Bestellt` oder `Vom Caterer bestätigt`, dazu die `Frist`, die Zahl der Essen und die `Nachmeldungen` danach.",
      "Tage vor dem Start der Bestellung stehen auf `Nicht bestellt`. Sie zählen nicht.",
      "Die bestellten Essen je Kind für einen Monat bekommen Sie bei `Bestellung` über das Menü mit den drei Punkten, zum Beispiel für die Essenspauschale. Wählen Sie unter `Monatsübersicht` den Eintrag `PDF` oder `Excel`. Liegt die Woche in zwei Monaten, steht dort `Exportieren…`: Wählen Sie dann den Monat, `PDF` oder `Excel` und `Herunterladen`.",
    ],
    differences: [
      "Wochenenden zählen bei `Tage vorher` mit. 1 heißt immer: am Tag davor.",
      "Die Schule ändert eine Anmeldung auch nach der Frist: unter `Speiseplan` in der Tagesliste. `Anmeldung ändern` öffnet ein Fenster, in dem Sie das Kind wählen. Für ein Kind aus der Liste wählen Sie im Menü mit den drei Punkten in seiner Zeile `Anmeldung ändern…`. Das geht für einen Tag, zurück `Wie an festen Tagen` oder für die festen Tage ab einem Datum. moto hält jede Änderung mit Konto und Zeit fest.",
    ],
    troubleshootingDetails: [
      "Fehlen die Fristen? Dann ist `Anmeldung zum Mittagessen` noch aus.",
      "Fehlt `Anmeldung ändern`? Wählen Sie die Ansicht `Tag`. Fehlt es dann noch, braucht Ihr Konto das Recht `Anmeldung eines Kindes ändern, auch nach der Frist`.",
      "Ist `Anmeldung ändern` grau? Für vergangene Tage und für Tage, an denen die OGS geschlossen ist, lässt sich die Anmeldung nicht ändern.",
    ],
    related: [HELP_TOPICS.leadMealPlan, HELP_TOPICS.settings],
  };
}
