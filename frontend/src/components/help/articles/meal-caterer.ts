import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Der Caterer der Schule (ADR 0077): Einladung per E-Mail unter
 * `Speiseplan`, Ende der Belieferung, und was der Caterer sieht.
 */
export function mealCatererTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadMealCaterer,
    title: "Caterer einladen",
    question: "Wie sieht unser Caterer die Bestellungen?",
    summary:
      "Sie laden Ihren Caterer per E-Mail ein. Er sieht dann die Bestellungen.",
    group: "elternarbeit",
    audience: "lead",
    icon: "Truck",
    requirements: [
      "`Anmeldung zum Mittagessen` ist eingeschaltet.",
      "Sie sind Admin Ihrer Schule. Nur Admins laden einen Caterer ein oder beenden die Belieferung.",
    ],
    steps: [
      "Öffnen Sie in der Seitenleiste `Speiseplan`.",
      "Gehen Sie zur Karte `Caterer`. Sie steht unter `Bestellung`.",
      "Wählen Sie `Caterer einladen`. Ein Fenster öffnet sich.",
      "Tragen Sie `Name des Caterers` und `E-Mail-Adresse des Caterers` ein. Die Lieferadresse ist freiwillig.",
      "Wählen Sie `Einladung senden`.",
    ],
    result:
      "Der Caterer bekommt eine E-Mail mit einem Link. Damit legt er sein Passwort fest. Danach sieht er die Bestellungen Ihrer Schule.",
    notes: [
      "Den Link sieht nur der Caterer. Er gilt 14 Tage.",
      "Der Caterer sieht nur Zahlen: je Tag und, wenn Sie so bestellen, je Klasse. Namen sieht er nur, wenn Sie unter `Einstellungen` → `Datenschutz` `Namen der Kinder für den Caterer` einschalten.",
      "Der Caterer kann eine Bestellung nur bestätigen. Dann steht bei Ihnen `Vom Caterer bestätigt`.",
      "Beliefert der Caterer schon andere Schulen, bestätigt er mit seinem Passwort. Ihre Schule kommt dann zu seinen Schulen dazu.",
      "Sein Passwort ändert der Caterer selbst. Er öffnet oben rechts sein Menü und wählt `Konto`. Hat er es vergessen, wählt er bei der Anmeldung `Passwort vergessen?`.",
      "Im `Konto` kann der Caterer auch die `Zwei-Faktor-Anmeldung` einschalten. Das ist freiwillig. Dann gibt er bei der Anmeldung zusätzlich einen Code aus seiner E-Mail ein.",
      "Die Belieferung beenden Sie auf der Karte `Caterer` über das Menü mit den drei Punkten: `Belieferung beenden`. moto fragt vorher nach. Danach sieht der Caterer Ihre Schule nicht mehr, und Sie können einen neuen Caterer einladen.",
      "Die Lieferliste bekommen Sie bei `Bestellung` über das Menü mit den drei Punkten. Wählen Sie unter `Lieferliste der Woche` `PDF`, `Excel` oder `CSV`. Die Woche wählen Sie oben unter dem Titel. Es ist dieselbe Liste, die Ihr Caterer sieht, ohne Namen.",
    ],
    troubleshootingDetails: [
      "Kam keine E-Mail an? Widerrufen Sie die Einladung unter `Offene Einladungen` im Menü mit den drei Punkten mit `Widerrufen`. Senden Sie dann eine neue.",
      "Fehlt die Karte `Caterer`? Dann ist `Anmeldung zum Mittagessen` aus, oder Ihr Konto darf den Speiseplan nicht bearbeiten.",
      "Steht auf der Karte nur „Einen Caterer einladen können nur Admins Ihrer Schule.“? Dann bitten Sie einen Admin Ihrer Schule um die Einladung.",
      "Kommt Ihr Caterer nicht mehr an den Code für die Zwei-Faktor-Anmeldung? Dann meldet er sich beim moto-Team. Sie als Schule können sie nicht zurücksetzen, weil ein Caterer mehrere Schulen beliefern kann.",
    ],
    related: [HELP_TOPICS.leadMealOrdering, HELP_TOPICS.leadMealPlan],
  };
}
