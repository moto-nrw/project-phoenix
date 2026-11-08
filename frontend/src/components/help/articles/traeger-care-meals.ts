import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * Mensa-Abrechnung des Trägers (#4686): je Schule je Essen oder als
 * Pauschale, abgerechnet im Monatslauf aus der Freigabe der OGS-Leitung.
 */
export function traegerCareMealsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareMeals,
    title: "Mensa abrechnen",
    question: "Wie rechne ich das Essen ab, je Essen oder als Pauschale?",
    summary:
      "Sie legen je Schule fest, wie die Mensa abgerechnet wird. Der Monatslauf setzt freigegebene Monate auf die Rechnung des Kindes.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "UtensilsCrossed",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung.",
      "Die OGS-Leitung gibt jeden Monat in moto frei. Erst dann wird er abgerechnet.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Preis festlegen",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Mensa`.",
          "Wählen Sie bei der Schule `Preis festlegen`.",
          "Wählen Sie bei `Gültig ab` den ersten Monat.",
          "Wählen Sie die Abrechnungsart: `Je Essen` oder `Pauschale`.",
          "Tragen Sie den Betrag ein, zum Beispiel 4,80 je Essen.",
          "Wählen Sie `Preis speichern`.",
        ],
      },
      {
        title: "Im Monatslauf",
        description:
          "Die Mensa steht im Monatslauf als eigene Zeile je Monat auf der Rechnung des Kindes, zum Beispiel „2 Mittagessen im Januar 2026“.",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Monatsbeiträge`.",
          "Starten oder öffnen Sie den Lauf eines Monats.",
          "Die Karte `Mensa` nennt Monate, die eine Schule noch nicht freigegeben hat, und Kinder ohne Vertrag.",
          "`Nicht mehr abrechenbar` heißt: Die Anwesenheiten des Monats sind nach der Aufbewahrungsfrist der Schule gelöscht. Diesen Monat rechnet kein Lauf mehr ab.",
          "Prüfen Sie die Rechnungen und geben Sie den Lauf frei.",
        ],
      },
    ],
    result:
      "Jeder freigegebene Mensa-Monat steht genau einmal auf einer Rechnung. Gibt eine Schule einen Monat später frei, rechnet ihn der nächste Lauf nachträglich ab.",
    notes: [
      "Nach einem Storno stellen Sie die Mensa im ursprünglichen Rechnungsmonat neu aus. Starten Sie dafür einen weiteren Lauf dieses Monats.",
      "Ermäßigungen wie Geschwister oder Einkommensstufen gelten nur für die Module. Die Mensa zahlen Eltern voll.",
      "Ein Preis, mit dem schon abgerechnet ist, bleibt für diese Monate. Ändern Sie ihn ab einem späteren Monat.",
    ],
    related: [HELP_TOPICS.traegerCareMonths, HELP_TOPICS.traegerCareReductions],
  };
}
